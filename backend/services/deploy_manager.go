package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	"sammcore-deployer/storage"
)

var validProjectName = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// deployLocks es un mutex por proyecto para evitar deploys/redeploys concurrentes
var deployLocks sync.Map

func getProjectLock(name string) *sync.Mutex {
	val, _ := deployLocks.LoadOrStore(name, &sync.Mutex{})
	return val.(*sync.Mutex)
}

type DeployRequest struct {
	Name             string            `json:"name"`
	Repo             string            `json:"repo"`
	Branch           string            `json:"branch,omitempty"`
	Type             string            `json:"type,omitempty"`
	Commit           string            `json:"commit,omitempty"`
	RequiresDatabase *bool             `json:"requires_database,omitempty"`
	Services         []ServiceSpec     `json:"services,omitempty"`
	BuildArgs        map[string]string `json:"build_args,omitempty"`
}

type DeployManager struct {
	kubeClient      kubernetes.Interface
	secretManager   *SecretManager
	templateManager *TemplateManager
	buildManager    *BuildManager
}

func NewDeployManager(kubeClient kubernetes.Interface) *DeployManager {
	return &DeployManager{
		kubeClient:      kubeClient,
		secretManager:   NewSecretManager(kubeClient),
		templateManager: NewTemplateManager(),
		buildManager:    NewBuildManager(kubeClient),
	}
}

// ValidateProjectName valida que el nombre cumpla RFC 1123, longitud 3-35, y no sea reservado
func ValidateProjectName(name string) error {
	if len(name) < 3 || len(name) > 35 {
		return fmt.Errorf("el nombre del proyecto debe tener entre 3 y 35 caracteres (actual: %d)", len(name))
	}
	if !validProjectName.MatchString(name) {
		return fmt.Errorf("el nombre del proyecto '%s' no cumple RFC 1123: solo minúsculas, dígitos y guiones, sin empezar ni terminar en guión", name)
	}
	if isReservedProjectName(name) {
		return fmt.Errorf("el nombre '%s' está reservado y no puede usarse como proyecto", name)
	}
	if strings.HasSuffix(name, "-api") || strings.HasSuffix(name, "-docs") {
		return fmt.Errorf("el nombre '%s' no puede terminar en '-api' ni '-docs' para evitar colisiones de subdominios", name)
	}
	return nil
}

func isReservedProjectName(name string) bool {
	reserved := map[string]bool{
		"deployer": true, "supabase": true, "monitoring": true,
		"ingress-nginx": true, "default": true, "api": true,
		"docs": true, "admin": true, "grafana": true,
		"prometheus": true, "traefik": true, "portainer": true,
		"sammcore-registry": true, "deployer-builds": true,
	}
	if strings.HasPrefix(name, "kube-") {
		return true
	}
	return reserved[name]
}

