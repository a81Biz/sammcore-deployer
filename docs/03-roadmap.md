# 📄 Roadmap de Implementación – SAMMCORE Deployer

Este documento define la trayectoria técnica del proyecto, contrastando el estado real actual contra los hitos de entrega y sus criterios de aceptación.

---

## 🟢 Fase 1: MVP Backend + CLI (✅ Completada)
🎯 Objetivo: Módulo base en Go para clonar e identificar el tipo de proyecto.

### Tareas
- [x] Estructura inicial en Go 1.23 con Go Modules.
- [x] Módulo `RepoManager` con soporte para clonado superficial (`depth: 1`) vía `go-git`.
- [x] Detección de 3 arquetipos: `compose`, `dockerfile` y `static`.
- [x] Pruebas unitarias en `backend/services/repo_manager_test.go`.

---

## 🟢 Fase 2: Backend REST API (✅ Completada)
🎯 Objetivo: Exposición HTTP de la lógica de análisis y registro.

### Tareas
- [x] Router con Gorilla Mux y middleware de CORS estricto.
- [x] Endpoint `GET /api/health` y `GET /metrics` para Prometheus.
- [x] Endpoint `POST /api/analyzeRepo` con detección de repositorios.
- [x] Endpoints iniciales de historial (`GET /api/projects`, `DELETE /api/projects/{id}`).

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

### 🔹 Hito 4.0: Alineación del Código con el Contrato (✅ Completado)
* **Objetivo:** Refactorizar el backend para satisfacer 100% el contrato de API, seguridad y modelo de datos antes de crear nuevos servicios.
* **Criterios de Aceptación:**
  1. Prefijo `/api` aplicado en todas las rutas del router.
  2. Middleware de autenticación Bearer Token activo y CORS restringido a la lista blanca.
  3. Detección de sitios estáticos (`ProjectStatic`) y limpieza inmediata con `defer os.RemoveAll`.
  4. Struct `Project` enriquecido y persistencia configurable vía variable de entorno `DATA_DIR`.
  5. Pruebas unitarias pasando al 100%.

### 🔹 Hito 4.1: DatabaseManager (Supabase Modelo A)
* **Objetivo:** Conexión interna a `postgres.supabase.svc.cluster.local:5432` con usuario `postgres`.
* **Criterios de Aceptación:**
  1. Aprovisionamiento idempotente: Si `<proyecto>_db` no existe, se ejecuta `CREATE DATABASE "<proyecto>_db"`.
  2. Si `<proyecto>_user` no existe, se crea con contraseña criptográfica y se ejecuta `GRANT "<proyecto>_user" TO postgres` para garantizar la asignación de propiedad sin requerir superusuario.
  3. Si la base o rol ya existen y se provee la contraseña existente del Secret, no se alteran contraseñas en PostgreSQL.
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
  1. Nombres sanitizados bajo RFC 1123 y filtrados contra la lista de nombres reservados (`kube-*`, etc.).
  2. Ingress genera reglas sin bloque `tls` interno (la terminación SSL la realiza el NGINX del host en `*.sammcore.local`).
  3. NetworkPolicy incluye regla intra-namespace (`podSelector: {}`), DNS (UDP/TCP 53) y exclusión de CIDRs privados RFC 1918.

### 🔹 Hito 4.4: DeployManager y Endpoint `POST /api/deploy`
* **Objetivo:** Aplicar los manifiestos al clúster K3s y monitorear el rollout.
* **Criterios de Aceptación:**
  1. Creación de `Namespace`, `ResourceQuota` (2 CPU, 2Gi RAM) y `NetworkPolicy`.
  2. Aplicación de `Deployments`, `Services` y `Ingress` mediante `client-go`.
  3. Monitoreo activo hasta que los pods alcancen el estado `Running` o reporten timeout.

### 🔹 Hito 4.5: Despliegue Piloto Backroom de Punta a Punta
* **Objetivo:** Desplegar exitosamente `https://github.com/a81Biz/backroom`.
* **Criterios de Aceptación:**
  1. Frontend accesible en `https://backroom.sammcore.local`.
  2. Backend API accesible en `https://api.backroom.sammcore.local/products`.
  3. Base de datos `backroom_db` conectada y funcional en PostgreSQL central.

---

## 🟢 Fase 5: CI/CD y Auto-Despliegue del Deployer (✅ Manifiestos y Dockerfiles en Repo)
🎯 Objetivo: Empaquetar y alojar el propio deployer dentro de K3s.

### Tareas
- [x] Dockerfile multi-stage para `backend` (Go 1.23 sobre Alpine).
- [x] Dockerfile multi-stage para `frontend` (React/Vite servido por NGINX).
- [x] Manifiestos declarativos en `manifests/`: `namespace.yaml`, `rbac.yaml` (con `apiGroup: rbac.authorization.k8s.io`), `pvc.yaml`, `secret.example.yaml`, `backend.yaml`, `frontend.yaml`, `ingress.yaml`.
- [x] Enrutamiento activo en `https://deployer.sammcore.local` (vía Ingress `/` y `/api`).
- [ ] Publicación del workflow `.github/workflows/deploy.yml` (pendiente de conceder el scope `workflow` en la autorización de GitHub CLI o push con token personal).

---

## ⚪ Fase 6: Panel Avanzado y Observabilidad (Pendiente)
🎯 Objetivo: Herramientas de administración avanzada y ciclo de vida.

### Tareas
- [ ] Streaming de logs en vivo desde pods hacia la UI (`GET /api/projects/:id/logs`).
- [ ] Dashboard dedicado en Grafana con métricas exportadas por `/metrics`.
- [ ] Eliminación selectiva en UI (`DELETE /api/projects/:id?delete_db=true|false`).
- [ ] Modal en UI para captura y almacenamiento local de la `DEPLOYER_API_KEY`.
