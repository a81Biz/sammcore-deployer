package services

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"sammcore-deployer/config"
)

// ProjectManifestParams contiene los parámetros para renderizar manifiestos de Kubernetes.
// En v2 usa Services[] en lugar de campos de imagen/puerto fijos.
type ProjectManifestParams struct {
	ProjectName      string            `json:"project_name"`
	Namespace        string            `json:"namespace"`
	Type             string            `json:"type"` // compose, dockerfile, static
	Domain           string            `json:"domain"`
	APIDomain        string            `json:"api_domain,omitempty"`
	RequiresDatabase bool              `json:"requires_database"`
	HasCustomEnv     bool              `json:"has_custom_env"`
	BuildArgs        map[string]string `json:"build_args,omitempty"`
	Services         []ServiceSpec     `json:"services,omitempty"`
	Images           map[string]string `json:"images,omitempty"` // servicio → imagen completa
}

const baseManifestsTemplate = `apiVersion: v1
kind: Namespace
metadata:
  name: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: sammcore-deployer
    project: {{ .ProjectName }}
    pod-security.kubernetes.io/enforce: baseline
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: {{ .ProjectName }}-quota
  namespace: {{ .Namespace }}
spec:
  hard:
    requests.cpu: "500m"
    requests.memory: "512Mi"
    limits.cpu: "2000m"
    limits.memory: "2Gi"
    pods: "10"
---
apiVersion: v1
kind: LimitRange
metadata:
  name: {{ .ProjectName }}-limits
  namespace: {{ .Namespace }}
spec:
  limits:
    - default:
        cpu: "500m"
        memory: "512Mi"
      defaultRequest:
        cpu: "50m"
        memory: "64Mi"
      type: Container
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .ProjectName }}-netpol
  namespace: {{ .Namespace }}
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Tráfico intra-namespace (Web -> API, workers -> API)
    - from:
        - podSelector: {}
    # Tráfico desde el Ingress Controller de K3s
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ingress-nginx
  egress:
    # Tráfico intra-namespace
    - to:
        - podSelector: {}
    # DNS interno del clúster (UDP y TCP 53)
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    # Base de Datos central de Supabase (PostgreSQL 5432)
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: supabase
      ports:
        - protocol: TCP
          port: 5432
    # Salida a Internet público excluyendo subredes privadas RFC 1918
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              - 10.0.0.0/8
              - 172.16.0.0/12
              - 192.168.0.0/16
`

// serviceDeploymentData es la estructura que alimenta la plantilla por servicio
type serviceDeploymentData struct {
	ProjectName      string
	Namespace        string
	ServiceName      string // nombre del servicio (ej: "frontend", "backend")
	FullName         string // ProjectName-ServiceName (ej: "backroom-frontend")
	Role             string // "web", "api", "worker", "app"
	Image            string
	Port             int
	RequiresDatabase bool
	HasCustomEnv     bool
	IsAPIOrWorker    bool // true si necesita envFrom con secrets
	DBHost           string
	BusyboxImage     string
}