// ApplyManifestsYAML deserializa un YAML multi-documento y aplica los recursos en K8s.
// A diferencia de la versión anterior, ahora retorna error en unmarshal fallidos y kinds no soportados.
func (dm *DeployManager) ApplyManifestsYAML(ctx context.Context, rawYAML string) error {
	docs := strings.Split(rawYAML, "\n---")
	for i, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed == "" {
			continue
		}

		var typeMeta metav1.TypeMeta
		if err := k8syaml.Unmarshal([]byte(trimmed), &typeMeta); err != nil {
			return fmt.Errorf("error al deserializar TypeMeta en documento %d: %w", i, err)
		}

		switch typeMeta.Kind {
		case "Namespace":
			var ns corev1.Namespace
			if err := k8syaml.Unmarshal([]byte(trimmed), &ns); err != nil {
				return fmt.Errorf("error al deserializar Namespace: %w", err)
			}
			if ns.Name != "" {
				if err := dm.applyNamespace(ctx, &ns); err != nil {
					return fmt.Errorf("error al aplicar Namespace %s: %w", ns.Name, err)
				}
			}
		case "ResourceQuota":
			var rq corev1.ResourceQuota
			if err := k8syaml.Unmarshal([]byte(trimmed), &rq); err != nil {
				return fmt.Errorf("error al deserializar ResourceQuota: %w", err)
			}
			if rq.Name != "" {
				if err := dm.applyResourceQuota(ctx, &rq); err != nil {
					return fmt.Errorf("error al aplicar ResourceQuota %s: %w", rq.Name, err)
				}
			}
		case "LimitRange":
			var lr corev1.LimitRange
			if err := k8syaml.Unmarshal([]byte(trimmed), &lr); err != nil {
				return fmt.Errorf("error al deserializar LimitRange: %w", err)
			}
			if lr.Name != "" {
				if err := dm.applyLimitRange(ctx, &lr); err != nil {
					return fmt.Errorf("error al aplicar LimitRange %s: %w", lr.Name, err)
				}
			}
		case "NetworkPolicy":
			var np networkingv1.NetworkPolicy
			if err := k8syaml.Unmarshal([]byte(trimmed), &np); err != nil {
				return fmt.Errorf("error al deserializar NetworkPolicy: %w", err)
			}
			if np.Name != "" {
				if err := dm.applyNetworkPolicy(ctx, &np); err != nil {
					return fmt.Errorf("error al aplicar NetworkPolicy %s: %w", np.Name, err)
				}
			}
		case "Deployment":
			var dep appsv1.Deployment
			if err := k8syaml.Unmarshal([]byte(trimmed), &dep); err != nil {
				return fmt.Errorf("error al deserializar Deployment: %w", err)
			}
			if dep.Name != "" {
				if err := dm.applyDeployment(ctx, &dep); err != nil {
					return fmt.Errorf("error al aplicar Deployment %s: %w", dep.Name, err)
				}
			}
		case "Service":
			var svc corev1.Service
			if err := k8syaml.Unmarshal([]byte(trimmed), &svc); err != nil {
				return fmt.Errorf("error al deserializar Service: %w", err)
			}
			if svc.Name != "" {
				if err := dm.applyService(ctx, &svc); err != nil {
					return fmt.Errorf("error al aplicar Service %s: %w", svc.Name, err)
				}
			}
		case "Ingress":
			var ing networkingv1.Ingress
			if err := k8syaml.Unmarshal([]byte(trimmed), &ing); err != nil {
				return fmt.Errorf("error al deserializar Ingress: %w", err)
			}
			if ing.Name != "" {
				if err := dm.applyIngress(ctx, &ing); err != nil {
					return fmt.Errorf("error al aplicar Ingress %s: %w", ing.Name, err)
				}
			}
		case "Job":
			var job batchv1.Job
			if err := k8syaml.Unmarshal([]byte(trimmed), &job); err != nil {
				return fmt.Errorf("error al deserializar Job: %w", err)
			}
			if job.Name != "" {
				if err := dm.applyJob(ctx, &job); err != nil {
					return fmt.Errorf("error al aplicar Job %s: %w", job.Name, err)
				}
			}
		case "":
			continue // Documento vacío
		default:
			return fmt.Errorf("tipo de recurso Kubernetes no soportado: %s (documento %d)", typeMeta.Kind, i)
		}
	}
	return nil
}

// applyNamespace verifica ownership antes de crear/actualizar.
// Rechaza namespaces que existen sin la label managed-by=sammcore-deployer.
func (dm *DeployManager) applyNamespace(ctx context.Context, ns *corev1.Namespace) error {
	existing, err := dm.kubeClient.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			return err
		}
		return err
	}

	// Verificar ownership: solo actualizar si el NS fue creado por sammcore-deployer
	if existing.Labels["app.kubernetes.io/managed-by"] != "sammcore-deployer" {
		return fmt.Errorf("el namespace '%s' existe pero no está gestionado por sammcore-deployer (falta label managed-by). Operación rechazada por seguridad", ns.Name)
	}

	ns.ResourceVersion = existing.ResourceVersion
	_, err = dm.kubeClient.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
	return err
}

