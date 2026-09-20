# 📄 Seguridad, Gobernanza y Operación – SAMMCORE Deployer

Este documento formaliza las políticas de seguridad, gobernanza de recursos, convenciones de nombres, la matriz completa de variables de entorno y el manual de bootstrap del `sammcore-deployer`.

---

## 1. 🛡️ Políticas de Seguridad y Control de Acceso

### 🔹 1.1. Autenticación de la API y Manejo en Frontend
* Todos los endpoints de mutación y consulta de logs requieren el header:
  ```http
  Authorization: Bearer <DEPLOYER_API_KEY>
  ```
  - Endpoints Protegidos: `POST /api/analyzeRepo`, `POST /api/deploy`, `DELETE /api/projects/:id`, `POST /api/projects/:id/redeploy`, `GET /api/projects/:id/logs`.
  - Endpoints Públicos de Diagnóstico: `GET /api/health`, `GET /metrics`.
* **Manejo en la UI (React/Vite):**  
  La UI solicita la clave al desarrollador en su primera visita mediante un componente modal React accesible desde el Navbar y la almacena localmente en `localStorage` del navegador (`sammcore_deployer_api_key`), inyectándola dinámicamente en los headers de cada llamada a `/api/*`. Si el servidor responde 401, el evento `deployer:auth-required` abre automáticamente la ventana modal.

### 🔹 1.2. Política de CORS (Cross-Origin Resource Sharing)
* Orígenes explícitamente autorizados mediante la variable `ALLOWED_ORIGINS` (con cabecera `Vary: Origin`):
  - Predeterminado: `https://deployer.sammcore.local,http://localhost:5173`.
* Toda petición con un origen fuera de esta lista blanca no recibe cabeceras CORS y las peticiones de preflight `OPTIONS` son rechazadas con código `403 Forbidden`.

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
* **Prohibición de Sufijos Colisionantes:**
  Se prohíben explícitamente nombres de proyecto que terminen en `-api` o `-docs`, dado que los subdominios de backend se estructuran automáticamente como `<proyecto>-api.sammcore.local`.
* **Lista Negra de Nombres Reservados:**
  Se prohíbe el registro de proyectos con nombres que coincidan con:
  `deployer`, `supabase`, `monitoring`, `ingress-nginx`, `default`, `api`, `docs`, `admin`, `grafana`, `prometheus`, `traefik`, `portainer`, o cualquier prefijo `kube-*`.

---

## 3. 📋 Matriz Completa de Variables de Entorno

### Backend (`deployer-backend`):
| Variable | Requerida | Descripción | Valor Predeterminado / Ejemplo |
| :--- | :---: | :--- | :--- |
| `PORT` | Sí | Puerto de escucha HTTP del servidor Go | `8080` |
| `DEPLOYER_API_KEY` | Sí | Clave maestra para autorizar llamadas a la API | Inyectada vía Secret |
| `ALLOW_INSECURE_DEV`| No | Permite arrancar sin API key en desarrollo | `false` |
| `ALLOWED_ORIGINS` | No | Lista blanca de orígenes CORS separados por coma | `https://deployer.sammcore.local,http://localhost:5173` |
| `DATA_DIR` | Sí | Directorio persistente para `history.json` (en PVC) | `/data` |
| `GITHUB_TOKEN` | No | Token de GitHub para aumentar cuota de API | Inyectado vía Secret |
| `SUPABASE_HOST` | Sí | Host interno de PostgreSQL | `postgres.supabase.svc.cluster.local` |
| `SUPABASE_PORT` | Sí | Puerto de PostgreSQL | `5432` |
| `SUPABASE_POSTGRES_PASSWORD` | Sí | Contraseña administrativa de Postgres | Inyectada vía Secret |

### Frontend (`deployer-frontend`):
| Variable | Requerida | Descripción | Valor Predeterminado / Ejemplo |
| :--- | :---: | :--- | :--- |
| `VITE_API_BASE` | Sí | Prefijo base para peticiones HTTP al backend | `/api` |

---

## 4. 🚀 Manual de Bootstrap (Paso a Paso)

