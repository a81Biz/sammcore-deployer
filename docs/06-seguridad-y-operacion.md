# 📄 Seguridad, Gobernanza y Operación – SAMMCORE Deployer

Este documento formaliza las políticas de seguridad, gobernanza de recursos, convenciones de nombres y manual operativo del `sammcore-deployer`.

---

## 1. 🛡️ Políticas de Seguridad y Control de Acceso

### 🔹 1.1. Autenticación de la API
* Todas las peticiones mutables (`POST /api/deploy`, `DELETE /api/projects/:id`, `POST /api/projects/:id/redeploy`) requieren el header:
  ```http
  Authorization: Bearer <DEPLOYER_API_KEY>
  ```
* La clave `DEPLOYER_API_KEY` se inyecta en el contenedor del backend mediante el Secret de Kubernetes `deployer-secrets`.
* Las peticiones de solo lectura (`GET /api/health`, `GET /metrics`) son públicas dentro de la red del clúster.

### 🔹 1.2. Política de CORS (Cross-Origin Resource Sharing)
* Se elimina la cabecera permisiva `Access-Control-Allow-Origin: *`.
* Orígenes explícitamente autorizados:
  - `https://deployer.sammcore.local` (interfaz en producción).
  - `http://localhost:5173` (desarrollo local con Vite).

### 🔹 1.3. Validación de Entrada y Repositorios
* **URL de Repositorio:** Exclusivamente URLs HTTPS válidas que coincidan con la expresión regular:
  ```regex
  ^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\.git)?$
  ```
* Se rechaza cualquier protocolo `file://`, `ssh://`, o URLs locales que apunten a la red interna para evitar ataques SSRF (Server-Side Request Forgery).

---

## 2. 🏷️ Convención de Nomenclatura

El nombre del proyecto (`projectName`) determina el nombre del `Namespace`, los `Services`, las bases de datos y los subdominios DNS:

* **Formato RFC 1123 Obligatorio:** Minúsculas alfanuméricas y guiones (`-`), longitud entre 3 y 40 caracteres:
  ```regex
  ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
  ```
* **Lista Negra de Nombres Reservados:**
  Se prohíben estrictamente los siguientes nombres para evitar colisiones con la infraestructura central de SAMMCORE:
  `deployer`, `supabase`, `monitoring`, `ingress-nginx`, `kube-system`, `kube-public`, `kube-node-lease`, `default`, `api`, `docs`, `admin`, `grafana`, `prometheus`, `traefik`, `portainer`.

---

## 3. ⚖️ Gobernanza de Recursos en K3s

Cada namespace de aplicación recibe automáticamente directivas de contención para evitar que un proyecto afecte la estabilidad del servidor físico:

* **ResourceQuota:**
  - `requests.cpu: "500m"`, `limits.cpu: "2000m"` (máximo 2 vCPUs).
  - `requests.memory: "512Mi"`, `limits.memory: "2Gi"` (máximo 2 GB de RAM).
  - `pods: "10"` (límite de pods concurrentes por proyecto).
* **NetworkPolicy:**
  - **Ingress:** Solo se permite tráfico proveniente del namespace `ingress-nginx`.
  - **Egress:** Solo se permite tráfico hacia `kube-system` (puerto 53 UDP/TCP para DNS), hacia `supabase` (puerto 5432 TCP para PostgreSQL) y hacia Internet público (excluyendo subredes de pods 10.42.0.0/16 y de servicios 10.43.0.0/16).

---

## 4. 📋 Matriz de Variables de Entorno

### Backend (`deployer-backend`):
| Variable | Requerida | Descripción | Valor Predeterminado / Ejemplo |
| :--- | :--- | :--- | :--- |
| `PORT` | Sí | Puerto de escucha del servidor Go | `8080` |
| `DEPLOYER_API_KEY` | Sí | Token para autorizar llamadas a la API | Inyectado vía Secret |
| `GITHUB_TOKEN` | No | Token de GitHub para aumentar cuota de API y clonar repos privados | Inyectado vía Secret |
| `SUPABASE_HOST` | Sí | Host interno de PostgreSQL | `postgres.supabase.svc.cluster.local` |
| `SUPABASE_PORT` | Sí | Puerto de PostgreSQL | `5432` |
| `SUPABASE_POSTGRES_PASSWORD` | Sí | Contraseña superusuario de Postgres | Inyectada vía Secret |
| `DATA_DIR` | Sí | Directorio de persistencia para `history.json` | `/data` (montado en PVC) |

### Frontend (`deployer-frontend`):
| Variable | Requerida | Descripción | Valor Predeterminado / Ejemplo |
| :--- | :--- | :--- | :--- |
| `VITE_API_BASE` | Sí | Ruta base para peticiones HTTP al backend | `/api` |

---

## 5. 💻 Guía de Ejecución en Desarrollo Local

Para probar el backend y frontend localmente en una estación de trabajo:

```bash
# 1. Backend (Go)
cd sammcore-deployer/backend
export PORT=8080
export DEPLOYER_API_KEY="test-secret-key"
export SUPABASE_HOST="192.168.68.107" # IP del host SAMMCORE
export SUPABASE_PORT="5432"
export SUPABASE_POSTGRES_PASSWORD="<password_de_supabase>"
export DATA_DIR="./data"
go run .

# 2. Frontend (React/Vite)
cd ../frontend
npm install
npm run dev # Inicia en http://localhost:5173
```
