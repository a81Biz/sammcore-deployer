package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"

	"sammcore-deployer/storage"
)

type DeployRequest struct {
	Name             string            `json:"name"`
	Repo             string            `json:"repo"`
	Branch           string            `json:"branch,omitempty"`
	Type             string            `json:"type,omitempty"`
	RequiresDatabase *bool             `json:"requires_database,omitempty"`
	WebImage         string            `json:"web_image,omitempty"`
	APIImage         string            `json:"api_image,omitempty"`
	AppImage         string            `json:"app_image,omitempty"`
	StaticImage      string            `json:"static_image,omitempty"`
	WebPort          int               `json:"web_port,omitempty"`
	APIPort          int               `json:"api_port,omitempty"`
	AppPort          int               `json:"app_port,omitempty"`
	BuildArgs        map[string]string `json:"build_args,omitempty"`
}

type DeployManager struct {
	kubeClient      kubernetes.Interface
	secretManager   *SecretManager
	templateManager *TemplateManager
}

func NewDeployManager(kubeClient kubernetes.Interface) *DeployManager {
	return &DeployManager{
		kubeClient:      kubeClient,
		secretManager:   NewSecretManager(kubeClient),
		templateManager: NewTemplateManager(),
	}
}

// ApplyManifestsYAML deserializa un YAML multi-documento y aplica los recursos en K8s
func (dm *DeployManager) ApplyManifestsYAML(ctx context.Context, rawYAML string) error {
	docs := strings.Split(rawYAML, "\n---")
	for _, doc := range docs {
		trimmed := strings.TrimSpace(doc)
		if trimmed == "" {
			continue
		}

		var typeMeta metav1.TypeMeta
		if err := k8syaml.Unmarshal([]byte(trimmed), &typeMeta); err != nil {
			continue
		}

		switch typeMeta.Kind {
		case "Namespace":
			var ns corev1.Namespace
			if err := k8syaml.Unmarshal([]byte(trimmed), &ns); err == nil && ns.Name != "" {
				if err := dm.applyNamespace(ctx, &ns); err != nil {
					return fmt.Errorf("error al aplicar Namespace %s: %w", ns.Name, err)
				}
			}
		case "ResourceQuota":
			var rq corev1.ResourceQuota
			if err := k8syaml.Unmarshal([]byte(trimmed), &rq); err == nil && rq.Name != "" {
				if err := dm.applyResourceQuota(ctx, &rq); err != nil {
					return fmt.Errorf("error al aplicar ResourceQuota %s: %w", rq.Name, err)
				}
			}
		case "LimitRange":
			var lr corev1.LimitRange
			if err := k8syaml.Unmarshal([]byte(trimmed), &lr); err == nil && lr.Name != "" {
				if err := dm.applyLimitRange(ctx, &lr); err != nil {
					return fmt.Errorf("error al aplicar LimitRange %s: %w", lr.Name, err)
				}
			}
		case "NetworkPolicy":
			var np networkingv1.NetworkPolicy
			if err := k8syaml.Unmarshal([]byte(trimmed), &np); err == nil && np.Name != "" {
				if err := dm.applyNetworkPolicy(ctx, &np); err != nil {
					return fmt.Errorf("error al aplicar NetworkPolicy %s: %w", np.Name, err)
				}
			}
		case "Deployment":
			var dep appsv1.Deployment
			if err := k8syaml.Unmarshal([]byte(trimmed), &dep); err == nil && dep.Name != "" {
				if err := dm.applyDeployment(ctx, &dep); err != nil {
					return fmt.Errorf("error al aplicar Deployment %s: %w", dep.Name, err)
				}
			}
		case "Service":
			var svc corev1.Service
			if err := k8syaml.Unmarshal([]byte(trimmed), &svc); err == nil && svc.Name != "" {
				if err := dm.applyService(ctx, &svc); err != nil {
					return fmt.Errorf("error al aplicar Service %s: %w", svc.Name, err)
				}
			}
		case "Ingress":
			var ing networkingv1.Ingress
			if err := k8syaml.Unmarshal([]byte(trimmed), &ing); err == nil && ing.Name != "" {
				if err := dm.applyIngress(ctx, &ing); err != nil {
					return fmt.Errorf("error al aplicar Ingress %s: %w", ing.Name, err)
				}
			}
		}
	}
	return nil
}

func (dm *DeployManager) applyNamespace(ctx context.Context, ns *corev1.Namespace) error {
	existing, err := dm.kubeClient.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			_, err := dm.kubeClient.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
			return err
		}
		return err
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

