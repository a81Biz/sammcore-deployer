# 📄 Estructura de Templates y Manifiestos – SAMMCORE Deployer

Este documento define la arquitectura de plantillas que `sammcore-deployer` utiliza para generar y parametrizar los recursos de Kubernetes.

---

## 1. 🎯 Reglas de Traducción Compose $\rightarrow$ Kubernetes

Cuando un repositorio contiene `docker-compose.yml`, el módulo `TemplateManager` aplica las siguientes reglas de conversión deterministas:

1. **Extracción del Servicio de Base de Datos:**
   - Si un servicio utiliza una imagen que contiene `postgres`, `mysql`, `mariadb` o expone puertos `5432`/`3306`, **dicho servicio se descarta del despliegue en K8s**.
   - Se activa la bandera `requires_database = true`.
   - Se inyectan las credenciales del clúster central de Supabase (Modelo A) mediante variables de entorno referenciadas al Secret `{{ .projectName }}-db-secrets`.
2. **Heurística Frontend (Web) vs Backend (API):**
   - **Servicio Web / Frontend:** Aquel que expone puertos estándar de interfaz (`80`, `3000`, `5173`, `8080`) o utiliza servidores web como NGINX o scripts de Vite. Se le asigna Ingress en `https://{{ .projectName }}.sammcore.local`.
   - **Servicio API / Backend:** Aquel que expone puertos de servicio (`8000`, `5000`, `8080`) o corre runtimes de backend (Go, Python/FastAPI, Node/Express). Se le asigna Ingress en `https://api.{{ .projectName }}.sammcore.local`.
3. **Manejo de `VITE_API_BASE` en Frontend:**
   - Como Vite hornea las variables `VITE_*` durante el comando `build`, la URL pública de la API (`https://api.{{ .projectName }}.sammcore.local`) se suministra como `--build-arg` durante la compilación de la imagen, o bien se provee un subpath proxy `/api/` en el Ingress del frontend.
4. **Traducción de Volúmenes y Dependencias:**
   - Los volúmenes montados por servicios de BD se descartan.
   - Los volúmenes de almacenamiento de archivos de la app se traducen a `PersistentVolumeClaim` de 5Gi con `storageClassName: local-path`.
   - La directiva `depends_on: [db]` se traduce en un `initContainer` ligero que verifica la disponibilidad del puerto 5432 en `postgres.supabase.svc.cluster.local`.

---

## 2. 🧩 Plantillas de Manifiestos por Arquetipo

### 🔹 2.1. Namespace y Gobernanza de Recursos
Aplicado a todo proyecto para garantizar aislamiento multi-tenant:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: {{ .namespace }}
  labels:
    app.kubernetes.io/managed-by: sammcore-deployer
    project: {{ .projectName }}
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: {{ .projectName }}-quota
  namespace: {{ .namespace }}
spec:
  hard:
    requests.cpu: "500m"
    requests.memory: "512Mi"
    limits.cpu: "2000m"
    limits.memory: "2Gi"
    pods: "10"
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: {{ .projectName }}-netpol
  namespace: {{ .namespace }}
spec:
  podSelector: {}
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Permitir tráfico desde el Ingress Controller de K3s
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ingress-nginx
  egress:
    # Permitir salida a DNS interno y PostgreSQL de Supabase
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: supabase
      ports:
        - protocol: TCP
          port: 5432
    # Permitir salida a Internet (para llamadas a APIs externas)
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              - 10.42.0.0/16
              - 10.43.0.0/16
```

---

### 🔹 2.2. Secret de Base de Datos Supabase (Modelo A)
Generado en memoria por `SecretManager`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: {{ .projectName }}-db-secrets
  namespace: {{ .namespace }}
type: Opaque
stringData:
  DB_HOST: "postgres.supabase.svc.cluster.local"
  DB_PORT: "5432"
  DB_NAME: "{{ .dbName }}"
  DB_USER: "{{ .dbUser }}"
  DB_PASSWORD: "{{ .dbPassword }}"
  DATABASE_URL: "postgresql://{{ .dbUser }}:{{ .dbPassword }}@postgres.supabase.svc.cluster.local:5432/{{ .dbName }}?sslmode=disable"
```

---

### 🔹 2.3. Deployment de API Backend con Healthchecks y Resources
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .projectName }}-api
  namespace: {{ .namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .projectName }}-api
  template:
    metadata:
      labels:
        app: {{ .projectName }}-api
    spec:
      containers:
        - name: api
          image: {{ .apiImage }}
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: {{ .apiPort }}
          envFrom:
            - secretRef:
                name: {{ .projectName }}-db-secrets
          readinessProbe:
            tcpSocket:
              port: {{ .apiPort }}
            initialDelaySeconds: 5
            periodSeconds: 10
          livenessProbe:
            tcpSocket:
              port: {{ .apiPort }}
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
  name: {{ .projectName }}-api
  namespace: {{ .namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .projectName }}-api
  ports:
    - name: http
      port: {{ .apiPort }}
      targetPort: {{ .apiPort }}
```

---

### 🔹 2.4. Ingress Multi-Subdominio Dinámico (Sin TLS interno)
> **Aclaración de Arquitectura:** El NGINX del host físico termina el TLS para `*.sammcore.local` con el certificado comodín de la LAN y reenvía HTTP al NodePort `30080` de `ingress-nginx`. Por lo tanto, el Ingress en Kubernetes **opera en HTTP plano y no requiere Secret de certificados en el namespace de cada proyecto**.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .projectName }}-ingress
  namespace: {{ .namespace }}
  annotations:
    nginx.ingress.kubernetes.io/proxy-body-size: "50m"
spec:
  ingressClassName: nginx
  rules:
    # 1. Enrutamiento del Frontend Web
    - host: {{ .projectName }}.sammcore.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .projectName }}-web
                port:
                  number: {{ .webPort }}

    # 2. Enrutamiento del API Backend
    - host: api.{{ .projectName }}.sammcore.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .projectName }}-api
                port:
                  number: {{ .apiPort }}
```
