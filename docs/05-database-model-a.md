# 📄 Plataforma de Base de Datos Horizontal — Modelo A (Supabase)

> **Motor Central:** Supabase PostgreSQL 15 (`namespace: supabase`)  
> **Host Interno:** `postgres.supabase.svc.cluster.local:5432`  
> **Credencial Maestra:** Inyectada en el pod deployer desde el Secret `deployer-secrets` como variable `SUPABASE_POSTGRES_PASSWORD`.  
> **Monitoreo:** Prometheus TSDB + Grafana (`postgres-exporter`).

---

## 1. ⚖️ Comparativa de Arquitectura: Modelo A vs Modelo B

| Criterio | Modelo A (Bases de Datos Dedicadas) ⭐ **Elegido** | Modelo B (Esquemas Lógicos en `postgres`) |
| :--- | :--- | :--- |
| **Aislamiento** | **Total a nivel de motor:** Cada proyecto tiene su propia base de datos (`<proyecto>_db`) y usuario (`<proyecto>_user`). No hay riesgo de cruce de datos accidental. | **Lógico:** Todos los proyectos comparten la base `postgres` y solo se separan por `SCHEMA <proyecto>`. |
| **Operación y Respaldos** | Independiente: `pg_dump -d <proyecto>_db` respalda o restaura un único proyecto sin tocar los demás. `VACUUM` y tuning por base. | Complejo: `pg_dump` debe filtrar esquemas específicos; un bloqueo o consumo alto afecta a toda la base. |
| **Herramientas de Gestión** | Administrable vía `psql`, clientes estándar (DBeaver, DataGrip) y herramientas multi-base como **Adminer**. | Directamente visible en el selector de esquemas del **Supabase Studio** self-hosted. |
| **Limitación Técnica de Supabase Studio** | **Importante:** El Supabase Studio oficial self-hosted conecta a una única base de datos configurada en sus variables de entorno (`postgres`) y su selector visual intercambia entre **esquemas**, no entre bases físicas. Por ende, **el criterio de aceptación del Modelo A no es Supabase Studio**, sino la verificación vía cliente SQL y métricas de Grafana. | Permite ver tablas en Supabase Studio, pero sacrifica aislamiento físico y robustez de respaldos. |

---

## 2. 🔐 Reglas de Aislamiento y Mapeo de Nombres

1. **Mapeo de Nombre de Proyecto a PostgreSQL:**
   - Si el proyecto en K8s se llama `mi-tienda` (RFC 1123 con guiones), en PostgreSQL se sustituye `-` por `_`:
     - Base de Datos: `mi_tienda_db`
     - Usuario/Rol: `mi_tienda_user`
   - Validación regex en Go: `^[a-z0-9_]{3,50}$`.
2. **Generación de Contraseñas:**
   - Nueva provisión: Cadena criptográfica de 32 caracteres alfanuméricos en memoria.
   - Re-despliegue (`redeploy`): Si el Secret `<proyecto>-db-secrets` ya existe en K8s, **se recupera y reutiliza la contraseña existente**, garantizando idempotencia absoluta y evitando desfasar el pod con la base.
3. **Manejo de Permisos y Propiedad (Solución a Postgres sin Superuser):**
   - En la imagen de Supabase, el rol `postgres` puede no tener el atributo `SUPERUSER`.
   - En PostgreSQL, para ejecutar `CREATE DATABASE ... OWNER "<user>"`, el creador debe tener rol superusuario o ser miembro del rol `<user>`.
   - Por tanto, la secuencia segura de Go ejecuta:
     ```sql
     -- 1. Crear rol
     CREATE USER "<user>" WITH ENCRYPTED PASSWORD '...';
     -- 2. Conceder rol al operador administrativo para poder asignar ownership
     GRANT "<user>" TO postgres;
     -- 3. Crear base con ownership asignado
     CREATE DATABASE "<db>" OWNER "<user>";
     -- 4. Aislar base
     REVOKE ALL ON DATABASE "<db>" FROM PUBLIC;
     GRANT ALL PRIVILEGES ON DATABASE "<db>" TO "<user>";
     ```

