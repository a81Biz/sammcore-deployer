# 📄 Roadmap de Implementación – SAMMCORE Deployer

Este documento define las fases de desarrollo de `sammcore-deployer`, desde un MVP mínimo hasta la integración completa en SAMMCORE.

---

## 🟢 Fase 1: MVP Backend + CLI
🎯 Objetivo: contar con un backend funcional que pueda analizar repos y desplegar usando plantillas, sin frontend.

### Tareas
- [ ] Inicializar `backend/` en Go.
- [ ] Implementar módulo `RepoManager`:
  - Clonar repositorios GitHub.
  - Detectar si contiene `docker-compose.yml`, `Dockerfile` o código estático.
- [ ] Implementar módulo `TemplateManager`:
  - Cargar templates desde `sammcore-templates`.
  - Generar manifests en base a tipo de proyecto.
- [ ] Implementar módulo `DeployManager`:
  - Construir imagen Docker.
  - Push a GHCR.
  - Aplicar manifests en K3s (`kubectl` o `client-go`).
- [ ] Crear CLI temporal (`main.go`) para pruebas locales.

---

## 🟡 Fase 2: Backend REST API
🎯 Objetivo: exponer la lógica como API REST para interacción con el futuro frontend.

### Endpoints
- [ ] `POST /analyzeRepo` → analiza repositorio y devuelve tipo.
- [ ] `POST /createSecrets` → crea `Secrets` en Kubernetes.
- [ ] `POST /deploy` → construye y despliega.
- [ ] `GET /status/:projectName` → estado de pods, servicios y URL.
- [ ] `DELETE /project/:projectName` → elimina namespace y recursos.

---

## 🟠 Fase 3: Frontend básico (React/Vite)
🎯 Objetivo: crear una interfaz mínima para registrar proyectos y ver estado.

### Tareas
- [ ] Crear proyecto `frontend/` con Vite.
- [ ] Pantalla **Registrar Proyecto**:
  - Campos: nombre, repo URL, tipo, dominio.
  - Botón “Analizar Repo”.
- [ ] Pantalla **Estado de proyectos**:
  - Tabla con nombre, URL, estado.
  - Botones: “Logs”, “Redeploy”, “Eliminar”.

---

## 🔵 Fase 4: Integración con K3s
🎯 Objetivo: ejecutar despliegues reales en el clúster SAMMCORE.

### Tareas
- [ ] Montar `KUBECONFIG` dentro del contenedor del backend.
- [ ] Probar despliegue de proyecto estático (ej. ElJuegoDLaVida).
- [ ] Probar despliegue de proyecto multi-servicio (ej. PHPtest).
- [ ] Confirmar creación de namespaces, Deployments, Services e Ingress.
- [ ] Validar que Secrets funcionan correctamente.

---

## 🟣 Fase 5: CI/CD del Deployer
🎯 Objetivo: automatizar despliegue del propio `sammcore-deployer`.

### Tareas
- [ ] Crear `Dockerfile` para backend y frontend.
- [ ] Crear workflow `.github/workflows/deploy.yml`:
  - Build → Push a GHCR.
  - Aplicar manifests con `kubectl`.
- [ ] Manifests `deployment.yaml`, `service.yaml`, `ingress.yaml` en `manifests/`.
- [ ] Exponer en `https://deployer.sammcore.local`.

---

## ⚫ Fase 6: Panel Avanzado
🎯 Objetivo: extender funcionalidades para administración completa.

### Mejoras
- [ ] Logs en tiempo real desde pods.
- [ ] Historial de despliegues por proyecto.
- [ ] Gestión de Secrets desde UI.
- [ ] Integración con repos privados (tokens SSH/https).
- [ ] Autenticación de usuarios para el panel.

---

## 📅 Prioridades
1. MVP Backend (Go + CLI).
2. API REST + Frontend básico.
3. Integración con K3s y validación con ElJuegoDLaVida.
4. Traducción de `docker-compose.yml` (PHPtest).
5. CI/CD del propio deployer.
6. Extensiones avanzadas de panel y seguridad.
