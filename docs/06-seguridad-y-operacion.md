# 📄 Seguridad, Gobernanza y Operación – SAMMCORE Deployer

Este documento formaliza las políticas de seguridad, gobernanza de recursos, convenciones de nombres y el manual de bootstrap del `sammcore-deployer`.

---

## 1. 🛡️ Políticas de Seguridad y Control de Acceso

### 🔹 1.1. Autenticación de la API y Manejo en Frontend
* Todos los endpoints de mutación y consulta de logs requieren el header:
  ```http
  Authorization: Bearer <DEPLOYER_API_KEY>
  ```
  - Endpoints Protegidos: `POST /api/analyzeRepo`, `POST /api/deploy`, `DELETE /api/projects/:id`, `POST /api/projects/:id/redeploy`, `GET /api/projects/:id/logs`.
  - Endpoints Públicos de Liveness: `GET /api/health`, `GET /metrics`.
* **Manejo en la UI (React/Vite):**  
  Para evitar exponer la clave en el bundle estático compilado (`VITE_*`), la UI solicita la clave al desarrollador en su primera visita mediante una modal segura y la almacena localmente en `localStorage` del navegador (`deployer_api_key`), inyectándola dinámicamente en los headers de cada llamada a `/api/*`.

### 🔹 1.2. Política de CORS (Cross-Origin Resource Sharing)
* Orígenes explícitamente autorizados en el middleware:
  - `https://deployer.sammcore.local` (interfaz en producción).
  - `http://localhost:5173` (desarrollo local de Vite).
  - `http://localhost:8080` (pruebas locales de backend).
* Toda petición con un origen fuera de esta lista blanca es rechazada.

### 🔹 1.3. Validación de Entrada y Repositorios
* **URL de Repositorio:** Exclusivamente URLs HTTPS válidas que coincidan con la expresión regular:
  ```regex
  ^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\.git)?$
  ```
* Se rechazan esquemas `file://`, `ssh://`, IPs locales o URLs con credenciales incrustadas para prevenir ataques SSRF.

---

## 2. 🏷️ Convención de Nomenclatura

El identificador de proyecto (`projectName`) rige la denominación de los recursos:

* **Formato RFC 1123 Obligatorio:** Minúsculas alfanuméricas y guiones (`-`), longitud entre 3 y 35 caracteres:
  ```regex
  ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
  ```
* **Lista Negra de Nombres Reservados:**
  Se prohíbe el registro de proyectos con nombres que coincidan con:
  `deployer`, `supabase`, `monitoring`, `ingress-nginx`, `default`, `api`, `docs`, `admin`, `grafana`, `prometheus`, `traefik`, `portainer`, o cualquier prefijo `kube-*`.

---

## 3. 🚀 Manual de Bootstrap (Despliegue Inicial de Infraestructura)

Para instalar el deployer desde cero en un clúster K3s nuevo, el administrador ejecuta:

```bash
# 1. Crear namespace
kubectl apply -f manifests/namespace.yaml

# 2. Extraer la contraseña maestra de PostgreSQL en Supabase
SUPABASE_PASS=$(kubectl get secret supabase-postgres-secret -n supabase -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)

# 3. Generar clave criptográfica para el deployer
API_KEY=$(openssl rand -hex 16)

# 4. Crear Secret del Deployer
kubectl create secret generic deployer-secrets -n deployer \
  --from-literal=DEPLOYER_API_KEY="$API_KEY" \
  --from-literal=SUPABASE_HOST="postgres.supabase.svc.cluster.local" \
  --from-literal=SUPABASE_PORT="5432" \
  --from-literal=SUPABASE_POSTGRES_PASSWORD="$SUPABASE_PASS"

# 5. Aplicar RBAC y almacenamiento persistente (PVC)
kubectl apply -f manifests/rbac.yaml
kubectl apply -f manifests/pvc.yaml

# 6. Desplegar cargas de trabajo e Ingress
kubectl apply -f manifests/backend.yaml
kubectl apply -f manifests/frontend.yaml
kubectl apply -f manifests/ingress.yaml
```

---

## 4. 💻 Guía de Desarrollo Local con Port-Forwarding

Para desarrollar localmente sin exponer la base de datos a la LAN física:

```bash
# 1. Crear túnel seguro al PostgreSQL de Supabase en el clúster
kubectl port-forward svc/postgres 5432:5432 -n supabase

# 2. Iniciar el Backend (Go 1.23)
cd sammcore-deployer/backend
export PORT=8080
export DEPLOYER_API_KEY="test-secret-key"
export SUPABASE_HOST="127.0.0.1" # Conecta a través del túnel local
export SUPABASE_PORT="5432"
export SUPABASE_POSTGRES_PASSWORD="<password_de_supabase>"
export DATA_DIR="./data"
go run .

# 3. Iniciar el Frontend (React/Vite)
cd ../frontend
npm install
npm run dev # Disponible en http://localhost:5173
```
