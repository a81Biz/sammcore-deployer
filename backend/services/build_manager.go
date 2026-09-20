package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
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

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// stripANSI remueve los caracteres y códigos de escape de color ANSI emitidos por Kaniko
func stripANSI(str string) string {
	return ansiRegex.ReplaceAllString(str, "")
}

// sanitizeBuildLogs limpia el volcado excesivo de APT/debconf y conserva solo las líneas críticas del build
func sanitizeBuildLogs(raw string, isOOM bool, sName string) string {
	cleaned := stripANSI(raw)
	lines := strings.Split(cleaned, "\n")
	var keptLines []string

	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		// Descartar ruido de instalación de paquetes Debian/APT
		if strings.HasPrefix(trimmed, "Setting up ") ||
			strings.HasPrefix(trimmed, "Unpacking ") ||
			strings.HasPrefix(trimmed, "Selecting previously unselected ") ||
			strings.HasPrefix(trimmed, "Preparing to unpack ") ||
			strings.HasPrefix(trimmed, "Get:") ||
			strings.HasPrefix(trimmed, "debconf: ") ||
			strings.HasPrefix(trimmed, "Processing triggers for ") ||
			strings.HasPrefix(trimmed, "Reading database ") {
			continue
		}
		keptLines = append(keptLines, l)
	}

	// Mantener únicamente las últimas 30 líneas más relevantes
	if len(keptLines) > 30 {
		keptLines = keptLines[len(keptLines)-30:]
	}

	result := strings.Join(keptLines, "\n")
	if isOOM {
		header := fmt.Sprintf("💥 ERROR: El build del servicio '%s' superó el límite de memoria asignado (OOMKilled).\nEl contenedor fue terminado por el kernel al exceder la RAM configurada durante el snapshot del filesystem.\n\nÚltimos logs relevantes antes del fallo:\n", sName)
		result = header + result
	}
	return result
}

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

