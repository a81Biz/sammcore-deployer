# 📄 Roadmap de Implementación – SAMMCORE Deployer

Este documento define la trayectoria técnica del proyecto, contrastando el estado real actual contra los hitos de entrega y sus criterios de aceptación.

---

## 🟢 Fase 1: MVP Backend + CLI (✅ Completada)
🎯 Objetivo: Módulo base en Go para clonar e identificar el tipo de proyecto.

### Tareas
- [x] Estructura inicial en Go con Go Modules.
- [x] Módulo `RepoManager` con soporte para clonado superficial (`depth: 1`) vía `go-git`.
- [x] Detección inicial de `docker-compose.yml` y `Dockerfile`.
- [x] Pruebas unitarias en `backend/services/repo_manager_test.go`.

---

## 🟢 Fase 2: Backend REST API (✅ Completada)
🎯 Objetivo: Exposición HTTP de la lógica de análisis y registro.

### Tareas
- [x] Router con Gorilla Mux y middleware de CORS.
- [x] Endpoint `GET /health` y `GET /metrics` para Prometheus.
- [x] Endpoint `POST /api/analyzeRepo` con detección de repositorios.
- [x] Endpoints iniciales de historial (`GET /history`, `DELETE /history/{id}`).

---

## 🟢 Fase 3: Frontend React / Vite (✅ Completada)
🎯 Objetivo: Interfaz gráfica para desarrolladores en `https://deployer.sammcore.local`.

### Tareas
- [x] Aplicación SPA con React, TypeScript y Vite.
- [x] Vista **Registrar Proyecto** (formulario reactivo de análisis).
- [x] Vista **Estado de Proyectos** (tabla de proyectos registrados).
- [x] Integración de variables de entorno (`VITE_API_BASE=/api`).

---

## 🟡 Fase 4: Orquestación K3s y Supabase Modelo A (🔄 En Desarrollo)
🎯 Objetivo: Ejecutar despliegues autónomos completos en el clúster SAMMCORE con base de datos horizontal y subdominios dinámicos.

### 🔹 Hito 4.1: DatabaseManager (Supabase Modelo A)
* **Objetivo:** Conexión interna a `postgres.supabase.svc.cluster.local:5432` con usuario `postgres`.
* **Criterios de Aceptación:**
  1. Aprovisionamiento idempotente: Si `<proyecto>_db` no existe, se ejecuta `CREATE DATABASE "<proyecto>_db"`.
  2. Si `<proyecto>_user` no existe, se crea con contraseña criptográfica y se asignan permisos exclusivos sobre `<proyecto>_db`.
  3. Si la base o rol ya existen (redeploy), la función es idempotente y no altera contraseñas existentes.
  4. Métricas de la base de datos se registran en `postgres-exporter` y son visibles en Grafana.

### 🔹 Hito 4.2: SecretManager e Idempotencia
* **Objetivo:** Creación segura de objetos `Secret` en Kubernetes desde memoria.
* **Criterios de Aceptación:**
  1. Si `<proyecto>-db-secrets` ya existe en K8s, recupera la contraseña existente para evitar desincronizar la BD.
  2. Si no existe, crea el objeto `Secret` en el namespace `<proyecto>` con `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD` y `DATABASE_URL`.
  3. Cero contraseñas en texto plano en Git o logs.

### 🔹 Hito 4.3: TemplateManager e Ingress Dinámico
* **Objetivo:** Renderizar manifiestos para los 3 arquetipos (`compose-multi-service`, `single-dockerfile`, `static-web`).
* **Criterios de Aceptación:**
  1. Nombres sanitizados bajo RFC 1123 y filtrados contra la lista de nombres reservados.
  2. Ingress genera reglas sin bloque `tls` interno (la terminación SSL la realiza el NGINX del host en `*.sammcore.local`).
  3. Para Compose multi-servicio: genera regla para `{{ .projectName }}.sammcore.local` (Web) y `api.{{ .projectName }}.sammcore.local` (API).

### 🔹 Hito 4.4: DeployManager y Endpoint `POST /api/deploy`
* **Objetivo:** Aplicar los manifiestos al clúster K3s y monitorear el rollout.
* **Criterios de Aceptación:**
  1. Creación de `Namespace`, `ResourceQuota` (2 CPU, 2Gi RAM) y `NetworkPolicy`.
  2. Aplicación de `Deployments`, `Services` y `Ingress` mediante `client-go`.
  3. Espera activa hasta que los pods alcancen el estado `Running` o reporten `CrashLoopBackOff` con timeout.

### 🔹 Hito 4.5: Despliegue Piloto Backroom de Punta a Punta
* **Objetivo:** Desplegar exitosamente `https://github.com/a81Biz/backroom`.
* **Criterios de Aceptación:**
  1. Frontend accesible en `https://backroom.sammcore.local`.
  2. Backend API accesible en `https://api.backroom.sammcore.local/products`.
  3. Base de datos `backroom_db` conectada y funcional en PostgreSQL central.

---

## 🟢 Fase 5: CI/CD y Auto-Despliegue del Deployer (✅ Completada)
🎯 Objetivo: Empaquetar y alojar el propio deployer dentro de K3s.

### Tareas
- [x] Dockerfile multi-stage para `backend` (Go binario ligero sobre Alpine).
- [x] Dockerfile multi-stage para `frontend` (React/Vite servido por NGINX).
- [x] Manifiestos declarativos en `manifests/`: `namespace.yaml`, `rbac.yaml`, `backend.yaml`, `frontend.yaml`, `ingress.yaml`.
- [x] Workflow `.github/workflows/deploy.yml` para compilar y empujar a GHCR en la rama `master`.
- [x] Enrutamiento activo en `https://deployer.sammcore.local` (vía Ingress `/` y `/api`).

---

## ⚪ Fase 6: Panel Avanzado y Observabilidad (Pendiente)
🎯 Objetivo: Herramientas de administración avanzada y ciclo de vida.

### Tareas
- [ ] Streaming de logs en vivo desde pods hacia la UI (`GET /api/projects/:id/logs`).
- [ ] Dashboard dedicado en Grafana con métricas exportadas por `/metrics`.
- [ ] Eliminación selectiva en UI (`DELETE /api/projects/:id?delete_db=true|false`).
- [ ] Integración de autenticación Bearer Token en la UI.
