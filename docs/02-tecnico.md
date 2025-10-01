# 📄 Documento Técnico – SAMMCORE Deployer

## 1. ⚙️ Arquitectura general
- **Frontend (React/Vite)**  
  - Formulario de registro de proyectos.  
  - Dashboard con estado de despliegues.  
  - Comunicación con backend vía REST.  

- **Backend (Go)**  
  - Endpoints:  
    - `/analyzeRepo`: clona repo y detecta tipo.  
    - `/createSecrets`: crea Secrets en Kubernetes.  
    - `/deploy`: genera manifiestos, construye imágenes, aplica en K3s.  
    - `/status`: consulta estado de pods y servicios.  
  - Usa `client-go` para interactuar con K3s.  
  - Usa `git` para clonar repositorios.  
  - Invoca `docker build` + `docker push` para imágenes.  

- **Repositorio de plantillas (`sammcore-templates`)**  
  - Carpeta con templates listos:  
    - `html-nginx`  
    - `php-apache-mysql`  
    - `go-api`  
    - etc.  

---

## 2. 📂 Estructura de repositorio

sammcore-deployer/
├── backend/ # API en Go
│ ├── handlers/
│ ├── services/
│ ├── templates/ # lógica para usar sammcore-templates
│ ├── kube/ # interacción con client-go
│ └── main.go
├── frontend/ # UI en React/Vite
│ ├── src/
│ └── public/
├── manifests/ # YAML de despliegue del propio deployer
│ ├── deployment.yaml
│ ├── service.yaml
│ └── ingress.yaml
├── .github/workflows/ # CI/CD del deployer
│ └── deploy.yml
└── README.md


---

## 3. 🧩 Módulos Backend
- `RepoManager`: clona y analiza repositorios.  
- `TemplateManager`: elige templates adecuados.  
- `SecretManager`: genera y aplica Kubernetes Secrets.  
- `DeployManager`: construye imágenes y aplica manifests.  
- `StatusManager`: obtiene estado de pods, logs y servicios.  

---

## 4. 🔐 Manejo de credenciales
- Los usuarios ingresan contraseñas en el frontend.  
- El backend las guarda **solo en memoria** y las transforma en **Kubernetes Secrets**.  
- Nunca se guardan en los repos ni en el filesystem del backend.  

---

## 5. 🔄 CI/CD del propio deployer
- Repo `sammcore-deployer` tiene su workflow GitHub Actions.  
- Cada push → build de imagen → push a GHCR → despliegue en SAMMCORE vía `kubectl`.  
- Se expone en `https://deployer.sammcore.local`.  

---

## 6. 🌐 Relación con la arquitectura SAMMCORE
El deployer es un **servicio original**, distinto de los que él mismo gestiona:
- Vive dentro de K3s como `sammcore-deployer`.  
- Se despliega desde **GitHub Actions** (no desde sí mismo).  
- Es el **único autorizado a crear/gestionar otros proyectos** dentro de SAMMCORE.  