func (dm *DeployManager) applyResourceQuota(ctx context.Context, rq *corev1.ResourceQuota) error {
	existing, err := dm.kubeClient.CoreV1().ResourceQuotas(rq.Namespace).Get(ctx, rq.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.CoreV1().ResourceQuotas(rq.Namespace).Create(ctx, rq, metav1.CreateOptions{})
			return err
		}
		return err
	}
	rq.ResourceVersion = existing.ResourceVersion
	_, err = dm.kubeClient.CoreV1().ResourceQuotas(rq.Namespace).Update(ctx, rq, metav1.UpdateOptions{})
	return err
}

func (dm *DeployManager) applyLimitRange(ctx context.Context, lr *corev1.LimitRange) error {
	existing, err := dm.kubeClient.CoreV1().LimitRanges(lr.Namespace).Get(ctx, lr.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.CoreV1().LimitRanges(lr.Namespace).Create(ctx, lr, metav1.CreateOptions{})
			return err
		}
		return err
	}
	lr.ResourceVersion = existing.ResourceVersion
	_, err = dm.kubeClient.CoreV1().LimitRanges(lr.Namespace).Update(ctx, lr, metav1.UpdateOptions{})
	return err
}

func (dm *DeployManager) applyNetworkPolicy(ctx context.Context, np *networkingv1.NetworkPolicy) error {
	existing, err := dm.kubeClient.NetworkingV1().NetworkPolicies(np.Namespace).Get(ctx, np.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.NetworkingV1().NetworkPolicies(np.Namespace).Create(ctx, np, metav1.CreateOptions{})
			return err
		}
		return err
	}
	np.ResourceVersion = existing.ResourceVersion
	_, err = dm.kubeClient.NetworkingV1().NetworkPolicies(np.Namespace).Update(ctx, np, metav1.UpdateOptions{})
	return err
}

func (dm *DeployManager) applyDeployment(ctx context.Context, dep *appsv1.Deployment) error {
	existing, err := dm.kubeClient.AppsV1().Deployments(dep.Namespace).Get(ctx, dep.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.AppsV1().Deployments(dep.Namespace).Create(ctx, dep, metav1.CreateOptions{})
			return err
		}
		return err
	}
	dep.ResourceVersion = existing.ResourceVersion
	_, err = dm.kubeClient.AppsV1().Deployments(dep.Namespace).Update(ctx, dep, metav1.UpdateOptions{})
	return err
}

func (dm *DeployManager) applyService(ctx context.Context, svc *corev1.Service) error {
	existing, err := dm.kubeClient.CoreV1().Services(svc.Namespace).Get(ctx, svc.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.CoreV1().Services(svc.Namespace).Create(ctx, svc, metav1.CreateOptions{})
			return err
		}
		return err
	}
	svc.ResourceVersion = existing.ResourceVersion
	svc.Spec.ClusterIP = existing.Spec.ClusterIP
	_, err = dm.kubeClient.CoreV1().Services(svc.Namespace).Update(ctx, svc, metav1.UpdateOptions{})
	return err
}

func (dm *DeployManager) applyIngress(ctx context.Context, ing *networkingv1.Ingress) error {
	existing, err := dm.kubeClient.NetworkingV1().Ingresses(ing.Namespace).Get(ctx, ing.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.NetworkingV1().Ingresses(ing.Namespace).Create(ctx, ing, metav1.CreateOptions{})
			return err
		}
		return err
	}
	ing.ResourceVersion = existing.ResourceVersion
	_, err = dm.kubeClient.NetworkingV1().Ingresses(ing.Namespace).Update(ctx, ing, metav1.UpdateOptions{})
	return err
}

func (dm *DeployManager) applyJob(ctx context.Context, job *batchv1.Job) error {
	existing, err := dm.kubeClient.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
			return err
		}
		return err
	}
	// Los Jobs no se pueden actualizar; eliminar y recrear
	propagation := metav1.DeletePropagationBackground
	_ = dm.kubeClient.BatchV1().Jobs(job.Namespace).Delete(ctx, existing.Name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	time.Sleep(2 * time.Second)
	_, err = dm.kubeClient.BatchV1().Jobs(job.Namespace).Create(ctx, job, metav1.CreateOptions{})
	return err
}

