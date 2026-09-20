package services

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

type ProjectManifestParams struct {
	ProjectName      string `json:"project_name"`
	Namespace        string `json:"namespace"`
	Type             string `json:"type"` // compose, dockerfile, static
	Domain           string `json:"domain"`
	APIDomain        string `json:"api_domain,omitempty"`
	RequiresDatabase bool   `json:"requires_database"`
	WebImage         string `json:"web_image,omitempty"`
	WebPort          int    `json:"web_port,omitempty"`
	APIImage         string `json:"api_image,omitempty"`
	APIPort          int    `json:"api_port,omitempty"`
	AppImage         string            `json:"app_image,omitempty"`
	AppPort          int               `json:"app_port,omitempty"`
	StaticImage      string            `json:"static_image,omitempty"`
	HasCustomEnv     bool              `json:"has_custom_env"`
	BuildArgs        map[string]string `json:"build_args,omitempty"`
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

const composeAppTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .ProjectName }}-web
  namespace: {{ .Namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .ProjectName }}-web
  template:
    metadata:
      labels:
        app: {{ .ProjectName }}-web
    spec:
      automountServiceAccountToken: false
      imagePullSecrets:
        - name: sammcore-registry-secret
      containers:
        - name: web
          image: {{ .WebImage }}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: {{ .WebPort }}
          readinessProbe:
            tcpSocket:
              port: {{ .WebPort }}
            initialDelaySeconds: 3
            periodSeconds: 5
          livenessProbe:
            tcpSocket:
              port: {{ .WebPort }}
            initialDelaySeconds: 10
            periodSeconds: 10
          resources:
            requests:
              cpu: 25m
              memory: 32Mi
            limits:
              cpu: 200m
              memory: 128Mi
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .ProjectName }}-web
  namespace: {{ .Namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .ProjectName }}-web
  ports:
    - name: http
      port: {{ .WebPort }}
      targetPort: {{ .WebPort }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .ProjectName }}-api
  namespace: {{ .Namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .ProjectName }}-api
  template:
    metadata:
      labels:
        app: {{ .ProjectName }}-api
    spec:
      automountServiceAccountToken: false
      imagePullSecrets:
        - name: sammcore-registry-secret
      {{- if .RequiresDatabase }}
      initContainers:
        - name: wait-for-db
          image: busybox:1.36
          command: ['sh', '-c', 'until nc -z -w 2 postgres.supabase.svc.cluster.local 5432; do echo esperando postgres; sleep 2; done']
          resources:
            requests:
              cpu: 10m
              memory: 16Mi
            limits:
              cpu: 50m
              memory: 32Mi
      {{- end }}
      containers:
        - name: api
          image: {{ .APIImage }}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: {{ .APIPort }}
          {{- if .RequiresDatabase }}
          envFrom:
            - secretRef:
                name: {{ .ProjectName }}-db-secrets
          {{- end }}
          {{- if .HasCustomEnv }}
          envFrom:
            - secretRef:
                name: {{ .ProjectName }}-env-secrets
          {{- end }}
          readinessProbe:
            tcpSocket:
              port: {{ .APIPort }}
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            tcpSocket:
              port: {{ .APIPort }}
            initialDelaySeconds: 15
            periodSeconds: 20
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 512Mi
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .ProjectName }}-api
  namespace: {{ .Namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .ProjectName }}-api
  ports:
    - name: http
      port: {{ .APIPort }}
      targetPort: {{ .APIPort }}
---
apiVersion: v1
kind: Service
metadata:
  name: backend
  namespace: {{ .Namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .ProjectName }}-api
  ports:
    - name: http
      port: {{ .APIPort }}
      targetPort: {{ .APIPort }}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .ProjectName }}-ingress
  namespace: {{ .Namespace }}
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
spec:
  ingressClassName: nginx
  rules:
    - host: {{ .Domain }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .ProjectName }}-web
                port:
                  number: {{ .WebPort }}
    {{- if .APIDomain }}
    - host: {{ .APIDomain }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .ProjectName }}-api
                port:
                  number: {{ .APIPort }}
    {{- end }}