// GetPodLogs retorna los logs recientes del pod principal de un proyecto
func (dm *DeployManager) GetPodLogs(ctx context.Context, namespace string, tailLines int64) (string, error) {
	pods, err := dm.kubeClient.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("error al listar pods en %s: %w", namespace, err)
	}

	if len(pods.Items) == 0 {
		return "No se encontraron pods activos en el namespace " + namespace, nil
	}

	// Seleccionar el pod principal (el primero disponible)
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

	// 1. Obtener Pods del namespace
	pods, err := dm.kubeClient.CoreV1().Pods(p.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("error listando pods en %s: %w", p.Namespace, err)
	}

	metrics.PodsCount = len(pods.Items)

	// 2. Intentar consultar métricas crudas de metrics-server
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

	// 3. Ensamblar información por cada pod
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

	// 4. Consultar ResourceQuota
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

// DeleteProjectDeployment desmantela el namespace y condicionalmente la BD en Postgres central
func (dm *DeployManager) DeleteProjectDeployment(ctx context.Context, projectName string, deleteDB bool) error {
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
	err := dm.kubeClient.CoreV1().Namespaces().Delete(ctx, projectName, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error al eliminar namespace %s: %w", projectName, err)
	}

	return nil
}

// ExecuteDeploy ejecuta la secuencia completa de despliegue
func (dm *DeployManager) ExecuteDeploy(ctx context.Context, p storage.Project, params ProjectManifestParams) error {
	projectName := p.Name
	namespace := p.Namespace

	// 1. Si requiere base de datos, aprovisionar en PostgreSQL Supabase (Modelo A)
	if params.RequiresDatabase {
		p.Status = storage.StatusProvisioning
		_ = storage.AddOrUpdateProject(p)

		host, port, user, password, dbname := GetSupabaseConfig()
		if password == "" {
			log.Printf("[Deploy] ⚠️ SUPABASE_POSTGRES_PASSWORD no establecida, saltando aprovisionamiento físico de BD")
		} else {
			masterDB, err := OpenMasterDB(host, port, user, password, dbname)
			if err != nil {
				log.Printf("[Deploy] Error conectando a Supabase DB: %v", err)
			} else {
				existingPass, _ := dm.secretManager.GetExistingDBPassword(ctx, namespace, projectName)
				if existingPass == "" && params.BuildArgs != nil {
					for _, k := range []string{"DB_PASSWORD", "POSTGRES_PASSWORD", "DATABASE_PASSWORD", "DB_PASS"} {
						if p, ok := params.BuildArgs[k]; ok && strings.TrimSpace(p) != "" {
							existingPass = strings.TrimSpace(p)
							break
						}
					}
				}
				provRes, err := ProvisionProjectDatabase(masterDB, projectName, existingPass)
				_ = masterDB.Close()
				if err != nil {
					p.Status = storage.StatusFailed
					_ = storage.AddOrUpdateProject(p)
					return fmt.Errorf("fallo al aprovisionar BD: %w", err)
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
				_ = dm.applyNamespace(ctx, nsObj)

				if err := dm.secretManager.EnsureDBSecret(ctx, namespace, projectName, provRes); err != nil {
					p.Status = storage.StatusFailed
					_ = storage.AddOrUpdateProject(p)
					return fmt.Errorf("fallo al crear DB secret: %w", err)
				}
			}
		}
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
		_ = dm.applyNamespace(ctx, nsObj)
		_ = dm.secretManager.EnsureCustomEnvSecret(ctx, namespace, projectName, params.BuildArgs)
	}

	// 2. Renderizar y aplicar manifiestos
	p.Status = storage.StatusDeploying
	_ = storage.AddOrUpdateProject(p)

	manifestsYAML, err := dm.templateManager.RenderAllManifests(params)
	if err != nil {
		p.Status = storage.StatusFailed
		_ = storage.AddOrUpdateProject(p)
		return fmt.Errorf("error renderizando manifiestos: %w", err)
	}

	if err := dm.ApplyManifestsYAML(ctx, manifestsYAML); err != nil {
		p.Status = storage.StatusFailed
		_ = storage.AddOrUpdateProject(p)
		return fmt.Errorf("error aplicando manifiestos en K8s: %w", err)
	}

	// Propagar sammcore-registry-secret si está presente en el namespace deployer
	_ = dm.secretManager.EnsureRegistrySecret(ctx, namespace)

	// 3. Monitorear despliegue (hasta 30s)
	go dm.monitorRollout(context.Background(), p)

	return nil
}

func (dm *DeployManager) monitorRollout(ctx context.Context, p storage.Project) {
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		deps, err := dm.kubeClient.AppsV1().Deployments(p.Namespace).List(ctx, metav1.ListOptions{})
		if err == nil && len(deps.Items) > 0 {
			allReady := true
			for _, d := range deps.Items {
				if d.Status.ReadyReplicas < 1 {
					allReady = false
					break
				}
			}
			if allReady {
				p.Status = storage.StatusRunning
				_ = storage.AddOrUpdateProject(p)
				log.Printf("[Deploy] Proyecto %s desplegado exitosamente en Running", p.Name)
				return
			}
		}
	}

	// Si superó deadline sin que todos los pods estén listos
	p.Status = storage.StatusRunning // Marcamos running si no falló explícitamente
	_ = storage.AddOrUpdateProject(p)
}
