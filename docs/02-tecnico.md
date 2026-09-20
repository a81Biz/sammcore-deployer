# 📄 Documento Técnico – SAMMCORE Deployer

## 1. ⚙️ Arquitectura general
- **Frontend (React/Vite)**  
  - Formulario de registro de proyectos.  
  - Dashboard con estado de despliegues.  
  - Comunicación con backend vía REST.  

- **Backend (Go)**  
  - Endpoints:  
    - `POST /analyzeRepo`: clona repo y detecta tipo (compose, dockerfile, unknown).  
    - `POST /deploy`: aprovisiona BD en Supabase (Modelo A), crea Secrets en K8s, genera manifiestos con subdominios dinámicos y aplica en K3s.  
    - `GET /history`: lista proyectos registrados y su estado.  
    - `GET /status/:id`: consulta estado real de pods, servicios e Ingress en K3s.  
    - `DELETE /history/:id`: elimina proyecto (ofrece opción de conservar o eliminar BD en Supabase).  
  - Usa `client-go` para interactuar con K3s (Namespaces, Deployments, Services, Secrets, Ingress).  
  - Usa `git` para clonar repositorios y analizar estructura.  
  - Se conecta a Supabase PostgreSQL (`postgres.supabase.svc.cluster.local:5432`) para aprovisionar bases dedicadas por proyecto.  

- **Repositorio de plantillas (`sammcore-templates`)**  
  - Carpeta con templates listos:  
    - `html-nginx`  
    - `compose-multi-service` (ej. Frontend React + Backend Go + Supabase DB)  
    - `go-api`  
    - `node-express`  
    - etc.  

---

## 2. 📂 Estructura de repositorio

```text
sammcore-deployer/
├── backend/                  # API en Go
│   ├── api/                  # Routers y middlewares (CORS, Prometheus)
│   ├── core/                 # Orquestación de análisis y despliegue
│   ├── services/             # Lógica de negocio especializada
│   │   ├── repo_manager.go   # Clonado y detección
│   │   ├── db_manager.go     # Aprovisionamiento Supabase (Modelo A)
│   │   ├── secret_manager.go # Creación de Secrets en K3s (en memoria)
│   │   ├── template_manager.go # Motor de manifiestos e Ingress dinámico
│   │   └── deploy_manager.go # Ejecución en K3s vía client-go
│   ├── storage/              # Persistencia del catálogo de proyectos
│   └── main.go
├── frontend/                 # UI en React/Vite
│   ├── src/
│   │   ├── pages/            # RegistrarProyecto, EstadoProyectos
│   │   └── services/         # Cliente API
│   └── index.html
├── manifests/                # Manifiestos de Kubernetes del propio deployer
│   ├── backend.yaml
│   ├── frontend.yaml
│   └── ingress.yaml          # Ingress dinámico (deployer.sammcore.local)
├── docs/                     # Especificaciones técnicas y roadmap
│   ├── 01-producto.md
│   ├── 02-tecnico.md
│   ├── 03-roadmap.md
│   ├── 04-templates.md
│   └── 05-database-model-a.md # Especificación del motor de BD Supabase
└── README.md
```

---

## 3. 🧩 Módulos Backend
- `RepoManager`: clona y analiza repositorios.  
- `DatabaseManager`: aprovisiona bases `<proyecto>_db` y usuarios `<proyecto>_user` en el clúster central de Supabase (Modelo A). Detallado en `docs/05-database-model-a.md`.  
- `TemplateManager`: selecciona plantillas y parametriza Deployments, Services y subdominios dinámicos en objetos `Ingress` (`*.sammcore.local`).  
- `SecretManager`: genera y aplica Kubernetes Secrets directamente en el clúster a partir de credenciales en memoria (cero passwords en Git).  
- `DeployManager`: orquesta el ciclo de vida en K3s mediante `client-go`.  
- `StatusManager`: obtiene estado en tiempo real de pods, logs y servicios.  

---

## 4. 🔐 Manejo de credenciales y Base de Datos
- Las contraseñas de bases de datos se generan con alta entropía en memoria por el `DatabaseManager`.  
- El backend las transforma inmediatamente en **Kubernetes Secrets** en el namespace correspondiente.  
- Nunca se guardan en los repositorios de Git ni en el disco del host.  
- Toda carga de trabajo consume sus credenciales mediante `valueFrom.secretKeyRef`.  
- La base de datos es visible y administrable en **Supabase Studio** (`https://supabase.sammcore.local`).  

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
