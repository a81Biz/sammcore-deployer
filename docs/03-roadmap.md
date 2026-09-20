# 📄 Roadmap de Implementación – SAMMCORE Deployer

Este documento define la trayectoria evolutiva y el estado de avance de `sammcore-deployer`, desde los prototipos iniciales hasta la orquestación autónoma en el clúster SAMMCORE.

---

## 🟢 Fase 1: MVP Backend + CLI (✅ Completada)
🎯 Objetivo: backend funcional con capacidad de inspeccionar repositorios y detectar su naturaleza.

### Tareas
- [x] Inicializar `backend/` en Go con dependencias Go modules.
- [x] Implementar módulo `RepoManager`:
  - Clonar repositorios GitHub (`go-git` / git nativo).
  - Detectar si contiene `docker-compose.yml`, `Dockerfile` o código estático.
- [x] CLI de validación con salida JSON estructurada.

---

## 🟢 Fase 2: Backend REST API (✅ Completada)
🎯 Objetivo: exponer las capacidades de análisis e histórico a través de endpoints HTTP.

### Endpoints
- [x] `GET /health` → verificación de liveness y readiness del backend.
- [x] `POST /analyzeRepo` → análisis en tiempo real de repo y branch con detección de tipo.
- [x] `GET /history` → catálogo persistido de proyectos inspeccionados.
- [x] `DELETE /history/:id` → limpieza de entradas en el historial.

---

## 🟢 Fase 3: Frontend React/Vite (✅ Completada)
🎯 Objetivo: interfaz web intuitiva para interactuar con el orquestador.

### Tareas
- [x] Crear aplicación `frontend/` con Vite, React y TypeScript.
- [x] Pantalla **Registrar Proyecto**: formulario de URL y rama con botón de análisis reactivo.
- [x] Pantalla **Estado de Proyectos**: tabla dinámica con badges de estado y acciones.
- [x] Enrutamiento SPA y cliente HTTP configurado con variables de entorno (`VITE_API_BASE`).

---

## 🟡 Fase 4: Orquestación K3s y Supabase Modelo A (🔄 En Progreso)
🎯 Objetivo: ejecutar despliegues reales autónomos en el clúster SAMMCORE con base de datos horizontal y subdominios dinámicos.

### Hitos de la Fase 4:
- [ ] **Hito 4.1: DatabaseManager (Supabase Modelo A)**
  - Conexión administrativa interna a `postgres.supabase.svc.cluster.local:5432`.
  - Aprovisionamiento idempotente de `<proyecto>_db` y rol `<proyecto>_user`.
  - Generación de contraseñas seguras en memoria y asignación estricta de permisos.
  - Validación de visibilidad en **Supabase Studio** (`https://supabase.sammcore.local`).
- [ ] **Hito 4.2: SecretManager**
  - Generación de objetos `Secret` en Kubernetes directamente desde credenciales en memoria.
  - Cero contraseñas en texto plano en Git o disco.
  - Formato estandarizado de variables (`DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USER`, `DB_PASSWORD`, `DATABASE_URL`).
- [ ] **Hito 4.3: IngressManager y Subdominios Dinámicos**
  - Configuración automática de reglas de Ingress en `ingress-nginx` (`NodePort: 30080`).
  - Soporte para múltiples subdominios por proyecto (ej. `sitio.sammcore.local` para Frontend y `api.sitio.sammcore.local` para Backend API).
  - Uso del certificado TLS comodín central (`sammcore-tls`).
- [ ] **Hito 4.4: DeployManager & Endpoint `POST /deploy`**
  - Ensamblado de manifiestos (`Namespace`, `Deployment`, `Service`, `Ingress`, `Secret`).
  - Aplicación al clúster K3s usando `client-go`.
  - Monitoreo del rollout y verificación del estado `Running`.
- [ ] **Hito 4.5: Despliegue Piloto Backroom**
  - Despliegue integral de `https://github.com/a81Biz/backroom`.
  - Validación de endpoints:
    - Web UI: `https://backroom.sammcore.local`
    - API REST: `https://api.backroom.sammcore.local/products`
    - Base de Datos: Tablas creadas e inspeccionables en Supabase Studio.

---

## 🟢 Fase 5: CI/CD y Auto-Despliegue del Deployer (✅ Completada)
🎯 Objetivo: empaquetar y alojar el propio `sammcore-deployer` dentro del clúster K3s.

### Tareas
- [x] Dockerfile optimizado para `backend` (Go binario ligero) y `frontend` (NGINX estático).
- [x] Manifiestos declarativos en `manifests/` (`backend.yaml`, `frontend.yaml`, `ingress.yaml`).
- [x] Despliegue en `namespace: deployer` dentro de K3s.
- [x] Exposición enrutada por Ingress en `https://deployer.sammcore.local`.

---

## ⚪ Fase 6: Panel Avanzado y Observabilidad (Pendiente)
🎯 Objetivo: herramientas avanzadas de administración, métricas y ciclo de vida.

### Tareas
- [ ] Streaming de logs en vivo desde pods de K8s hacia la UI.
- [ ] Dashboard de métricas en Grafana para despliegues (vía `/metrics` en el backend).
- [ ] Opción en UI para desmantelar proyectos (`DELETE /project/:id`) con selector de:
  - *Conservar base de datos para auditoría*.
  - *Eliminar base de datos permanentemente*.
- [ ] Autenticación de acceso al panel deployer.
