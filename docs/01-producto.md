# 📄 Documento de Producto – SAMMCORE Deployer

## 1. 🎯 Propósito
El **SAMMCORE Deployer** es una aplicación web que permite a los usuarios **desplegar proyectos en SAMMCORE sin conocimientos avanzados de Kubernetes**.  
Su función principal es ser el **punto central de entrada** para gestionar la publicación de aplicaciones desde repositorios GitHub hacia el clúster de K3s de SAMMCORE.

---

## 2. 👤 Usuarios objetivo
- **Desarrolladores**: quieren desplegar sus proyectos sin preocuparse por los detalles de K8s.  
- **Administradores de SAMMCORE**: buscan mantener ordenados los servicios y controlar credenciales.  
- **Estudiantes / investigadores**: aprenden sobre despliegue modular en un entorno controlado.

---

## 3. 🚀 Alcance
El deployer permitirá:
1. Registrar proyectos desde un repositorio (GitHub/GitLab).  
2. Detectar la estructura del proyecto:  
   - Estático (HTML/JS).  
   - Dockerfile único.  
   - docker-compose (multi-servicio).  
3. Ofrecer plantillas de despliegue si el repo no las tiene.  
4. Manejar credenciales y contraseñas de forma segura (K8s Secrets).  
5. Construir imágenes Docker y publicarlas en un registro (GHCR o local).  
6. Desplegar automáticamente en K3s con namespace, Service e Ingress.  
7. Mostrar estado, logs y accesos desde un panel web.

---

## 4. 📊 Casos de uso
- **Caso 1**: Usuario sube un proyecto HTML simple → se empaqueta en un contenedor Nginx y se publica como `juego.sammcore.local`.  
- **Caso 2**: Proyecto con `Dockerfile` → el deployer construye la imagen y lo expone como microservicio en `api.sammcore.local`.  
- **Caso 3**: Proyecto PHP con `docker-compose` → se traduce a varios Deployments (Apache, MySQL, SFTP), con Secrets para contraseñas, y se publica como `php.sammcore.local`.

---

## 5. 🧩 Diferencia con otros servicios
- **Portainer**: administra contenedores ya desplegados.  
- **SAMMCORE Deployer**: crea los despliegues desde cero a partir de repositorios.  

👉 **Es el bootstrapper de proyectos en SAMMCORE.**