// EnsureImages construye las imágenes de Docker para cada servicio de forma secuencial
// a través de Jobs de Kaniko en el namespace deployer-builds.
func (bm *BuildManager) EnsureImages(ctx context.Context, p storage.Project, services []ServiceSpec, commit string) (map[string]string, error) {
	images := make(map[string]string)
	shortCommit := commit
	if len(shortCommit) > 12 {
		shortCommit = shortCommit[:12]
	}

	type buildTask struct {
		name string
		spec ServiceSpec
	}
	var buildsNeeded []buildTask

	for _, s := range services {
		if s.BuildContext == "" || s.Dockerfile == "" {
			continue
		}

		fullImageRef := fmt.Sprintf("%s/%s/%s:%s", bm.registryURL, p.Name, s.Name, shortCommit)

		if bm.imageExistsInRegistry(p.Name, s.Name, shortCommit) {
			log.Printf("Imagen ya existe en el registro para %s/%s:%s, omitiendo build", p.Name, s.Name, shortCommit)
			images[s.Name] = fullImageRef
			continue
		}

		images[s.Name] = fullImageRef
		buildsNeeded = append(buildsNeeded, buildTask{name: s.Name, spec: s})
	}

	if len(buildsNeeded) == 0 {
		return images, nil
	}

	// Asegurar que el namespace deployer-builds existe
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

	repoURL := p.Repo
	githubToken := secrets.GetGithubToken()
	if githubToken != "" {
		parsedURL, err := url.Parse(repoURL)
		if err == nil {
			parsedURL.User = url.UserPassword("x-access-token", githubToken)
			repoURL = parsedURL.String()
		}
	}

	// Compilación secuencial servicio por servicio
	totalBuilds := len(buildsNeeded)
	for idx, task := range buildsNeeded {
		sName := task.name
		sSpec := task.spec
		buildIndex := idx + 1

		jobNameCommit := commit
		if len(jobNameCommit) > 8 {
			jobNameCommit = jobNameCommit[:8]
		}
		jobName := fmt.Sprintf("build-%s-%s-%s", p.Name, sName, jobNameCommit)
		if len(jobName) > 63 {
			jobName = jobName[:63]
			jobName = strings.TrimSuffix(jobName, "-")
		}

		fullImageRef := images[sName]

		// Actualizar estado en storage en tiempo real
		p.Status = storage.StatusBuilding
		if p.TotalSteps > 0 {
			p.StepDescription = fmt.Sprintf("Paso %d/%d: Compilando imagen de '%s' (%d/%d)...", p.CurrentStep, p.TotalSteps, sName, buildIndex, totalBuilds)
		} else {
			p.StepDescription = fmt.Sprintf("Compilando imagen de '%s' (%d/%d)...", sName, buildIndex, totalBuilds)
		}
		_ = storage.AddOrUpdateProject(p)

		ttlSeconds := int32(300)
		backoffLimit := int32(0)

		job := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      jobName,
				Namespace: nsName,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by": "sammcore-deployer",
					"project":                      p.Name,
					"service":                      sName,
				},
			},
			Spec: batchv1.JobSpec{
				TTLSecondsAfterFinished: &ttlSeconds,
				BackoffLimit:            &backoffLimit,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"job-name":                     jobName,
							"app.kubernetes.io/managed-by": "sammcore-deployer",
							"project":                      p.Name,
							"service":                      sName,
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
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("50m"),
										corev1.ResourceMemory: resource.MustParse("64Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("256Mi"),
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
									"--snapshot-mode=redo",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "workspace",
										MountPath: "/workspace",
									},
								},
								Resources: corev1.ResourceRequirements{
									Requests: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("200m"),
										corev1.ResourceMemory: resource.MustParse("512Mi"),
									},
									Limits: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("2500m"),
										corev1.ResourceMemory: resource.MustParse("3500Mi"),
									},
								},
							},
						},
					},
				},
			},
		}

		// Eliminar Job previo si existía con el mismo nombre
		_ = bm.kubeClient.BatchV1().Jobs(nsName).Delete(ctx, jobName, metav1.DeleteOptions{
			PropagationPolicy: func() *metav1.DeletionPropagation { p := metav1.DeletePropagationBackground; return &p }(),
		})

		_, err := bm.kubeClient.BatchV1().Jobs(nsName).Create(ctx, job, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return nil, fmt.Errorf("error al crear el Job de Kaniko para %s: %v", sName, err)
		}

		// Monitorear finalización del Job (timeout 15 minutos)
		jobTimeout := time.After(15 * time.Minute)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		jobFinished := false
		for !jobFinished {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-jobTimeout:
				return nil, fmt.Errorf("timeout de 15 minutos alcanzado esperando el build de %s", sName)
			case <-ticker.C:
				currentJob, err := bm.kubeClient.BatchV1().Jobs(nsName).Get(ctx, jobName, metav1.GetOptions{})
				if err != nil {
					log.Printf("Error al verificar Job %s: %v", jobName, err)
					continue
				}

				if currentJob.Status.Succeeded > 0 {
					jobFinished = true
					policy := metav1.DeletePropagationBackground
					_ = bm.kubeClient.BatchV1().Jobs(nsName).Delete(ctx, jobName, metav1.DeleteOptions{
						PropagationPolicy: &policy,
					})
				} else if currentJob.Status.Failed > 0 {
					failLogs, logErr := bm.getJobFailureDetails(ctx, nsName, jobName, sName)
					if logErr != nil {
						failLogs = fmt.Sprintf("Error en build de %s (no se pudieron obtener logs: %v)", sName, logErr)
					}
					p.LastError = failLogs
					_ = storage.AddOrUpdateProject(p)
					return nil, fmt.Errorf("%s", failLogs)
				}
			}
		}
	}

	return images, nil
}