// GetPodLogs retorna los logs recientes del pod principal de un proyecto
func (dm *DeployManager) GetPodLogs(ctx context.Context, namespace string, tailLines int64) (string, error) {
	pods, err := dm.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("error al listar pods en %s: %w", namespace, err)
	}

	if len(pods.Items) == 0 {
		return "No se encontraron pods activos en el namespace " + namespace, nil
	}

	targetPod := pods.Items[0].Name

	req := dm.kubeClient.CoreV1().Pods(namespace).GetLogs(targetPod, &corev1.PodLogOptions{
		TailLines: &tailLines,
	})

	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("error al obtener stream de logs de pod %s: %w", targetPod, err)
	}
	defer podLogs.Close()

	buf := new(bytes.Buffer)
	if _, err := io.Copy(buf, podLogs); err != nil {
		return "", fmt.Errorf("error al leer stream de logs: %w", err)
	}

	return buf.String(), nil
}

type ContainerMetricInfo struct {
	Name        string `json:"name"`
	CPUUsage    string `json:"cpu_usage"`
	MemoryUsage string `json:"memory_usage"`
}

type PodMetricInfo struct {
	Name        string                `json:"name"`
	Status      string                `json:"status"`
	Ready       bool                  `json:"ready"`
	Restarts    int32                 `json:"restarts"`
	CPUUsage    string                `json:"cpu_usage"`
	MemoryUsage string                `json:"memory_usage"`
	Containers  []ContainerMetricInfo `json:"containers,omitempty"`
}

type ProjectMetrics struct {
	ProjectID   string            `json:"project_id"`
	ProjectName string            `json:"project_name"`
	Namespace   string            `json:"namespace"`
	Status      string            `json:"status"`
	PodsCount   int               `json:"pods_count"`
	Pods        []PodMetricInfo   `json:"pods"`
	QuotaUsage  map[string]string `json:"quota_usage,omitempty"`
	QuotaLimits map[string]string `json:"quota_limits,omitempty"`
}

// GetProjectMetrics consulta dinámicamente el estado y recursos de los pods de un proyecto en K8s
func (dm *DeployManager) GetProjectMetrics(ctx context.Context, p storage.Project) (*ProjectMetrics, error) {
	metrics := &ProjectMetrics{
		ProjectID:   p.ID,
		ProjectName: p.Name,
		Namespace:   p.Namespace,
		Status:      string(p.Status),
		Pods:        make([]PodMetricInfo, 0),
		QuotaUsage:  make(map[string]string),
		QuotaLimits: make(map[string]string),
	}

	pods, err := dm.kubeClient.CoreV1().Pods(p.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listando pods en %s: %w", p.Namespace, err)
	}

	metrics.PodsCount = len(pods.Items)

	type rawMetricList struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Containers []struct {
				Name  string `json:"name"`
				Usage struct {
					CPU    string `json:"cpu"`
					Memory string `json:"memory"`
				} `json:"usage"`
			} `json:"containers"`
		} `json:"items"`
	}

	rawMetricsMap := make(map[string]map[string]ContainerMetricInfo)
	if dm.kubeClient.Discovery() != nil && dm.kubeClient.Discovery().RESTClient() != nil {
		rawBytes, errRaw := dm.kubeClient.Discovery().RESTClient().Get().AbsPath("/apis/metrics.k8s.io/v1beta1/namespaces/" + p.Namespace + "/pods").DoRaw(ctx)
		if errRaw == nil && len(rawBytes) > 0 {
			var rml rawMetricList
			if json.Unmarshal(rawBytes, &rml) == nil {
				for _, item := range rml.Items {
					cmMap := make(map[string]ContainerMetricInfo)
					for _, c := range item.Containers {
						cmMap[c.Name] = ContainerMetricInfo{
							Name:        c.Name,
							CPUUsage:    c.Usage.CPU,
							MemoryUsage: c.Usage.Memory,
						}
					}
					rawMetricsMap[item.Metadata.Name] = cmMap
				}
			}
		}
	}

	for _, pod := range pods.Items {
		var totalRestarts int32
		allReady := true
		for _, cs := range pod.Status.ContainerStatuses {
			totalRestarts += cs.RestartCount
			if !cs.Ready {
				allReady = false
			}
		}

		podInfo := PodMetricInfo{
			Name:       pod.Name,
			Status:     string(pod.Status.Phase),
			Ready:      allReady && len(pod.Status.ContainerStatuses) > 0,
			Restarts:   totalRestarts,
			Containers: make([]ContainerMetricInfo, 0),
		}

		if cMap, exists := rawMetricsMap[pod.Name]; exists {
			for _, cMetric := range cMap {
				podInfo.Containers = append(podInfo.Containers, cMetric)
				if podInfo.CPUUsage == "" {
					podInfo.CPUUsage = cMetric.CPUUsage
					podInfo.MemoryUsage = cMetric.MemoryUsage
				}
			}
		} else {
			podInfo.CPUUsage = "0m"
			podInfo.MemoryUsage = "0Mi"
		}

		metrics.Pods = append(metrics.Pods, podInfo)
	}

	quotas, errQ := dm.kubeClient.CoreV1().ResourceQuotas(p.Namespace).List(ctx, metav1.ListOptions{})
	if errQ == nil && len(quotas.Items) > 0 {
		q := quotas.Items[0]
		for k, v := range q.Status.Used {
			metrics.QuotaUsage[string(k)] = v.String()
		}
		for k, v := range q.Status.Hard {
			metrics.QuotaLimits[string(k)] = v.String()
		}
	}

	return metrics, nil
}

