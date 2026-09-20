# 📄 Estructura de Templates y Manifiestos – SAMMCORE Deployer

Este documento define la arquitectura de plantillas que `sammcore-deployer` utiliza para generar y parametrizar los manifiestos de Kubernetes (`Deployment`, `Service`, `Ingress`, `Secret`).

---

## 1. 🎯 Filosofía de Despliegue en SAMMCORE

* **Base de Datos Horizontal (Modelo A):**  
  Las aplicaciones **no despliegan contenedores de base de datos individuales**. La persistencia relacional es provista exclusivamente por el clúster central de **Supabase PostgreSQL** (`postgres.supabase.svc.cluster.local:5432`), garantizando aislamiento por proyecto (`<proyecto>_db`) y visibilidad directa en Supabase Studio y Grafana.
* **Cero Secretos en Repositorio:**  
  Las credenciales nunca residen en texto plano. Se inyectan en tiempo de ejecución a través de objetos `Secret` de Kubernetes creados en memoria por el backend del deployer.
* **Enrutamiento Dinámico Multi-Subdominio:**  
  Cada proyecto puede exponer múltiples puntos de entrada bajo la zona `*.sammcore.local`:
  - Frontend / Web: `https://<proyecto>.sammcore.local`
  - API / Backend: `https://api.<proyecto>.sammcore.local`
  - Documentación / Swagger: `https://docs.<proyecto>.sammcore.local`

---

## 2. 🧩 Tipos de Plantillas Soportadas

```text
sammcore-templates/
├── compose-multi-service/    # Stack completo (ej. Backroom: Web + API + Supabase DB)
│    ├── web-deployment.yaml
│    ├── web-service.yaml
│    ├── api-deployment.yaml
│    ├── api-service.yaml
│    └── ingress.yaml
├── single-dockerfile/        # Microservicio individual (Go, Node, Python)
│    ├── deployment.yaml
│    ├── service.yaml
│    └── ingress.yaml
└── static-web/               # Sitios estáticos (React, Vue, HTML/JS puro)
     ├── deployment.yaml
     ├── service.yaml
     └── ingress.yaml
```

---

## 3. 📝 Especificación de Manifiestos Parametrizados

### 🔹 3.1. Secret de Base de Datos (`secret.yaml`)
Generado dinámicamente por `SecretManager` tras el aprovisionamiento en Supabase:

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

### 🔹 3.2. Deployment de Backend API con Inyección de DB (`api-deployment.yaml`)
El pod consume las credenciales de la base de datos de Supabase sin exponer contraseñas:

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
          ports:
            - containerPort: {{ .apiPort }}
          env:
            - name: DB_HOST
              valueFrom:
                secretKeyRef:
                  name: {{ .projectName }}-db-secrets
                  key: DB_HOST
            - name: DB_PORT
              valueFrom:
                secretKeyRef:
                  name: {{ .projectName }}-db-secrets
                  key: DB_PORT
            - name: DB_NAME
              valueFrom:
                secretKeyRef:
                  name: {{ .projectName }}-db-secrets
                  key: DB_NAME
            - name: DB_USER
              valueFrom:
                secretKeyRef:
                  name: {{ .projectName }}-db-secrets
                  key: DB_USER
            - name: DB_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: {{ .projectName }}-db-secrets
                  key: DB_PASSWORD
            - name: DATABASE_URL
              valueFrom:
                secretKeyRef:
                  name: {{ .projectName }}-db-secrets
                  key: DATABASE_URL
```

---

### 🔹 3.3. Ingress Multi-Subdominio Dinámico (`ingress.yaml`)
Enruta el tráfico de forma diferenciada para la interfaz web y la API REST:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: {{ .projectName }}-ingress
  namespace: {{ .namespace }}
  annotations:
    ingress.class: nginx
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

  tls:
    - hosts:
        - {{ .projectName }}.sammcore.local
        - api.{{ .projectName }}.sammcore.local
      secretName: sammcore-tls
```

---

## 4. 📋 Matriz de Variables de Sustitución

| Variable | Tipo | Descripción | Ejemplo |
| :--- | :--- | :--- | :--- |
| `{{ .projectName }}` | String | Identificador alfanumérico del proyecto | `backroom` |
| `{{ .namespace }}` | String | Espacio de nombres aislado en K3s | `backroom` |
| `{{ .webImage }}` | String | Imagen del frontend (o NGINX empaquetado) | `backroom-web:latest` |
| `{{ .webPort }}` | Int | Puerto del contenedor frontend | `80` |
| `{{ .apiImage }}` | String | Imagen del backend compilado | `backroom-api:latest` |
| `{{ .apiPort }}` | Int | Puerto de escucha del backend | `8000` |
| `{{ .dbName }}` | String | Base de datos aprovisionada en Supabase | `backroom_db` |
| `{{ .dbUser }}` | String | Rol de PostgreSQL aprovisionado en Supabase | `backroom_user` |
| `{{ .dbPassword }}` | String | Credencial criptográfica en memoria | `32-char-random-string` |