---

## 3. 🧩 Implementación en Go (`DatabaseManager`)

```go
package services

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

var validPostgresIdentifier = regexp.MustCompile(`^[a-z0-9_]{3,50}$`)

type DBProvisionResult struct {
	Host        string
	Port        int
	Database    string
	Username    string
	Password    string
	DatabaseURL string
}

func ToPostgresName(k8sProject string) string {
	clean := strings.ReplaceAll(k8sProject, "-", "_")
	return strings.ToLower(clean)
}

func ProvisionProjectDatabase(masterDB *sql.DB, project, existingPassword string) (*DBProvisionResult, error) {
	pgBase := ToPostgresName(project)
	dbName := fmt.Sprintf("%s_db", pgBase)
	userName := fmt.Sprintf("%s_user", pgBase)

	if !validPostgresIdentifier.MatchString(dbName) || !validPostgresIdentifier.MatchString(userName) {
		return nil, fmt.Errorf("identificador PostgreSQL inválido: %s", dbName)
	}

	// 1. Idempotencia: reutilizar contraseña del Secret si ya existía
	password := existingPassword
	if password == "" {
		password = generateSecurePassword(32)
	}

	// 2. Crear rol si no existe
	var roleExists bool
	err := masterDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", userName).Scan(&roleExists)
	if err != nil {
		return nil, fmt.Errorf("error al verificar rol: %w", err)
	}

	if !roleExists {
		query := fmt.Sprintf("CREATE USER %s WITH ENCRYPTED PASSWORD %s", pq.QuoteIdentifier(userName), pq.QuoteLiteral(password))
		if _, err := masterDB.Exec(query); err != nil {
			return nil, fmt.Errorf("error al crear usuario: %w", err)
		}
	}

	// 3. Conceder rol a postgres para permitir asignación de ownership sin requerir superuser
	_, _ = masterDB.Exec(fmt.Sprintf("GRANT %s TO postgres", pq.QuoteIdentifier(userName)))

	// 4. Crear base de datos si no existe
	var dbExists bool
	err = masterDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&dbExists)
	if err != nil {
		return nil, fmt.Errorf("error al verificar base de datos: %w", err)
	}

	if !dbExists {
		query := fmt.Sprintf("CREATE DATABASE %s OWNER %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(userName))
		if _, err := masterDB.Exec(query); err != nil {
			return nil, fmt.Errorf("error al crear base de datos: %w", err)
		}
	}

	// 5. Permisos de aislamiento estricto
	_, _ = masterDB.Exec(fmt.Sprintf("REVOKE ALL ON DATABASE %s FROM PUBLIC", pq.QuoteIdentifier(dbName)))
	_, _ = masterDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(userName)))

	return &DBProvisionResult{
		Host:        "postgres.supabase.svc.cluster.local",
		Port:        5432,
		Database:    dbName,
		Username:    userName,
		Password:    password,
		DatabaseURL: fmt.Sprintf("postgresql://%s:%s@postgres.supabase.svc.cluster.local:5432/%s?sslmode=disable", userName, password, dbName),
	}, nil
}
```

---

## 4. 🔄 Ciclo de Vida y Desmantelamiento

* **Re-despliegue:** Reutiliza `DB_PASSWORD` del Secret existente. No altera contraseñas ni permisos en Postgres.
* **Eliminación (`DELETE /api/projects/:id?delete_db=true|false`):**
  - Predeterminado (`delete_db=false`): La base persiste intacta en Supabase.
  - Con borrado (`delete_db=true`):
    ```sql
    SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = 'mi_proyecto_db';
    DROP DATABASE IF EXISTS "mi_proyecto_db";
    DROP USER IF EXISTS "mi_proyecto_user";
    ```
