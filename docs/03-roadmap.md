# 📄 Roadmap de Implementación – SAMMCORE Deployer

Este documento define la trayectoria técnica del proyecto, contrastando el estado real actual contra los hitos de entrega y sus criterios de aceptación.

---

## 🟢 Fase 1: MVP Backend + CLI (✅ Completada)
🎯 Objetivo: Módulo base en Go para clonar e identificar el tipo de proyecto.

### Tareas
- [x] Estructura inicial en Go 1.23 con Go Modules.
- [x] Módulo `RepoManager` con soporte para clonado superficial (`depth: 1`) vía `go-git`.
- [x] Detección de 3 arquetipos: `compose`, `dockerfile` y `static` acotada a la raíz.
- [x] Pruebas unitarias en `backend/services/repo_manager_test.go`.

---

## 🟢 Fase 2: Backend REST API (✅ Completada)
🎯 Objetivo: Exposición HTTP de la lógica de análisis y registro.

### Tareas
- [x] Router con Gorilla Mux y middleware de CORS estricto (`Vary: Origin`, lista blanca).
- [x] Endpoint `GET /api/health` y `GET /metrics` para Prometheus.
- [x] Endpoint `POST /api/analyzeRepo` con autenticación Bearer y validación de repositorios.
- [x] Eliminación de rutas no autenticadas en la raíz.

---

## 🟢 Fase 3: Frontend React / Vite (✅ Completada)
🎯 Objetivo: Interfaz gráfica para desarrolladores en `https://deployer.sammcore.local`.

### Tareas
- [x] Aplicación SPA con React, TypeScript y Vite.
- [x] Gestión de clave API en Navbar con persistencia en `localStorage` y componente modal React.
- [x] Vista **Registrar Proyecto** (análisis con captura de clave y manejo de errores).
- [x] Vista **Estado de Proyectos** (tabla consumiendo `/api/projects`).
- [x] Integración de variables de entorno (`VITE_API_BASE=/api`).

---

## 🟡 Fase 4: Orquestación K3s y Supabase Modelo A (🔄 En Desarrollo)
🎯 Objetivo: Ejecutar despliegues autónomos completos en el clúster SAMMCORE con base de datos horizontal y subdominios dinámicos.

### 🔹 Hito 4.0: Alineación del Código con el Contrato (✅ Completado y Verificado)
* **Objetivo:** Refactorizar backend, frontend y manifiestos para satisfacer 100% el contrato de API, seguridad, aislamiento y modelo de datos.
* **Criterios de Aceptación Cumplidos y Auditados:**
  1. **Prefijo `/api` unificado:** Eliminación absoluta de rutas públicas huérfanas en el router raíz.
  2. **Autenticación Bearer Token Falla Cerrada:** Middleware con `subtle.ConstantTimeCompare`, obligatoriedad de `Bearer `, y aborto al inicio si `DEPLOYER_API_KEY` está vacía (salvo `ALLOW_INSECURE_DEV=true`).
  3. **CORS Estricto:** Lista blanca configurable vía `ALLOWED_ORIGINS`, `Vary: Origin` y rechazo 403 en OPTIONS ajenos.
  4. **Análisis No Persistente (Read-Only):** `POST /api/analyzeRepo` devuelve `AnalyzePlan` sin tocar `history.json` ni mutar el estado de proyectos en ejecución.
  5. **Parseo YAML Estructurado de Compose:** Soporte para `docker-compose.yml`, `docker-compose.yaml`, `compose.yml` y `compose.yaml` mediante `gopkg.in/yaml.v3`. Extracción exacta del puerto de contenedor (`target`) y descarte total de falsos positivos por comentarios.
  6. **Clonado Seguro con Timeout:** `context.WithTimeout(2*time.Minute)` y rechazo inmediato si la rama explícita solicitada no existe (sin fallback silencioso a master o default).
  7. **Identificadores Deterministas y Sanitización:** IDs calculados como `owner-repo`, sanitización RFC 1123, prefijo automático `app-` para nombres reservados o cortos, y prohibición explícita de nombres que terminen en `-api` o `-docs`.
  8. **Protección contra Corrupción en Store:** Si `history.json` contiene JSON inválido, se crea respaldo `history.json.corrupt.<timestamp>` y se aborta con error en lugar de sobreescribir con array vacío.
  9. **Stubs 501 Not Implemented:** `/api/deploy`, `/api/projects/{id}` (DELETE), `/api/projects/{id}/logs` y `redeploy` responden 501 con modelo de error uniforme `{"status":"error","error":"...","code":"NOT_IMPLEMENTED"}`.
  10. **Aislamiento de Builds:** Manifiesto `manifests/builds-namespace.yaml` para aislar compilaciones de Kaniko con `ResourceQuota`, `LimitRange` y `NetworkPolicy`.
  11. **Frontend Moderno:** Modal React nativo para configuración de clave API y captura de errores 401 sin cuadros `prompt()` de navegador.
  12. **Pruebas Unitarias al 100%:** Suites completas pasando en `api`, `core`, `services` y `storage`.

### 🔹 Hito 4.1: DatabaseManager (Supabase Modelo A) (✅ Completado)
* **Objetivo:** Conexión interna a `postgres.supabase.svc.cluster.local:5432` con usuario `postgres`.
* **Criterios de Aceptación:**
  - [x] Matriz de 4 casos de idempotencia implementada (rol existe/no existe, secret existe/no existe).
  - [x] Ejecución de `GRANT "<proyecto>_user" TO postgres` previa a `CREATE DATABASE` para asegurar ownership sin requerir superusuario.
  - [x] Aislamiento estricto: `REVOKE ALL ON DATABASE "<proyecto>_db" FROM PUBLIC` y `REVOKE CONNECT ON DATABASE postgres FROM "<proyecto>_user"`.
  - [x] Límite de conexiones: `CONNECTION LIMIT 20` por usuario de proyecto en `CREATE` y `ALTER ROLE`.
  - [x] Purga limpia con `delete_db=true` (`pg_terminate_backend`, `DROP DATABASE`, `REVOKE`, `DROP USER`).
  - [x] Implementación en `services/db_manager.go` con pruebas unitarias en `services/db_manager_test.go`.