Para instalar el deployer desde cero en un clúster K3s nuevo, el administrador ejecuta:

```bash
# 1. Crear namespaces del sistema y de compilación
kubectl apply -f manifests/namespace.yaml
kubectl apply -f manifests/builds-namespace.yaml

# 2. Extraer la contraseña maestra de PostgreSQL en Supabase
SUPABASE_PASS=$(kubectl get secret supabase-postgres-secret -n supabase -o jsonpath='{.data.POSTGRES_PASSWORD}' | base64 -d)

# 3. Generar clave criptográfica para el deployer y mostrarla al administrador
API_KEY=$(openssl rand -hex 16)
echo "======================================================"
echo "DEPLOYER_API_KEY GENERADA: $API_KEY"
echo "Guarde esta clave para configurar su navegador en la UI"
echo "======================================================"

# 4. Crear Secret del Deployer
kubectl create secret generic deployer-secrets -n deployer \
  --from-literal=DEPLOYER_API_KEY="$API_KEY" \
  --from-literal=SUPABASE_HOST="postgres.supabase.svc.cluster.local" \
  --from-literal=SUPABASE_PORT="5432" \
  --from-literal=SUPABASE_POSTGRES_PASSWORD="$SUPABASE_PASS"

# 5. Crear Secret de autenticación para Kaniko en deployer-builds (GHCR / Registro local)
kubectl create secret docker-registry kaniko-registry-secret -n deployer-builds \
  --docker-server=ghcr.io \
  --docker-username=a81biz \
  --docker-password="$GITHUB_TOKEN"

# 6. Aplicar RBAC y almacenamiento persistente (PVC)
kubectl apply -f manifests/rbac.yaml
kubectl apply -f manifests/pvc.yaml

# 7. Desplegar cargas de trabajo e Ingress
kubectl apply -f manifests/backend.yaml
kubectl apply -f manifests/frontend.yaml
kubectl apply -f manifests/ingress.yaml

# 8. Verificación Post-Bootstrap
echo "Verificando pods..."
kubectl get pods -n deployer -w
```

---

## 5. 🔍 Comandos de Verificación Post-Bootstrap

```bash
# 1. Healthcheck público (debe responder {"status":"ok"})
curl -k https://deployer.sammcore.local/api/health

# 2. Verificar rechazo sin autenticación (debe retornar 401)
curl -k -i -X POST https://deployer.sammcore.local/api/analyzeRepo

# 3. Verificar llamada autenticada exitosa
curl -k -X POST https://deployer.sammcore.local/api/analyzeRepo \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"repo":"https://github.com/a81Biz/backroom","branch":"master"}'
```

---

## 6. 🛠️ Guía de Troubleshooting por Estado del Proyecto

| Estado | Síntoma Típico | Causa Probable | Acción Correctiva |
| :--- | :--- | :--- | :--- |
| `analyzed` | Análisis falla inmediatamente | URL inválida o repositorio privado sin token | Verificar formato HTTPS o inyectar `GITHUB_TOKEN` en `deployer-secrets`. |
| `provisioning_db` | Bloqueado en creación de BD | Host de PostgreSQL inaccesible o contraseña incorrecta | Verificar conectividad a `postgres.supabase.svc.cluster.local:5432` y revisar `SUPABASE_POSTGRES_PASSWORD`. |
| `building_image` | Job de Kaniko en CrashLoopBackOff | Fallo de sintaxis en Dockerfile o credenciales GHCR inválidas | Inspeccionar logs del Job en `deployer-builds`: `kubectl logs job/kaniko-build-<proyecto> -n deployer-builds`. |
| `deploying_k8s` | Pods en `Pending` | Cuota de recursos agotada o falta de memoria en nodo | Revisar eventos del namespace: `kubectl describe resourcequota -n <proyecto>`. |
| `failed` | Pods en `CrashLoopBackOff` | Fallo de conexión a BD o variables de entorno faltantes | Consultar logs del pod de la app: `kubectl logs deploy/<proyecto>-api -n <proyecto>`. |