// DeleteProjectDeployment desmantela el namespace y condicionalmente la BD en Postgres central.
// Verifica ownership del namespace antes de eliminar.
func (dm *DeployManager) DeleteProjectDeployment(ctx context.Context, projectName string, deleteDB bool) error {
	// Verificar que el namespace pertenece a sammcore-deployer
	existing, err := dm.kubeClient.CoreV1().Namespaces().Get(ctx, projectName, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error al verificar namespace %s: %w", projectName, err)
	}
	if existing != nil && existing.Labels["app.kubernetes.io/managed-by"] != "sammcore-deployer" {
		return fmt.Errorf("el namespace '%s' no está gestionado por sammcore-deployer, eliminación rechazada", projectName)
	}

	// 1. Eliminar base de datos si se solicita
	if deleteDB {
		host, port, user, password, dbname := GetSupabaseConfig()
		if password != "" {
			if masterDB, err := OpenMasterDB(host, port, user, password, dbname); err == nil {
				_ = DeprovisionProjectDatabase(masterDB, projectName, true)
				_ = masterDB.Close()
			}
		}
	}

	// 2. Eliminar Namespace en Kubernetes
	err = dm.kubeClient.CoreV1().Namespaces().Delete(ctx, projectName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error al eliminar namespace %s: %w", projectName, err)
	}

	return nil
}

// updateProjectStatus actualiza el estado y opcionalmente el error del proyecto
// updateProjectProgress actualiza el estado, paso actual y descripción del despliegue
func updateProjectProgress(p *storage.Project, status storage.ProjectStatus, currentStep, totalSteps int, stepDesc, lastError string) {
	p.Status = status
	if currentStep > 0 {
		p.CurrentStep = currentStep
	}
	if totalSteps > 0 {
		p.TotalSteps = totalSteps
	}
	if stepDesc != "" {
		p.StepDescription = stepDesc
	}
	p.LastError = lastError
	p.UpdatedAt = time.Now()
	_ = storage.AddOrUpdateProject(*p)
}

func updateProjectStatus(p *storage.Project, status storage.ProjectStatus, lastError string) {
	updateProjectProgress(p, status, p.CurrentStep, p.TotalSteps, p.StepDescription, lastError)
}

