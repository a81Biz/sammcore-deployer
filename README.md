# 🏗️ SAMMCORE Deployer

El **SAMMCORE Deployer** es el orquestador central que permite a los desarrolladores publicar proyectos en el servidor **SAMMCORE** de forma modular, segura y automatizada sobre Kubernetes (**K3s**), sin requerir la redacción manual de manifiestos ni la gestión de infraestructura compleja.

---

## 📚 Documentación Técnica (Fuente de Verdad)

Las especificaciones arquitectónicas y operativas se dividen en los siguientes documentos normativos:

1. **[01-producto.md](docs/01-producto.md)** → Visión de producto, arquetipos soportados (`compose`, `dockerfile`, `static`), usuarios y flujos.
2. **[02-tecnico.md](docs/02-tecnico.md)** → Arquitectura K3s, contrato único de API REST (`/api/...`), módulos del backend, compilación con Kaniko y persistencia en PVC.
3. **[03-roadmap.md](docs/03-roadmap.md)** → Plan de implementación y criterios de aceptación técnicos por hito (Fases 1 a 6).
4. **[04-templates.md](docs/04-templates.md)** → Reglas de traducción Compose a K8s, especificaciones completas de manifiestos, ResourceQuota, NetworkPolicy y enrutamiento dinámico sin TLS interno.
5. **[05-database-model-a.md](docs/05-database-model-a.md)** → Especificación del Modelo A en PostgreSQL central de Supabase, aislamiento, idempotencia de contraseñas y realidad técnica de Supabase Studio.
6. **[06-seguridad-y-operacion.md](docs/06-seguridad-y-operacion.md)** → Autenticación Bearer Token, CORS estricto, nombres reservados RFC 1123, variables de entorno y guía de desarrollo local.
7. **[Bitácora Histórica](docs/bitacora/)** → Registro cronológico de avances de fases previas (`fase-1-cli.md`, `fase-2-api.md`, `fase-3-frontend.md`, `fase-4-analisis-inicial.md`).

---

## 🎯 Diferencia Clave
* **Aplicaciones de Usuario:** Se analizan, compilan, aprovisionan y despliegan de manera automatizada a través del `sammcore-deployer`.
* **SAMMCORE Deployer:** Es una aplicación fundacional que reside en el namespace `deployer` dentro de K3s, con su propio pipeline de CI/CD hacia GHCR, expuesta en `https://deployer.sammcore.local`.

---

## ⚙️ Tecnologías
* **Backend:** Go 1.22 (`client-go` para K8s, `go-git` para clonado, `lib/pq` para PostgreSQL central).
* **Frontend:** React + TypeScript + Vite servido con NGINX estático.
* **Orquestación:** K3s en `192.168.68.107` con `ingress-nginx` (NodePort 30080).
* **Base de Datos Horizontal:** Supabase PostgreSQL 15 (Modelo A).
* **CI/CD:** GitHub Actions (`.github/workflows/deploy.yml` y `test.yml`).
* **Registro de Contenedores:** GitHub Container Registry (GHCR).

---

## 📌 Estado del Proyecto

| Fase | Descripción | Estado |
| :--- | :--- | :--- |
| **Fase 1: MVP Backend** | Detección de arquetipos y pruebas unitarias de clonado | ✅ Completada |
| **Fase 2: Backend REST API** | Endpoints HTTP `/api/analyzeRepo`, `/health`, `/metrics` | ✅ Completada |
| **Fase 3: Frontend Inicial** | Interfaz React/Vite en `https://deployer.sammcore.local` | ✅ Completada |
| **Fase 4: Orquestación K3s + Supabase** | `DatabaseManager`, `SecretManager`, `TemplateManager` y despliegue Backroom | 🔄 En Especificación / Desarrollo |
| **Fase 5: CI/CD del Deployer** | Dockerfiles multi-stage, manifiestos K8s (`namespace`, `rbac`, `ingress`) y workflow GHCR | ✅ Implementada en repo |
| **Fase 6: Panel Avanzado** | Streaming de logs, dashboards en Grafana y autenticación en UI | ⚪ Pendiente |
