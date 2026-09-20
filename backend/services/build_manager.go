package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"sammcore-deployer/secrets"
	"sammcore-deployer/storage"
)

// BuildManager gestiona la construcción de imágenes de Docker mediante Jobs de Kaniko en K3s
// y las empuja al registro local.
type BuildManager struct {
	kubeClient  kubernetes.Interface
	registryURL string
}

func NewBuildManager(kubeClient kubernetes.Interface) *BuildManager {
	registryURL := os.Getenv("REGISTRY_URL")
	if registryURL == "" {
		registryURL = "registry.sammcore-registry.svc.cluster.local:5000"
	}
	return &BuildManager{kubeClient: kubeClient, registryURL: registryURL}
}

// imageExistsInRegistry comprueba si un tag de imagen ya existe en el registro local
func (bm *BuildManager) imageExistsInRegistry(project, service, tag string) bool {
	checkURL := fmt.Sprintf("http://%s/v2/%s/%s/manifests/%s", bm.registryURL, project, service, tag)
	resp, err := http.Head(checkURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// EnsureImages construye las imágenes de Docker para cada servicio a través de Jobs de Kaniko
// en el namespace deployer-builds.
// Devuelve un mapa de nombre de servicio -> referencia completa de la imagen.
// Formato de tag: <registryURL>/<project>/<service>:<commit[:12]>
// Si el tag ya existe en el registro, omite la construcción.
func (bm *BuildManager) EnsureImages(ctx context.Context, p storage.Project, services []ServiceSpec, commit string) (map[string]string, error) {
	// 1. Derivar el tag y comprobar qué servicios necesitan build
	images := make(map[string]string)
	shortCommit := commit
	if len(shortCommit) > 12 {
		shortCommit = shortCommit[:12]
	}

	buildsNeeded := make(map[string]ServiceSpec)

	for _, s := range services {
		if s.BuildContext == "" || s.Dockerfile == "" {
			// omitir si no tiene build context o dockerfile
			continue
		}

		fullImageRef := fmt.Sprintf("%s/%s/%s:%s", bm.registryURL, p.Name, s.Name, shortCommit)

		if bm.imageExistsInRegistry(p.Name, s.Name, shortCommit) {
			log.Printf("Imagen ya existe en el registro para %s/%s:%s, omitiendo build", p.Name, s.Name, shortCommit)
			images[s.Name] = fullImageRef
			continue
		}

		images[s.Name] = fullImageRef
		buildsNeeded[s.Name] = s
	}

	if len(buildsNeeded) == 0 {
		return images, nil
	}

	// 3. Asegurar que el namespace deployer-builds existe
	nsName := "deployer-builds"
	_, err := bm.kubeClient.CoreV1().Namespaces().Get(ctx, nsName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err = bm.kubeClient.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsName,
					Labels: map[string]string{
						"app.kubernetes.io/managed-by": "sammcore-deployer",
					},
				},
			}, metav1.CreateOptions{})
			if err != nil && !errors.IsAlreadyExists(err) {
				return nil, fmt.Errorf("error al crear el namespace %s: %v", nsName, err)
			}
		} else {
			return nil, fmt.Errorf("error al obtener el namespace %s: %v", nsName, err)
		}
	}

	// 4. Crear los Jobs de Kaniko
	repoURL := p.Repo
	githubToken := secrets.GetGithubToken()
	if githubToken != "" {
		parsedURL, err := url.Parse(repoURL)
		if err == nil {
			parsedURL.User = url.UserPassword("x-access-token", githubToken)
			repoURL = parsedURL.String()
		}
	}

	jobNames := make(map[string]string) // map service name -> job name

	for sName, sSpec := range buildsNeeded {
		jobNameCommit := commit
		if len(jobNameCommit) > 8 {
			jobNameCommit = jobNameCommit[:8]
		}
		jobName := fmt.Sprintf("build-%s-%s-%s", p.Name, sName, jobNameCommit)
		if len(jobName) > 63 {
			jobName = jobName[:63]
			jobName = strings.TrimSuffix(jobName, "-")
		}

		jobNames[sName] = jobName
		fullImageRef := images[sName]

		ttlSeconds := int32(600)
		backoffLimit := int32(0)

		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: nsName,
			},
			Spec: batchv1.JobSpec{
				TTLSecondsAfterFinished: &ttlSeconds,
				BackoffLimit:            &backoffLimit,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"job-name": jobName,
						},
					},
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Volumes: []corev1.Volume{
							{
								Name: "workspace",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						InitContainers: []corev1.Container{
							{
								Name:  "git-clone",
								Image: "alpine/git:latest",
								Command: []string{
									"sh", "-c",
									fmt.Sprintf("git clone --depth 1 %s /workspace && cd /workspace && git fetch --depth 1 origin %s && git checkout %s", repoURL, commit, commit),
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "workspace",
										MountPath: "/workspace",
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:  "kaniko",
								Image: "gcr.io/kaniko-project/executor:latest",
								Args: []string{
									"--context=dir:///workspace",
									"--context-sub-path=" + sSpec.BuildContext,
									"--dockerfile=" + sSpec.Dockerfile,
									"--destination=" + fullImageRef,
									"--insecure",
									"--skip-tls-verify",
									"--cache=true",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "workspace",
										MountPath: "/workspace",
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("100m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("1000m"),
										corev1.ResourceMemory: resource.MustParse("1Gi"),
									},
								},
							},
						},
					},
				},
			},
		}

		_, err := bm.kubeClient.BatchV1().Jobs(nsName).Create(ctx, job, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("error al crear el Job de Kaniko para %s: %v", sName, err)
		}
	}

	// 5. Esperar a que se completen los Jobs
	timeout := time.After(15 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	completedJobs := make(map[string]bool)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeout:
			return nil, fmt.Errorf("timeout de 15 minutos alcanzado esperando los builds")
		case <-ticker.C:
			// Actualizar estado del proyecto a construyendo
			p.Status = storage.StatusBuilding
			_ = storage.AddOrUpdateProject(p)

			allDone := true
			for sName, jobName := range jobNames {
				if completedJobs[sName] {
					continue
				}

				job, err := bm.kubeClient.BatchV1().Jobs(nsName).Get(ctx, jobName, metav1.GetOptions{})
				if err != nil {
					log.Printf("Error al obtener el Job %s: %v", jobName, err)
					continue
				}

				if job.Status.Succeeded > 0 {
					completedJobs[sName] = true
					// 8. Limpiar Jobs completados con éxito (en realidad el TTL los limpiará, pero podemos forzar)
					policy := metav1.DeletePropagationBackground
					_ = bm.kubeClient.BatchV1().Jobs(nsName).Delete(ctx, jobName, metav1.DeleteOptions{
						PropagationPolicy: &policy,
					})
				} else if job.Status.Failed > 0 {
					// 7. Si falla, obtener logs del pod fallido
					logs, logErr := bm.getJobPodLogs(ctx, nsName, jobName)
					if logErr != nil {
						logs = fmt.Sprintf("no se pudieron obtener los logs: %v", logErr)
					}
					p.LastError = logs
					_ = storage.AddOrUpdateProject(p)
					return nil, fmt.Errorf("el build para el servicio %s falló: %s", sName, logs)
				} else {
					allDone = false
				}
			}

			if allDone {
				return images, nil
			}
		}
	}
}

// getJobPodLogs obtiene los logs del contenedor kaniko del pod asociado a un Job
func (bm *BuildManager) getJobPodLogs(ctx context.Context, namespace, jobName string) (string, error) {
	labelSelector := fmt.Sprintf("job-name=%s", jobName)
	pods, err := bm.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", err
	}
	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no se encontraron pods para el job %s", jobName)
	}

	podName := pods.Items[0].Name
	req := bm.kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "kaniko",
	})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer podLogs.Close()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
