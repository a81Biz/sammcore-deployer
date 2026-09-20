# 🏗️ SAMMCORE Deployer

El **SAMMCORE Deployer** es el orquestador central que permite a los desarrolladores publicar proyectos en el servidor **SAMMCORE** de forma modular, segura y automatizada sobre Kubernetes (**K3s**), sin requerir la redacción manual de manifiestos ni la gestión de infraestructura compleja.

---

## 📚 Documentación Técnica (Fuente de Verdad)

Las especificaciones arquitectónicas y operativas se dividen en los siguientes documentos normativos:

1. **[01-producto.md](docs/01-producto.md)** → Visión de producto, arquetipos soportados (`compose`, `dockerfile`, `static`), usuarios y flujos.
2. **[02-tecnico.md](docs/02-tecnico.md)** → Arquitectura K3s, contrato único de API REST (`/api/...`), payload de despliegue, Kaniko y persistencia en PVC.
3. **[03-roadmap.md](docs/03-roadmap.md)** → Plan de implementación, Hito 4.0 completado y criterios de aceptación técnicos por hito (Fases 1 a 6).
4. **[04-templates.md](docs/04-templates.md)** → Reglas de traducción Compose a K8s, especificaciones completas de manifiestos por arquetipo, ResourceQuota, NetworkPolicy corregida y Kaniko Job.
5. **[05-database-model-a.md](docs/05-database-model-a.md)** → Especificación del Modelo A en PostgreSQL central de Supabase, mapeo de nombres (`-` a `_`), permisos sin superusuario (`GRANT TO postgres`) e idempotencia estricta.
6. **[06-seguridad-y-operacion.md](docs/06-seguridad-y-operacion.md)** → Autenticación Bearer Token, CORS estricto, nombres reservados (`kube-*`), manual de bootstrap completo y desarrollo local con `kubectl port-forward`.
7. **[Bitácora Histórica](docs/bitacora/)** → Registro cronológico no normativo de fases de desarrollo previas.

---

## 🎯 Diferencia Clave
* **Aplicaciones de Usuario:** Se analizan, compilan, aprovisionan y despliegan de manera automatizada a través del `sammcore-deployer`.
* **SAMMCORE Deployer:** Es una aplicación fundacional que reside en el namespace `deployer` dentro de K3s, expuesta en `https://deployer.sammcore.local`.

---

## ⚙️ Tecnologías
* **Backend:** Go 1.23 (`client-go` para K8s, `go-git` para clonado, `lib/pq` para PostgreSQL central).
* **Frontend:** React + TypeScript + Vite servido con NGINX estático.
* **Orquestación:** K3s con `ingress-nginx` (NodePort 30080) y terminación SSL en el host.
* **Base de Datos Horizontal:** Supabase PostgreSQL 15 (Modelo A).
* **Registro de Contenedores:** Docker Registry local en K3s (`sammcore-registry:5000`, NodePort 30500) con compilación in-cluster vía Kaniko.

---

## 📌 Estado del Proyecto

| Fase / Hito | Descripción | Estado |
| :--- | :--- | :--- |
| **Fase 1: MVP Backend** | Detección de arquetipos y pruebas unitarias de clonado | ✅ Completada |
| **Fase 2: Backend REST API** | Endpoints HTTP `/api/analyzeRepo`, `/api/health`, `/metrics` | ✅ Completada |
| **Fase 3: Frontend Inicial** | Interfaz React/Vite en `https://deployer.sammcore.local` | ✅ Completada |
| **Hitos 4.1 a 4.4: Orquestación K3s** | `DatabaseManager`, `SecretManager`, `TemplateManager`, `DeployManager` | ✅ Completados |
| **Deployer v2: Pipeline Kaniko** | Compilación in-cluster secuencial, registry local K3s, eliminación de ErrImagePull | ✅ Completada |
| **Hito 4.5: Piloto Backroom** | Despliegue de Backroom 100% operativo en producción (`backend`, `frontend`, `worker`) | ✅ Completado |
| **Fase 5: Manifiestos y CI/CD** | Manifiestos K8s, secrets y template de workflow en `manifests/` | ✅ Completada |
| **Fase 6: Panel Avanzado** | Streaming de logs limpios en vivo (sin ANSI), métricas dinámicas en tiempo real | ✅ Completada |
