# 📄 Arquitectura Fase 4 — SAMMCORE-Deployer

## 1. Rol del Deployer

* **Orquestador central de despliegues.**
  Se encarga de:

  1. Detectar cambios en GitHub (push, release, manual trigger).
  2. Construir la imagen del proyecto (Docker o Compose).
  3. Generar manifiestos Kubernetes (`Deployment`, `Service`, `Ingress`).
  4. Aplicarlos en el clúster **K3s de SAMMCORE**.
  5. Registrar el histórico de despliegues y exponer métricas.

* Corre **como servicio independiente en el host SAMMCORE**, **no dentro del clúster K3s**, lo que permite:

  * Mantener control incluso si el clúster se degrada.
  * Integrarse directamente con Docker local y Portainer.
  * Exportar métricas para Grafana/Prometheus.

---

## 2. Relación con el ecosistema SAMMCORE

* **Portainer:**
  Visualiza los contenedores y despliegues gestionados por el Deployer.
* **Grafana (con Prometheus):**
  Consume métricas del Deployer (`/metrics`) y del clúster K3s.
* **Kubernetes Dashboard:**
  Muestra el detalle técnico de Deployments/Pods/Ingress generados.
* **NGINX + DNS Local:**
  Expone los proyectos desplegados bajo `*.sammcore.local` con certificados TLS.

---

## 3. Diagrama de arquitectura

```mermaid
flowchart LR
    subgraph GH[GitHub]
      R1[Repositorios\nProyectos]
    end

    subgraph SC[SAMMCORE Host]
      D1[SAMMCORE-Deployer\n(Servicio independiente)]
      P1[Portainer]
      G1[Grafana]
      N1[NGINX\nProxy HTTPS]
    end

    subgraph K3s[K3s Cluster]
      A1[API Server]
      K1[Deployments]
      K2[Services]
      K3[Ingress]
      K4[Pods]
      KD[Kubernetes Dashboard]
    end

    R1 -->|push/release| D1
    D1 -->|kubectl/client-go| A1
    A1 --> K1 & K2 & K3 & K4
    KD --> A1

    D1 -->|estado contenedores| P1
    D1 -->|/metrics Prometheus| G1
    G1 -->|dashboards| D1

    K3s --> N1 -->|DNS *.sammcore.local| Users[Usuarios LAN]
```

---

## 4. Flujo de trabajo

1. **Evento en GitHub:** nuevo código o release.
2. **SAMMCORE-Deployer:**

   * Construye imagen → publica en registro (o local).
   * Genera manifiestos según tipo de proyecto.
   * Aplica los manifiestos en K3s.
   * Registra acción en `history.json`.
   * Expone métricas de éxito/error.
3. **K3s:** despliega recursos (Pods, Services, Ingress).
4. **NGINX:** enruta tráfico interno con TLS + DNS local.
5. **Portainer:** refleja estado de contenedores/servicios.
6. **Grafana:** muestra métricas y alertas de despliegue.
7. **Usuarios LAN:** acceden a apps con `https://proyecto.sammcore.local`.

---

## 5. Integraciones clave

* **Prometheus exporter en el Deployer:**
  Endpoint `/metrics` → número de despliegues, errores, estado de pods, duración de build/deploy.
* **Portainer API:**
  Opcional para correlacionar contenedores locales con despliegues en K3s.
* **K3s API (client-go):**
  Para aplicar manifiestos y consultar estado real.
* **NGINX Ingress:**
  Publicación final con certificados autofirmados instalados en la red local.

---

📌 Con este documento dejamos clara la **posición del Deployer en la arquitectura SAMMCORE**: no es un “servicio oculto” del clúster, sino una **pieza lateral y estratégica**, integrándose de frente con Grafana, Portainer y Dashboard.