`

const dockerfileAppTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .ProjectName }}-app
  namespace: {{ .Namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .ProjectName }}-app
  template:
    metadata:
      labels:
        app: {{ .ProjectName }}-app
    spec:
      automountServiceAccountToken: false
      imagePullSecrets:
        - name: sammcore-registry-secret
      {{- if .RequiresDatabase }}
      initContainers:
        - name: wait-for-db
          image: busybox:1.36
          command: ['sh', '-c', 'until nc -z -w 2 postgres.supabase.svc.cluster.local 5432; do sleep 2; done']
          resources:
            requests:
              cpu: 10m
              memory: 16Mi
            limits:
              cpu: 50m
              memory: 32Mi
      {{- end }}
      containers:
        - name: app
          image: {{ .AppImage }}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: {{ .AppPort }}
          {{- if .RequiresDatabase }}
          envFrom:
            - secretRef:
                name: {{ .ProjectName }}-db-secrets
          {{- end }}
          {{- if .HasCustomEnv }}
          envFrom:
            - secretRef:
                name: {{ .ProjectName }}-env-secrets
          {{- end }}
          readinessProbe:
            tcpSocket:
              port: {{ .AppPort }}
            initialDelaySeconds: 5
            periodSeconds: 10
          resources:
            requests:
              cpu: 50m
              memory: 64Mi
            limits:
              cpu: 500m
              memory: 256Mi
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .ProjectName }}-app
  namespace: {{ .Namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .ProjectName }}-app
  ports:
    - name: http
      port: {{ .AppPort }}
      targetPort: {{ .AppPort }}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .ProjectName }}-ingress
  namespace: {{ .Namespace }}
spec:
  ingressClassName: nginx
  rules:
    - host: {{ .Domain }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .ProjectName }}-app
                port:
                  number: {{ .AppPort }}
`

const staticAppTemplate = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .ProjectName }}-static
  namespace: {{ .Namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .ProjectName }}-static
  template:
    metadata:
      labels:
        app: {{ .ProjectName }}-static
    spec:
      automountServiceAccountToken: false
      imagePullSecrets:
        - name: sammcore-registry-secret
      containers:
        - name: nginx
          image: {{ .StaticImage }}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: 80
          resources:
            requests:
              cpu: 10m
              memory: 16Mi
            limits:
              cpu: 100m
              memory: 64Mi
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .ProjectName }}-static
  namespace: {{ .Namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .ProjectName }}-static
  ports:
    - name: http
      port: 80
      targetPort: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .ProjectName }}-ingress
  namespace: {{ .Namespace }}
spec:
  ingressClassName: nginx
  rules:
    - host: {{ .Domain }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .ProjectName }}-static
                port:
                  number: 80
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

func (tm *TemplateManager) RenderAppManifests(params ProjectManifestParams) (string, error) {
	var rawTemplate string
	switch strings.ToLower(params.Type) {
	case "compose":
		rawTemplate = composeAppTemplate
	case "dockerfile":
		rawTemplate = dockerfileAppTemplate
	case "static":
		rawTemplate = staticAppTemplate
	default:
		return "", fmt.Errorf("tipo de proyecto no soportado para generación de manifiestos: %s", params.Type)
	}

	tmpl, err := template.New("app").Parse(rawTemplate)
	if err != nil {
		return "", fmt.Errorf("error al parsear plantilla de aplicación (%s): %w", params.Type, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, params); err != nil {
		return "", fmt.Errorf("error al renderizar manifiesto de aplicación (%s): %w", params.Type, err)
	}

	return buf.String(), nil
}

func (tm *TemplateManager) RenderAllManifests(params ProjectManifestParams) (string, error) {
	base, err := tm.RenderBaseManifests(params)
	if err != nil {
		return "", err
	}

	app, err := tm.RenderAppManifests(params)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s---\n%s", base, app), nil
}