// deploymentTemplate genera un Deployment + Service (si tiene puerto) por cada servicio
const serviceDeploymentTemplate = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .FullName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: sammcore-deployer
    project: {{ .ProjectName }}
    service: {{ .ServiceName }}
    role: {{ .Role }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .FullName }}
  template:
    metadata:
      labels:
        app: {{ .FullName }}
        project: {{ .ProjectName }}
        role: {{ .Role }}
    spec:
      automountServiceAccountToken: false
      {{- if .RequiresDatabase }}
      initContainers:
        - name: wait-for-db
          image: {{ .BusyboxImage }}
          command: ['sh', '-c', 'until nc -z -w 2 {{ .DBHost }} 5432; do echo esperando postgres; sleep 2; done']
          resources:
            requests:
              cpu: 10m
              memory: 16Mi
            limits:
              cpu: 50m
              memory: 32Mi
      {{- end }}
      containers:
        - name: {{ .ServiceName }}
          image: {{ .Image }}
          imagePullPolicy: IfNotPresent
          {{- if gt .Port 0 }}
          ports:
            - name: http
              containerPort: {{ .Port }}
          {{- end }}
          {{- if .IsAPIOrWorker }}
          {{- if or .RequiresDatabase .HasCustomEnv }}
          envFrom:
            {{- if .RequiresDatabase }}
            - secretRef:
                name: {{ .ProjectName }}-db-secrets
            {{- end }}
            {{- if .HasCustomEnv }}
            - secretRef:
                name: {{ .ProjectName }}-env-secrets
            {{- end }}
          {{- end }}
          {{- end }}
          {{- if gt .Port 0 }}
          readinessProbe:
            tcpSocket:
              port: {{ .Port }}
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            tcpSocket:
              port: {{ .Port }}
            initialDelaySeconds: 15
            periodSeconds: 30
          {{- end }}
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
`

const serviceTemplate = `---
apiVersion: v1
kind: Service
metadata:
  name: {{ .FullName }}
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: sammcore-deployer
    project: {{ .ProjectName }}
    service: {{ .ServiceName }}
spec:
  type: ClusterIP
  selector:
    app: {{ .FullName }}
  ports:
    - name: http
      port: {{ .Port }}
      targetPort: {{ .Port }}
`

// serviceAliasTemplate crea un Service alias "backend" que apunta al servicio API,
// permitiendo que el nginx del frontend haga proxy_pass a http://backend:<port>
const serviceAliasTemplate = `---
apiVersion: v1
kind: Service
metadata:
  name: backend
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: sammcore-deployer
    project: {{ .ProjectName }}
spec:
  type: ClusterIP
  selector:
    app: {{ .FullName }}
  ports:
    - name: http
      port: {{ .Port }}
      targetPort: {{ .Port }}
`

// ingressRuleData es la estructura para una regla individual de Ingress
type ingressRuleData struct {
	Host        string
	ServiceName string
	Port        int
}

// ingressData agrupa todas las reglas de Ingress para el proyecto
type ingressData struct {
	ProjectName  string
	Namespace    string
	IngressClass string
	Rules        []ingressRuleData
}

const ingressTemplate = `---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .ProjectName }}-ingress
  namespace: {{ .Namespace }}
  labels:
    app.kubernetes.io/managed-by: sammcore-deployer
    project: {{ .ProjectName }}
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
spec:
  ingressClassName: {{ .IngressClass }}
  rules:
    {{- range .Rules }}
    - host: {{ .Host }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .ServiceName }}
                port:
    {{- end }}
