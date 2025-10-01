# 🏗️ SAMMCORE Deployer

El **SAMMCORE Deployer** es una aplicación web que permite a los usuarios desplegar proyectos en el servidor **SAMMCORE** de forma modular y automatizada, sin necesidad de dominar Kubernetes.  
Es el punto de entrada central para crear aplicaciones a partir de repositorios GitHub, empaquetarlas en contenedores y exponerlas en el clúster K3s.

---

## 📚 Documentación

- [01-producto.md](docs/01-producto.md) → Visión del producto, usuarios objetivo, alcance y casos de uso.  
- [02-tecnico.md](docs/02-tecnico.md) → Arquitectura técnica, módulos, repositorio y CI/CD.  
- [03-roadmap.md](docs/03-roadmap.md) → Plan de implementación paso a paso (MVP → versión completa).  
- [04-templates.md](docs/04-templates.md) → Estructura y uso de plantillas para distintos tipos de proyectos.  

---

## 🎯 Diferencia clave
- **Servicios comunes**: se despliegan desde el `sammcore-deployer`.  
- **SAMMCORE Deployer**: se despliega directamente desde GitHub Actions como servicio “fundacional” en `https://deployer.sammcore.local`.

---

## 🚀 Flujo general
1. Usuario registra un proyecto en el deployer.  
2. El deployer analiza el repo:  
   - HTML/JS estático.  
   - Dockerfile único.  
   - docker-compose multi-servicio.  
3. Selecciona plantilla o usa las definiciones del repo.  
4. Construye contenedores → los publica en GHCR.  
5. Genera y aplica manifests en K3s.  
6. Expone la app en `*.sammcore.local`.  

---

## ⚙️ Tecnologías
- **Frontend**: React + Vite  
- **Backend**: Go (`client-go` para K8s, git para repos, docker CLI para builds)  
- **Orquestación**: K3s (Kubernetes ligero en SAMMCORE)  
- **CI/CD**: GitHub Actions (solo para el propio deployer)  
- **Registro**: GitHub Container Registry (GHCR)  

---

## 📌 Estado actual
✔ Documentación lista  
✔ Plan de desarrollo definido  
⬜ Backend mínimo (Fase 1 – en progreso)  
⬜ Frontend inicial (Fase 3)  
⬜ Integración con K3s (Fase 4)  
⬜ CI/CD del deployer (Fase 5)

---
