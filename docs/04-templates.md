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
   - Como Vite hornea las variables `VITE_*` durante el comando `build`, la URL pública de la API (`https://api.{{ .projectName }}.sammcore.local`) se suministra como `--build-arg` durante la compilación de la imagen, o bien se provee un subpath proxy `/api/` en el NGINX del frontend.
4. **Traducción de Volúmenes y Dependencias:**
   - Los volúmenes montados por servicios de BD se descartan.
   - Los volúmenes de almacenamiento de archivos de la app se traducen a `PersistentVolumeClaim` de 5Gi con `storageClassName: local-path`.
   - La directiva `depends_on: [db]` se traduce en un `initContainer` ligero que verifica la disponibilidad del puerto 5432 en `postgres.supabase.svc.cluster.local`.

---

## 2. 🧩 Plantillas de Manifiestos Base por Proyecto

### 🔹 2.1. Namespace, Cuotas y NetworkPolicy
Aplicado a todo proyecto para garantizar aislamiento multi-tenant sin romper la comunicación interna:

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
    # 1. Tráfico intra-namespace (Web -> API, workers -> API)
    - from:
        - podSelector: {}
    # 2. Tráfico desde el Ingress Controller de K3s
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ingress-nginx
  egress:
    # 1. Tráfico intra-namespace (comunicación interna entre pods del proyecto)
    - to:
        - podSelector: {}
    # 2. DNS interno del clúster (UDP y TCP 53)
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - protocol: UDP
          port: 53
        - protocol: TCP
          port: 53
    # 3. Base de Datos central de Supabase (PostgreSQL 5432)
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: supabase
      ports:
        - protocol: TCP
          port: 5432
    # 4. Salida a Internet público (excluyendo subredes privadas RFC 1918 y pods/servicios K8s)
    - to:
        - ipBlock:
            cidr: 0.0.0.0/0
            except:
              - 10.0.0.0/8       # Redes privadas Clase A y pods/services K3s
              - 172.16.0.0/12    # Redes privadas Clase B
              - 192.168.0.0/16   # LAN local física de SAMMCORE
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

## 3. 📦 Plantillas de Despliegue por Arquetipo

### 🔹 Arquetipo 1: Stack Multi-Servicio (`compose-multi-service`)

#### Web Deployment & Service:
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .projectName }}-web
  namespace: {{ .namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .projectName }}-web
  template:
    metadata:
      labels:
        app: {{ .projectName }}-web
    spec:
      containers:
        - name: web
          image: {{ .webImage }}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: {{ .webPort }}
          readinessProbe:
            tcpSocket:
              port: {{ .webPort }}
            initialDelaySeconds: 3
            periodSeconds: 5
          livenessProbe:
            tcpSocket:
              port: {{ .webPort }}
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
  name: {{ .projectName }}-web
  namespace: {{ .namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .projectName }}-web
  ports:
    - name: http
      port: {{ .webPort }}
      targetPort: {{ .webPort }}
```

#### API Deployment con InitContainer de BD & Service:
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
      initContainers:
        - name: wait-for-db
          image: busybox:1.36
          command: ['sh', '-c', 'until nc -z -w 2 postgres.supabase.svc.cluster.local 5432; do echo esperando postgres; sleep 2; done']
      containers:
        - name: api
          image: {{ .apiImage }}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: {{ .apiPort }}
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

#### Ingress Multi-Subdominio (Compose):
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

---

### 🔹 Arquetipo 2: Microservicio Individual (`single-dockerfile`)
Un único pod para backend o servicio con un único host:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .projectName }}-app
  namespace: {{ .namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .projectName }}-app
  template:
    metadata:
      labels:
        app: {{ .projectName }}-app
    spec:
      containers:
        - name: app
          image: {{ .appImage }}
          imagePullPolicy: IfNotPresent
          ports:
            - name: http
              containerPort: {{ .appPort }}
          readinessProbe:
            tcpSocket:
              port: {{ .appPort }}
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
  name: {{ .projectName }}-app
  namespace: {{ .namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .projectName }}-app
  ports:
    - name: http
      port: {{ .appPort }}
      targetPort: {{ .appPort }}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .projectName }}-ingress
  namespace: {{ .namespace }}
spec:
  ingressClassName: nginx
  rules:
    - host: {{ .projectName }}.sammcore.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .projectName }}-app
                port:
                  number: {{ .appPort }}
```

---

### 🔹 Arquetipo 3: Sitio Web Estático (`static-web`)
Servido mediante imagen ligera de NGINX con un único host:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .projectName }}-static
  namespace: {{ .namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .projectName }}-static
  template:
    metadata:
      labels:
        app: {{ .projectName }}-static
    spec:
      containers:
        - name: nginx
          image: {{ .staticImage }}
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
  name: {{ .projectName }}-static
  namespace: {{ .namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .projectName }}-static
  ports:
    - name: http
      port: 80
      targetPort: 80
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .projectName }}-ingress
  namespace: {{ .namespace }}
spec:
  ingressClassName: nginx
  rules:
    - host: {{ .projectName }}.sammcore.local
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .projectName }}-static
                port:
                  number: 80
```

---

## 4. 🔨 Plantilla de Build In-Cluster con Kaniko

Si el proyecto no se construye en el CI del repo cliente, `DeployManager` lanza este Job en un namespace temporal de build o en el namespace del proyecto:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: kaniko-build-{{ .projectName }}
  namespace: {{ .namespace }}
spec:
  ttlSecondsAfterFinished: 120
  backoffLimit: 1
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: kaniko
          image: gcr.io/kaniko-project/executor:v1.23.0
          args:
            - "--context=git://github.com/{{ .repoSlug }}.git#refs/heads/{{ .branch }}"
            - "--dockerfile={{ .dockerfilePath }}"
            - "--destination={{ .targetImage }}"
            - "--cache=true"
          volumeMounts:
            - name: docker-config
              mountPath: /kaniko/.docker
          resources:
            requests:
              cpu: 500m
              memory: 512Mi
            limits:
              cpu: 2000m
              memory: 2Gi
      volumes:
        - name: docker-config
          secret:
            secretName: kaniko-registry-secret
            items:
              - key: .dockerconfigjson
                path: config.json
```