// ExecuteDeploy ejecuta la secuencia completa de despliegue con mutex por proyecto:
// 1. Validar namespace (ownership)
// 2. Provisionar BD si necesita (Paso 1)
// 3. Construir imágenes vía Kaniko si hay servicios con código (Paso 2)
// 4. Renderizar y aplicar manifiestos K8s (Paso 3)
// 5. Monitorear rollout y readiness de pods (Paso 4)
func (dm *DeployManager) ExecuteDeploy(ctx context.Context, p storage.Project, params ProjectManifestParams) error {
	// Mutex por proyecto: evitar deploys concurrentes
	lock := getProjectLock(p.Name)
	lock.Lock()
	defer lock.Unlock()

	projectName := p.Name
	namespace := p.Namespace

	// Calcular total de pasos
	totalSteps := 3
	if params.RequiresDatabase {
		totalSteps = 4
	}
	p.TotalSteps = totalSteps
	currentStep := 1

	// 1. Si requiere base de datos, aprovisionar en PostgreSQL Supabase (Modelo A)
	if params.RequiresDatabase {
		desc := fmt.Sprintf("Paso %d/%d: Aprovisionando base de datos PostgreSQL en Supabase...", currentStep, totalSteps)
		updateProjectProgress(&p, storage.StatusProvisioning, currentStep, totalSteps, desc, "")

		host, port, user, password, dbname := GetSupabaseConfig()
		if password == "" {
			errMsg := "SUPABASE_POSTGRES_PASSWORD no establecida, no se puede aprovisionar BD"
			log.Printf("[Deploy] ⚠️ %s", errMsg)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Error al aprovisionar BD", errMsg)
			return fmt.Errorf(errMsg)
		}

		masterDB, err := OpenMasterDB(host, port, user, password, dbname)
		if err != nil {
			errMsg := fmt.Sprintf("error conectando a Supabase DB: %v", err)
			log.Printf("[Deploy] %s", errMsg)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Error de conexión a BD", errMsg)
			return fmt.Errorf(errMsg)
		}

		existingPass, _ := dm.secretManager.GetExistingDBPassword(ctx, namespace, projectName)
		if existingPass == "" && params.BuildArgs != nil {
			for _, k := range []string{"DB_PASSWORD", "POSTGRES_PASSWORD", "DATABASE_PASSWORD", "DB_PASS"} {
				if pw, ok := params.BuildArgs[k]; ok && strings.TrimSpace(pw) != "" {
					existingPass = strings.TrimSpace(pw)
					break
				}
			}
		}
		provRes, err := ProvisionProjectDatabase(masterDB, projectName, existingPass)
		_ = masterDB.Close()
		if err != nil {
			errMsg := fmt.Sprintf("fallo al aprovisionar BD: %v", err)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Error al aprovisionar BD", errMsg)
			return fmt.Errorf(errMsg)
		}

		// Crear namespace previo para poder inyectar el Secret
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":       "sammcore-deployer",
					"project":                            projectName,
					"pod-security.kubernetes.io/enforce": "baseline",
				},
			},
		}
		if err := dm.applyNamespace(ctx, nsObj); err != nil {
			errMsg := fmt.Sprintf("fallo al crear namespace para BD: %v", err)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Error al crear namespace", errMsg)
			return fmt.Errorf(errMsg)
		}

		if err := dm.secretManager.EnsureDBSecret(ctx, namespace, projectName, provRes); err != nil {
			errMsg := fmt.Sprintf("fallo al crear DB secret: %v", err)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Error al crear secreto de BD", errMsg)
			return fmt.Errorf(errMsg)
		}

		currentStep++
	}

	// Si hay variables de entorno personalizadas, asegurar namespace y secreto
	if params.HasCustomEnv && len(params.BuildArgs) > 0 {
		nsObj := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"app.kubernetes.io/managed-by":       "sammcore-deployer",
					"project":                            projectName,
					"pod-security.kubernetes.io/enforce": "baseline",
				},
			},
		}
		if err := dm.applyNamespace(ctx, nsObj); err != nil {
			errMsg := fmt.Sprintf("fallo al crear namespace para env secrets: %v", err)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Error al crear namespace", errMsg)
			return fmt.Errorf(errMsg)
		}
		if err := dm.secretManager.EnsureCustomEnvSecret(ctx, namespace, projectName, params.BuildArgs); err != nil {
			errMsg := fmt.Sprintf("fallo al crear env secret: %v", err)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Error al crear secretos personalizados", errMsg)
			return fmt.Errorf(errMsg)
		}
	}

	// 2. Construir imágenes vía Kaniko (verificar si hay servicios con BuildContext)
	needsBuild := false
	for _, s := range params.Services {
		if s.BuildContext != "" {
			needsBuild = true
			break
		}
	}

	if needsBuild {
		if p.Commit == "" {
			errMsg := "no se especificó el commit del repositorio de Git para compilar las imágenes requeridas"
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Error: falta commit de Git", errMsg)
			return fmt.Errorf(errMsg)
		}

		desc := fmt.Sprintf("Paso %d/%d: Compilando imágenes Docker con Kaniko...", currentStep, totalSteps)
		updateProjectProgress(&p, storage.StatusBuilding, currentStep, totalSteps, desc, "")

		images, err := dm.buildManager.EnsureImages(ctx, p, params.Services, p.Commit)
		if err != nil {
			errMsg := fmt.Sprintf("fallo al construir imágenes: %v", err)
			dm.RollbackDeployment(ctx, p, errMsg)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Rollback: fallo en compilación de imágenes", errMsg)
			return fmt.Errorf(errMsg)
		}

		if len(images) == 0 {
			errMsg := "no se generaron imágenes en el registro local para los servicios del proyecto"
			dm.RollbackDeployment(ctx, p, errMsg)
			updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Rollback: imágenes no generadas", errMsg)
			return fmt.Errorf(errMsg)
		}

		params.Images = images
		p.Images = images
		_ = storage.AddOrUpdateProject(p)
	}
	currentStep++

	// 3. Renderizar y aplicar manifiestos
	desc := fmt.Sprintf("Paso %d/%d: Aplicando manifiestos Kubernetes (Deployments, Services, Ingress)...", currentStep, totalSteps)
	updateProjectProgress(&p, storage.StatusDeploying, currentStep, totalSteps, desc, "")

	manifestsYAML, err := dm.templateManager.RenderAllManifests(params)
	if err != nil {
		errMsg := fmt.Sprintf("error renderizando manifiestos: %v", err)
		dm.RollbackDeployment(ctx, p, errMsg)
		updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Rollback: error al renderizar manifiestos", errMsg)
		return fmt.Errorf(errMsg)
	}

	if err := dm.ApplyManifestsYAML(ctx, manifestsYAML); err != nil {
		errMsg := fmt.Sprintf("error aplicando manifiestos en K8s: %v", err)
		dm.RollbackDeployment(ctx, p, errMsg)
		updateProjectProgress(&p, storage.StatusFailed, currentStep, totalSteps, "Rollback: error al aplicar manifiestos en K8s", errMsg)
		return fmt.Errorf(errMsg)
	}
	currentStep++

	// 4. Monitorear despliegue
	p.CurrentStep = currentStep
	go dm.monitorRollout(context.Background(), p)

	return nil
}

