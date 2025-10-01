# 📄 Estructura de Templates – SAMMCORE Deployer

Este documento define cómo se organizan y utilizan las plantillas que el `sammcore-deployer` aplica para proyectos sin Dockerfile propio o con `docker-compose.yml`.

---

## 1. 🎯 Objetivo
Centralizar **templates reutilizables** que permitan generar despliegues consistentes en Kubernetes, según el tipo de proyecto:

- Estáticos (HTML/JS).
- Apps con Dockerfile único.
- Stacks multi-servicio (ej. PHP + MySQL).
- Microservicios backend (Go, Node, Python, etc.).

---

## 2. 📂 Estructura de directorios
```

sammcore-templates/
├── html-nginx/
│    ├── Dockerfile
│    ├── deployment.yaml
│    ├── service.yaml
│    └── ingress.yaml
├── php-apache-mysql/
│    ├── apache.Dockerfile
│    ├── mysql-deployment.yaml
│    ├── mysql-service.yaml
│    ├── apache-deployment.yaml
│    ├── apache-service.yaml
│    ├── ingress.yaml
│    └── pvc.yaml
├── go-api/
│    ├── Dockerfile
│    ├── deployment.yaml
│    ├── service.yaml
│    └── ingress.yaml
└── node-express/
├── Dockerfile
├── deployment.yaml
├── service.yaml
└── ingress.yaml

````

---

## 3. 🧩 Contenido de cada template

### 🔹 Dockerfile
- Define cómo construir la imagen.
- Debe ser **lo más genérico posible**, con variables de build.

Ejemplo (html-nginx):
```dockerfile
FROM nginx:alpine
COPY ./public /usr/share/nginx/html
EXPOSE 80
````

---

### 🔹 deployment.yaml

* Deployment de Kubernetes.
* Se parametrizan:

  * Nombre del proyecto.
  * Imagen.
  * Namespace.
  * Puertos.

Ejemplo:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .projectName }}
  namespace: {{ .namespace }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {{ .projectName }}
  template:
    metadata:
      labels:
        app: {{ .projectName }}
    spec:
      containers:
        - name: {{ .projectName }}
          image: {{ .image }}
          ports:
            - containerPort: {{ .port }}
```

---

### 🔹 service.yaml

* Service tipo `ClusterIP` o `NodePort` según caso.
* Conecta al Deployment.

```yaml
apiVersion: v1
kind: Service
metadata:
  name: {{ .projectName }}
  namespace: {{ .namespace }}
spec:
  type: ClusterIP
  selector:
    app: {{ .projectName }}
  ports:
    - port: {{ .port }}
      targetPort: {{ .port }}
```

---

### 🔹 ingress.yaml

* Define exposición en `*.sammcore.local`.
* Usa certificados ya existentes en `sammcore-tls`.

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .projectName }}
  namespace: {{ .namespace }}
spec:
  ingressClassName: nginx
  rules:
    - host: {{ .domain }}
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: {{ .projectName }}
                port:
                  number: {{ .port }}
  tls:
    - hosts:
        - {{ .domain }}
      secretName: sammcore-tls
```

---

### 🔹 pvc.yaml (opcional)

* Solo para apps que requieren persistencia (ej. MySQL, SFTP).
* Define almacenamiento para datos.

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: {{ .projectName }}-pvc
  namespace: {{ .namespace }}
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 5Gi
```

---

## 4. 🔐 Manejo de Secrets

Cada template debe incluir variables sensibles como **referencias a Secrets**, no en claro.

Ejemplo (PHP con MySQL):

```yaml
env:
  - name: MYSQL_USER
    valueFrom:
      secretKeyRef:
        name: phpapp-secrets
        key: MYSQL_USER
  - name: MYSQL_PASSWORD
    valueFrom:
      secretKeyRef:
        name: phpapp-secrets
        key: MYSQL_PASSWORD
```

---

## 5. 🛠️ Motor de plantillas

El backend del deployer:

* Carga el template correspondiente según tipo de proyecto.
* Reemplaza las variables (`projectName`, `namespace`, `image`, `domain`, `port`).
* Aplica los manifests en K3s.

---

## 6. 📌 Buenas prácticas

* Nunca incluir contraseñas fijas en templates.
* Usar `Secrets` siempre para credenciales.
* Mantener templates versionados y probados.
* Añadir tests básicos para validar manifests antes de aplicar.