// getJobFailureDetails obtiene el log sanitizado y determina si ocurrió un OOM
func (bm *BuildManager) getJobFailureDetails(ctx context.Context, namespace, jobName, sName string) (string, error) {
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

	pod := pods.Items[0]
	isOOM := false

	for _, cs := range append(pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses...) {
		if cs.State.Terminated != nil {
			if cs.State.Terminated.Reason == "OOMKilled" || cs.State.Terminated.ExitCode == 137 {
				isOOM = true
				break
			}
		}
	}

	containerToLog := "kaniko"
	for _, ics := range pod.Status.InitContainerStatuses {
		if ics.State.Terminated != nil && ics.State.Terminated.ExitCode != 0 {
			containerToLog = ics.Name
			break
		}
	}

	tail := int64(80)
	req := bm.kubeClient.CoreV1().Pods(namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: containerToLog,
		TailLines: &tail,
	})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		if isOOM {
			return fmt.Sprintf("💥 ERROR: El contenedor '%s' del servicio '%s' fue terminado por falta de memoria (OOMKilled - límite de RAM excedido).", containerToLog, sName), nil
		}
		return "", err
	}
	defer podLogs.Close()

	buf := new(strings.Builder)
	_, _ = io.Copy(buf, podLogs)

	return sanitizeBuildLogs(buf.String(), isOOM, sName), nil
}

// GetActiveBuildLogs retorna los logs en tiempo real del pod de Kaniko que esté compilando actualmente para este proyecto
func (bm *BuildManager) GetActiveBuildLogs(ctx context.Context, projectName string) (string, error) {
	pods, err := bm.kubeClient.CoreV1().Pods("deployer-builds").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", err
	}

	prefix := fmt.Sprintf("build-%s-", projectName)
	var latestPod *corev1.Pod

	for i := range pods.Items {
		pod := &pods.Items[i]
		if strings.HasPrefix(pod.Name, prefix) {
			if latestPod == nil || pod.CreationTimestamp.After(latestPod.CreationTimestamp.Time) {
				latestPod = pod
			}
		}
	}

	if latestPod == nil {
		return "No hay tareas de compilación activas en deployer-builds para este proyecto.", nil
	}

	for _, ics := range latestPod.Status.InitContainerStatuses {
		if ics.Name == "git-clone" && (ics.State.Running != nil || ics.State.Waiting != nil) {
			return fmt.Sprintf("Pod %s: Clonando código fuente desde GitHub...", latestPod.Name), nil
		}
	}

	tail := int64(50)
	req := bm.kubeClient.CoreV1().Pods("deployer-builds").GetLogs(latestPod.Name, &corev1.PodLogOptions{
		Container: "kaniko",
		TailLines: &tail,
	})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return fmt.Sprintf("Pod %s (%s): Inicializando ejecutor Kaniko...", latestPod.Name, latestPod.Status.Phase), nil
	}
	defer podLogs.Close()

	buf := new(strings.Builder)
	_, _ = io.Copy(buf, podLogs)

	cleaned := stripANSI(buf.String())
	lines := strings.Split(cleaned, "\n")
	var kept []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "Setting up ") ||
			strings.HasPrefix(trimmed, "Unpacking ") ||
			strings.HasPrefix(trimmed, "Selecting previously unselected ") ||
			strings.HasPrefix(trimmed, "Preparing to unpack ") ||
			strings.HasPrefix(trimmed, "Get:") ||
			strings.HasPrefix(trimmed, "debconf: ") ||
			strings.HasPrefix(trimmed, "Processing triggers for ") ||
			strings.HasPrefix(trimmed, "Reading database ") {
			continue
		}
		kept = append(kept, l)
	}

	if len(kept) == 0 {
		return fmt.Sprintf("Pod %s: Iniciando construcción de imagen...", latestPod.Name), nil
	}

	if len(kept) > 35 {
		kept = kept[len(kept)-35:]
	}

	return strings.Join(kept, "\n"), nil
}
