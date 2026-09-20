# 📄 Módulo de Base de Datos Horizontal — Modelo A (Supabase)

> **Servicio de Base de Datos:** Supabase PostgreSQL Clúster (`namespace: supabase`)  
> **Host Interno:** `postgres.supabase.svc.cluster.local:5432`  
> **Panel Visual:** `https://supabase.sammcore.local` (Supabase Studio)  
> **Monitoreo:** Prometheus TSDB + Grafana (`postgres-exporter`)

---

## 1. 🎯 Propósito y Filosofía del Modelo A

En lugar de que cada proyecto desplegado por `sammcore-deployer` levante su propio contenedor de PostgreSQL aislado (desperdiciando memoria RAM y fragmentando la administración), el ecosistema opera bajo el **Modelo A**:

* **Una única plataforma horizontal de base de datos:** El motor PostgreSQL 15 de Supabase corre en el namespace `supabase` con almacenamiento persistente NVMe (`local-path-provisioner`).
* **Aislamiento estricto por proyecto:** Cada proyecto registrado en el deployer recibe su propia base de datos lógica y su propio usuario restringido.
* **Gobierno y visibilidad centralizada:** Todas las bases creadas son visibles inmediatamente en **Supabase Studio** para inspeccionar tablas, relaciones, roles y ejecutar SQL en vivo, además de ser monitoreadas automáticamente por Grafana.

---

## 2. 🔐 Política de Seguridad y Aislamiento

Para garantizar que un proyecto no pueda ver ni modificar los datos de otro:

```mermaid
flowchart TD
    subgraph SupabaseCluster["🐘 PostgreSQL 15 (namespace: supabase)"]
        subgraph DB1["🗄️ backroom_db"]
            Tables1["Tablas: products, orders, suppliers"]
        end
        subgraph DB2["🗄️ tienda_db"]
            Tables2["Tablas: users, catalog, carts"]
        end
    end

    subgraph Projects["☸️ Cargas de Trabajo (Namespaces k8s)"]
        App1["🐹 backroom-backend\n(user: backroom_user)"]
        App2["🚀 tienda-backend\n(user: tienda_user)"]
    end

    App1 -->|Acceso EXCLUSIVO| DB1
    App1 -.->|DENEGADO / Bloqueado| DB2

    App2 -->|Acceso EXCLUSIVO| DB2
    App2 -.->|DENEGADO / Bloqueado| DB1
```

### Reglas de Aislamiento:
1. **Nombre de Base de Datos:** `<nombre-proyecto>_db` (en minúsculas y caracteres alfanuméricos seguros).
2. **Nombre de Rol/Usuario:** `<nombre-proyecto>_user`.
3. **Contraseña:** Generada automáticamente por el backend del deployer con alta entropía (32 caracteres criptográficos aleatorios).
4. **Permisos:**
   ```sql
   -- Aislamiento de base
   REVOKE ALL ON DATABASE <proyecto>_db FROM PUBLIC;
   GRANT ALL PRIVILEGES ON DATABASE <proyecto>_db TO <proyecto>_user;
   ALTER DATABASE <proyecto>_db OWNER TO <proyecto>_user;
   ```
5. **Acceso Cruzado Bloqueado:** `<proyecto>_user` no posee permisos de conexión (`CONNECT`) a ninguna otra base del clúster ni al rol superusuario `postgres`.

---

## 3. 🧩 Componente `DatabaseManager` en el Backend de Go

El módulo `DatabaseManager` dentro de `backend/services/db_manager.go` es responsable de:

1. **Detección Automática:**
   * Durante `POST /analyzeRepo`, si se detecta un servicio de base de datos en `docker-compose.yml` (ej. imagen `postgres`), o variables de entorno relativas a base de datos (`DB_HOST`, `DATABASE_URL`), el deployer marca el proyecto como `requires_database: true`.
   * En la interfaz web, el usuario también puede marcar o desmarcar la casilla *"Requiere Base de Datos en Supabase"*.