// RollbackDeployment elimina los recursos desplegados en K8s cuando ocurre un fallo o timeout,
// evitando que queden pods zombis, CrashLoopBackOff o trabajos de Kaniko residuales.
func (dm *DeployManager) RollbackDeployment(ctx context.Context, p storage.Project, reason string) {
	log.Printf("[Rollback] 🧹 Ejecutando rollback para el proyecto %s (Razón: %s)...", p.Name, reason)

	// 1. Limpiar cualquier Job residual de Kaniko en deployer-builds
	jobs, err := dm.kubeClient.BatchV1().Jobs("deployer-builds").List(ctx, metav1.ListOptions{})
	if err == nil {
		bgPolicy := metav1.DeletePropagationBackground
		prefix := fmt.Sprintf("build-%s-", p.Name)
		for _, j := range jobs.Items {
			if strings.HasPrefix(j.Name, prefix) {
				log.Printf("[Rollback] Eliminando Job de Kaniko residual %s", j.Name)
				_ = dm.kubeClient.BatchV1().Jobs("deployer-builds").Delete(ctx, j.Name, metav1.DeleteOptions{PropagationPolicy: &bgPolicy})
			}
		}
	}

	// 2. Eliminar Deployments en el namespace para detener pods inmediatamente
	deps, err := dm.kubeClient.AppsV1().Deployments(p.Namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		fgPolicy := metav1.DeletePropagationForeground
		for _, d := range deps.Items {
			log.Printf("[Rollback] Eliminando Deployment fallido %s/%s", p.Namespace, d.Name)
			_ = dm.kubeClient.AppsV1().Deployments(p.Namespace).Delete(ctx, d.Name, metav1.DeleteOptions{PropagationPolicy: &fgPolicy})
		}
	}

	// 3. Eliminar Services del namespace
	svcs, err := dm.kubeClient.CoreV1().Services(p.Namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, s := range svcs.Items {
			log.Printf("[Rollback] Eliminando Service %s/%s", p.Namespace, s.Name)
			_ = dm.kubeClient.CoreV1().Services(p.Namespace).Delete(ctx, s.Name, metav1.DeleteOptions{})
		}
	}

	// 4. Eliminar Ingress del namespace
	ings, err := dm.kubeClient.NetworkingV1().Ingresses(p.Namespace).List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, ing := range ings.Items {
			log.Printf("[Rollback] Eliminando Ingress %s/%s", p.Namespace, ing.Name)
			_ = dm.kubeClient.NetworkingV1().Ingresses(p.Namespace).Delete(ctx, ing.Name, metav1.DeleteOptions{})
		}
	}

	log.Printf("[Rollback] ✅ Limpieza de rollback completada para %s", p.Name)
}

