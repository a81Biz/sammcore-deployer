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
- [x] Gestión de clave API en Navbar con persistencia en `localStorage`.
- [x] Vista **Registrar Proyecto** (análisis con captura de clave y manejo de errores).
- [x] Vista **Estado de Proyectos** (tabla consumiendo `/api/projects`).
- [x] Integración de variables de entorno (`VITE_API_BASE=/api`).

---

## 🟡 Fase 4: Orquestación K3s y Supabase Modelo A (🔄 En Desarrollo)
🎯 Objetivo: Ejecutar despliegues autónomos completos en el clúster SAMMCORE con base de datos horizontal y subdominios dinámicos.

### 🔹 Hito 4.0: Alineación del Código con el Contrato (✅ Completado)
* **Objetivo:** Refactorizar el backend y frontend para satisfacer 100% el contrato de API, seguridad y modelo de datos.
* **Criterios de Aceptación:**
  1. Prefijo `/api` unificado y eliminación de alias inseguros en la raíz.
  2. Middleware de autenticación Bearer Token activo con `subtle.ConstantTimeCompare`.
  3. Aborto del servidor al arranque si `DEPLOYER_API_KEY` está vacía (salvo `ALLOW_INSECURE_DEV=true`).
  4. CORS estricto con lista blanca configurable y `Vary: Origin`.
  5. Stubs `501 Not Implemented` en `/api/deploy`, `/api/projects/{id}` (DELETE) y `redeploy`.
  6. Detección acotada a la raíz (`compose`, `dockerfile`, `static`) y extracción de puertos.
  7. Detección real de base de datos relacional en compose.
  8. Higiene de disco con `defer os.RemoveAll` garantizado.
  9. Persistencia atómica (`.tmp` + rename) con mutex unificado.
  10. UI con modal de clave en Navbar y llamadas a `/api/projects`.
  11. `go.sum` commiteado y pruebas unitarias passing al 100%.

### 🔹 Hito 4.1: DatabaseManager (Supabase Modelo A)
* **Objetivo:** Conexión interna a `postgres.supabase.svc.cluster.local:5432` con usuario `postgres`.
* **Criterios de Aceptación:**
  1. Matriz de 4 casos de idempotencia implementada (rol existe/no existe, secret existe/no existe).
  2. Ejecución de `GRANT "<proyecto>_user" TO postgres` previa a `CREATE DATABASE` para asegurar ownership sin requerir superusuario.
  3. Aislamiento estricto: `REVOKE ALL ON DATABASE "<proyecto>_db" FROM PUBLIC` y `REVOKE CONNECT ON DATABASE postgres FROM "<proyecto>_user"`.
  4. Límite de conexiones: `CONNECTION LIMIT 20` por usuario de proyecto.
  5. Métricas de base de datos registradas en `postgres-exporter` y visibles en Grafana.

### 🔹 Hito 4.2: SecretManager e Idempotencia
* **Objetivo:** Creación segura de objetos `Secret` en Kubernetes desde memoria.
* **Criterios de Aceptación:**
  1. Si `<proyecto>-db-secrets` ya existe en K8s, recupera la contraseña existente para evitar desincronizar la BD.
  2. Si no existe, crea el objeto `Secret` en el namespace `<proyecto>` con `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD` y `DATABASE_URL`.
  3. Cero contraseñas en texto plano en Git o logs.

### 🔹 Hito 4.3: TemplateManager e Ingress Dinámico
* **Objetivo:** Renderizar manifiestos para los 3 arquetipos (`compose-multi-service`, `single-dockerfile`, `static-web`).
* **Criterios de Aceptación:**
  1. Nombres sanitizados bajo RFC 1123 (`[a-z0-9-]`, 3-35 caracteres) y filtrados contra nombres reservados (`kube-*`, etc.).
  2. Subdominios de nivel único compatibles con Wildcard TLS: `{{ .projectName }}.sammcore.local` (Web) y `{{ .projectName }}-api.sammcore.local` (API).
  3. NetworkPolicy con tráfico intra-namespace (`podSelector: {}`), DNS (UDP/TCP 53) y exclusión de CIDRs privados RFC 1918.
  4. Inclusión obligatoria de `LimitRange`, `requests` y `limits` en todos los contenedores e initContainers.

### 🔹 Hito 4.4: DeployManager y Endpoint `POST /api/deploy`
* **Objetivo:** Aplicar los manifiestos al clúster K3s de forma asíncrona (`202 Accepted`) y monitorear el rollout.
* **Criterios de Aceptación:**
  1. Creación de `Namespace` con label de Pod Security `baseline`.
  2. Aplicación de `ResourceQuota`, `LimitRange`, `NetworkPolicy`, `Deployments`, `Services` y `Ingress`.
  3. Monitoreo activo hasta que los pods alcancen el estado `Running` o timeout.

### 🔹 Hito 4.5: Despliegue Piloto Backroom de Punta a Punta
* **Objetivo:** Desplegar exitosamente `https://github.com/a81Biz/backroom`.
* **Criterios de Aceptación:**
  1. Frontend accesible en `https://backroom.sammcore.local`.
  2. Backend API accesible en `https://backroom-api.sammcore.local/products` con candado SSL válido.
  3. Base de datos `backroom_db` conectada y funcional en PostgreSQL central.

---

## 🟡 Fase 5: CI/CD y Auto-Despliegue del Deployer (🔄 En Progreso)
🎯 Objetivo: Automatizar compilación y despliegue del propio deployer.

### Tareas
- [x] Dockerfile multi-stage para `backend` (Go 1.23 sobre Alpine).
- [x] Dockerfile multi-stage para `frontend` (React/Vite servido por NGINX).
- [x] Manifiestos declarativos en `manifests/`: `namespace.yaml`, `rbac.yaml`, `pvc.yaml`, `backend.yaml`, `frontend.yaml`, `ingress.yaml`.
- [x] Enrutamiento activo en `https://deployer.sammcore.local` (vía Ingress `/` y `/api`).
- [ ] Workflow `.github/workflows/deploy.yml` pendiente de publicación en GitHub (requiere otorgar scope `workflow` a GitHub CLI o push con Personal Access Token).

---

## ⚪ Fase 6: Panel Avanzado y Observabilidad (Pendiente)
🎯 Objetivo: Herramientas de administración avanzada y ciclo de vida.

### Tareas
- [ ] Streaming de logs en vivo desde pods hacia la UI (`GET /api/projects/:id/logs`).
- [ ] Dashboard dedicado en Grafana con métricas exportadas por `/metrics`.
- [ ] Eliminación selectiva en UI (`DELETE /api/projects/:id?delete_db=true|false`).