### 🔹 Hito 4.2: SecretManager e Idempotencia (✅ Completado)
* **Objetivo:** Creación segura de objetos `Secret` en Kubernetes desde memoria.
* **Criterios de Aceptación:**
  - [x] Si `<proyecto>-db-secrets` ya existe en K8s, recupera la contraseña existente para evitar desincronizar la BD.
  - [x] Si no existe, crea el objeto `Secret` en el namespace `<proyecto>` con `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD` y `DATABASE_URL`.
  - [x] Cero contraseñas en texto plano en Git o logs.
  - [x] Implementación en `services/secret_manager.go` con pruebas unitarias en `services/secret_manager_test.go`.

### 🔹 Hito 4.3: TemplateManager e Ingress Dinámico (✅ Completado)
* **Objetivo:** Renderizar manifiestos para los 3 arquetipos (`compose-multi-service`, `single-dockerfile`, `static-web`).
* **Criterios de Aceptación:**
  - [x] Nombres sanitizados bajo RFC 1123 (`[a-z0-9-]`, 3-35 caracteres) y filtrados contra nombres reservados (`kube-*`, etc.).
  - [x] Subdominios de nivel único compatibles con Wildcard TLS: `{{ .projectName }}.sammcore.local` (Web) y `{{ .projectName }}-api.sammcore.local` (API).
  - [x] Inclusión de `imagePullSecrets: [{name: sammcore-registry-secret}]` en los pods para descarga de imágenes de GHCR.
  - [x] NetworkPolicy con tráfico intra-namespace (`podSelector: {}`), DNS (UDP/TCP 53) y exclusión de CIDRs privados RFC 1918.
  - [x] Inclusión obligatoria de `LimitRange`, `requests` y `limits` en todos los contenedores e initContainers.
  - [x] Implementación en `services/template_manager.go` con pruebas unitarias en `services/template_manager_test.go`.

### 🔹 Hito 4.4: DeployManager y Endpoints REST (✅ Completado)
* **Objetivo:** Aplicar los manifiestos al clúster K3s de forma asíncrona (`202 Accepted`), endpoints de ciclo de vida y monitoreo de rollout.
* **Criterios de Aceptación:**
  - [x] Creación de `Namespace` con label de Pod Security `baseline`.
  - [x] Aplicación de `ResourceQuota`, `LimitRange`, `NetworkPolicy`, `Deployments`, `Services` e `Ingress` mediante `client-go`.
  - [x] Endpoint `POST /api/deploy` asíncrono con respuesta `202 Accepted`.
  - [x] Endpoint `DELETE /api/projects/:id?delete_db=true|false` con purga de Namespace y PostgreSQL.
  - [x] Endpoint `GET /api/projects/:id/logs` con streaming/tail de pods en vivo vía K8s API.
  - [x] Endpoint `POST /api/projects/:id/redeploy` con actualización de estado y re-ejecución.
  - [x] Implementación en `services/deploy_manager.go` y `api/router.go` con pruebas en `deploy_manager_test.go` y `router_test.go`.

### 🔹 Hito 4.5: Despliegue Piloto Backroom de Punta a Punta (✅ Completado)
* **Objetivo:** Desplegar exitosamente `https://github.com/a81Biz/backroom`.
* **Criterios de Aceptación:**
  1. [x] Frontend accesible en `https://backroom.sammcore.local` (HTTP 200 OK con single-page application Vite/React).
  2. [x] Backend API accesible en `https://backroom-api.sammcore.local/health` y `https://backroom-api.sammcore.local/api/products`.
  3. [x] Base de datos `backroom_db` conectada y funcional en PostgreSQL central de Supabase (Modelo A con 5 tablas migradas y aisladas bajo el rol `backroom_user`).

---

## 🟢 Fase 5: CI/CD y Auto-Despliegue del Deployer (✅ Completada)
🎯 Objetivo: Automatizar compilación y despliegue del propio deployer.

### Tareas
- [x] Dockerfile multi-stage para `backend` (Go 1.23 sobre Alpine).
- [x] Dockerfile multi-stage para `frontend` (React/Vite servido por NGINX).
- [x] Manifiestos declarativos en `manifests/`: `namespace.yaml`, `builds-namespace.yaml`, `rbac.yaml`, `pvc.yaml`, `backend.yaml`, `frontend.yaml`, `ingress.yaml`.
- [x] Enrutamiento activo en `https://deployer.sammcore.local` (vía Ingress `/` y `/api`).
- [x] Workflow de CI/CD para GitHub Actions en `manifests/ci/deploy.yml` para publicación continua de imágenes en `ghcr.io`.

---

## ⚪ Fase 6: Panel Avanzado y Observabilidad (Pendiente)
🎯 Objetivo: Herramientas de administración avanzada y ciclo de vida.

### Tareas
- [ ] Streaming de logs en vivo desde pods hacia la UI (`GET /api/projects/:id/logs`).
- [ ] Dashboard dedicado en Grafana con métricas exportadas por `/metrics`.
- [ ] Eliminación selectiva en UI (`DELETE /api/projects/:id?delete_db=true|false`).