// monitorRollout verifica el estado de los Deployments hasta que todos estén listos.
// Si excede el deadline, ejecuta RollbackDeployment y marca el proyecto como FAILED con el motivo detallado.
func (dm *DeployManager) monitorRollout(ctx context.Context, p storage.Project) {
	desc := fmt.Sprintf("Paso %d/%d: Verificando disponibilidad (readiness) de pods en K3s...", p.TotalSteps, p.TotalSteps)
	updateProjectProgress(&p, storage.StatusDeploying, p.TotalSteps, p.TotalSteps, desc, "")

	deadline := time.Now().Add(120 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(5 * time.Second)
		deps, err := dm.kubeClient.AppsV1().Deployments(p.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil || len(deps.Items) == 0 {
			continue
		}

		allReady := true
		for _, d := range deps.Items {
			if d.Status.ReadyReplicas < 1 {
				allReady = false
				break
			}
		}
		if allReady {
			updateProjectProgress(&p, storage.StatusRunning, p.TotalSteps, p.TotalSteps, "Despliegue completado exitosamente y operativo", "")
			log.Printf("[Deploy] Proyecto %s desplegado exitosamente en Running", p.Name)
			return
		}
	}

	// TIMEOUT: recopilar motivos de fallo de los pods
	failReason := dm.collectFailureReasons(ctx, p.Namespace)
	errMsg := fmt.Sprintf("timeout esperando rollout (120s): %s", failReason)
	log.Printf("[Deploy] ⚠️ Proyecto %s: %s", p.Name, errMsg)

	// Ejecutar rollback automático: eliminar deployments/pods rotos
	dm.RollbackDeployment(ctx, p, errMsg)
	updateProjectProgress(&p, storage.StatusFailed, p.TotalSteps, p.TotalSteps, "Rollback ejecutado: recursos limpiados tras fallo", errMsg)
}

// collectFailureReasons examina los pods del namespace y recopila los motivos de fallo
func (dm *DeployManager) collectFailureReasons(ctx context.Context, namespace string) string {
	pods, err := dm.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "no se pudieron listar pods"
	}

	var reasons []string
	for _, pod := range pods.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			if cs.Ready {
				continue
			}
			if cs.State.Waiting != nil {
				reason := cs.State.Waiting.Reason
				msg := cs.State.Waiting.Message
				entry := fmt.Sprintf("pod=%s container=%s reason=%s", pod.Name, cs.Name, reason)
				if msg != "" {
					entry += " msg=" + msg
				}
				reasons = append(reasons, entry)
			}
			if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
				reasons = append(reasons, fmt.Sprintf("pod=%s container=%s terminated exitCode=%d reason=%s",
					pod.Name, cs.Name, cs.State.Terminated.ExitCode, cs.State.Terminated.Reason))
			}
		}
	}

	if len(reasons) == 0 {
		return "pods no están listos (sin motivo específico reportado)"
	}
	return strings.Join(reasons, "; ")
}