2. **Aprovisionamiento Idempotente:**
   * Conexión administrativa interna usando el driver nativo de Go (`pgx` o `database/sql` + `lib/pq`):
     ```text
     postgres://postgres:${MASTER_PASSWORD}@postgres.supabase.svc.cluster.local:5432/postgres?sslmode=disable
     ```
   * Ejecución segura:
     ```sql
     -- Crear rol si no existe
     DO $$
     BEGIN
       IF NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '<user>') THEN
         CREATE USER <user> WITH ENCRYPTED PASSWORD '<password>';
       END IF;
     END
     $$;

     -- Crear base de datos si no existe
     SELECT 'CREATE DATABASE <db>'
     WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '<db>')\gexec
     ```

3. **Generación del Objeto de Conexión:**
   El `DatabaseManager` retorna una estructura en memoria con:
   ```go
   type DBProvisionResult struct {
       Host        string `json:"host"`        // postgres.supabase.svc.cluster.local
       Port        int    `json:"port"`        // 5432
       Database    string `json:"database"`    // <proyecto>_db
       Username    string `json:"username"`    // <proyecto>_user
       Password    string `json:"password"`    // string generado
       DatabaseURL string `json:"database_url"`// postgresql://...
   }
   ```

4. **Entrega a `SecretManager`:**
   El resultado **nunca se escribe en disco ni se envía a Git**. Se transfiere en memoria directamente a `SecretManager`, que crea el recurso `Secret` en Kubernetes:
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: <proyecto>-db-secrets
     namespace: <proyecto>
   type: Opaque
   stringData:
     DB_HOST: "postgres.supabase.svc.cluster.local"
     DB_PORT: "5432"
     DB_NAME: "<proyecto>_db"
     DB_USER: "<proyecto>_user"
     DB_PASSWORD: "<password_generado>"
     DATABASE_URL: "postgresql://<proyecto>_user:<password>@postgres.supabase.svc.cluster.local:5432/<proyecto>_db?sslmode=disable"
   ```

---

## 4. 📊 Integración con Monitoreo y Administración Visual

Una vez aprovisionada la base:

1. **Supabase Studio (`https://supabase.sammcore.local`):**
   * El administrador abre la consola web, selecciona la base `<proyecto>_db` en el menú desplegable.
   * Cuenta con:
     * **Table Editor:** Vista tipo hoja de cálculo para editar filas y columnas.
     * **SQL Editor:** Ejecución de queries manuales, `EXPLAIN ANALYZE`, creación de vistas.
     * **Database Settings:** Estadísticas de tamaño en disco, índices y extensiones activas (`pgvector`, `uuid-ossp`, etc.).
2. **Prometheus + Grafana:**
   * El pod `postgres-exporter` en `namespace: supabase` ya tiene un `ServiceMonitor` activo.
   * Recolecta métricas automáticas por base de datos:
     * `pg_stat_database_xact_commit{datname="<proyecto>_db"}`
     * `pg_stat_database_xact_rollback{datname="<proyecto>_db"}`
     * `pg_stat_database_numbackends{datname="<proyecto>_db"}` (conexiones concurrentes).

---

## 5. 🗑️ Ciclo de Vida y Desmantelamiento

Cuando un proyecto es eliminado desde la interfaz de `sammcore-deployer` (`DELETE /project/:id`):

1. El deployer consulta al usuario mediante confirmación:
   * *¿Desea conservar la base de datos para respaldo, o eliminarla permanentemente?*
2. Si se selecciona eliminación permanente:
   ```sql
   DROP DATABASE IF EXISTS <proyecto>_db;
   DROP USER IF EXISTS <proyecto>_user;
   ```
3. Si se selecciona conservar: La base permanece intacta en Supabase Studio y solo se eliminan los pods, services y secrets del namespace en Kubernetes.