`

type TemplateManager struct{}

func NewTemplateManager() *TemplateManager {
	return &TemplateManager{}
}

func (tm *TemplateManager) RenderBaseManifests(params ProjectManifestParams) (string, error) {
	tmpl, err := template.New("base").Parse(baseManifestsTemplate)
	if err != nil {
		return "", fmt.Errorf("error al parsear plantilla base: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("error al ejecutar plantilla base: %w", err)
	}

	return buf.String(), nil
}

// RenderServiceManifests genera los Deployments, Services e Ingress para todos los servicios del proyecto
func (tm *TemplateManager) RenderServiceManifests(params ProjectManifestParams) (string, error) {
	depTmpl, err := template.New("deployment").Parse(serviceDeploymentTemplate)
	if err != nil {
		return "", fmt.Errorf("error al parsear plantilla de deployment: %w", err)
	}

	svcTmpl, err := template.New("service").Parse(serviceTemplate)
	if err != nil {
		return "", fmt.Errorf("error al parsear plantilla de service: %w", err)
	}

	aliasTmpl, err := template.New("alias").Parse(serviceAliasTemplate)
	if err != nil {
		return "", fmt.Errorf("error al parsear plantilla de alias: %w", err)
	}

	ingTmpl, err := template.New("ingress").Parse(ingressTemplate)
	if err != nil {
		return "", fmt.Errorf("error al parsear plantilla de ingress: %w", err)
	}

	var buf bytes.Buffer
	var ingressRules []ingressRuleData
	var apiService *serviceDeploymentData
	cfg := config.Load()

	for _, svc := range params.Services {
		image := ""
		if params.Images != nil {
			image = params.Images[svc.Name]
		}
		if image == "" {
			continue // Sin imagen, no se puede desplegar
		}

		fullName := fmt.Sprintf("%s-%s", params.ProjectName, svc.Name)
		isAPIOrWorker := svc.Role == RoleAPI || svc.Role == RoleWorker || svc.Role == RoleApp

		data := serviceDeploymentData{
			ProjectName:      params.ProjectName,
			Namespace:        params.Namespace,
			ServiceName:      svc.Name,
			FullName:         fullName,
			Role:             string(svc.Role),
			Image:            image,
			Port:             svc.Port,
			RequiresDatabase: params.RequiresDatabase && isAPIOrWorker,
			HasCustomEnv:     params.HasCustomEnv && isAPIOrWorker,
			IsAPIOrWorker:    isAPIOrWorker,
			DBHost:           cfg.DBAppHost,
			BusyboxImage:     cfg.BusyboxImage,
		}

		// Renderizar Deployment
		if err := depTmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("error al renderizar deployment para %s: %w", svc.Name, err)
		}

		// Renderizar Service (solo si tiene puerto y no es worker)
		if svc.Port > 0 && svc.Role != RoleWorker {
			if err := svcTmpl.Execute(&buf, data); err != nil {
				return "", fmt.Errorf("error al renderizar service para %s: %w", svc.Name, err)
			}
		}

		// Registrar servicio API para el alias "backend"
		if svc.Role == RoleAPI || svc.Role == RoleApp {
			apiService = &data
		}

		// Construir reglas de Ingress
		switch svc.Role {
		case RoleWeb:
			ingressRules = append(ingressRules, ingressRuleData{
				Host:        params.Domain,
				ServiceName: fullName,
				Port:        svc.Port,
			})
		case RoleAPI, RoleApp:
			host := params.Domain
			if params.APIDomain != "" && svc.Role == RoleAPI {
				host = params.APIDomain
			}
			ingressRules = append(ingressRules, ingressRuleData{
				Host:        host,
				ServiceName: fullName,
				Port:        svc.Port,
			})
		}
	}

	// Service alias "backend" para que nginx del frontend pueda hacer proxy_pass
	if apiService != nil {
		if err := aliasTmpl.Execute(&buf, apiService); err != nil {
			return "", fmt.Errorf("error al renderizar alias backend: %w", err)
		}
	}

	// Renderizar Ingress con todas las reglas
	if len(ingressRules) > 0 {
		ingData := ingressData{
			ProjectName:  params.ProjectName,
			Namespace:    params.Namespace,
			IngressClass: cfg.IngressClass,
			Rules:        ingressRules,
		}
		if err := ingTmpl.Execute(&buf, ingData); err != nil {
			return "", fmt.Errorf("error al renderizar ingress: %w", err)
		}
	}

	return buf.String(), nil
}

// RenderAllManifests genera los manifiestos base (namespace, quota, netpol) y los de aplicación (deployments, services, ingress)
func (tm *TemplateManager) RenderAllManifests(params ProjectManifestParams) (string, error) {
	base, err := tm.RenderBaseManifests(params)
	if err != nil {
		return "", err
	}

	app, err := tm.RenderServiceManifests(params)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s\n%s", strings.TrimSpace(base), strings.TrimSpace(app)), nil
}

// RenderAppManifests mantiene compatibilidad con código existente pero usa la nueva lógica per-service
func (tm *TemplateManager) RenderAppManifests(params ProjectManifestParams) (string, error) {
	return tm.RenderServiceManifests(params)
}
