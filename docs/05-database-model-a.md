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

## 2. 🔐 Matriz de Idempotencia de 4 Casos

Para garantizar que un re-despliegue nunca desincronice las contraseñas entre PostgreSQL y los pods de Kubernetes, el `DatabaseManager` evalúa los 4 estados posibles:

| Caso | ¿Existe Rol en Postgres? | ¿Existe Secret en K8s? | Acción a Ejecutar |
| :--- | :---: | :---: | :--- |
| **1. Despliegue Inicial** | ❌ No | ❌ No | Generar contraseña criptográfica en memoria $\rightarrow$ `CREATE USER ... WITH ENCRYPTED PASSWORD` $\rightarrow$ Crear objeto `Secret` en K8s. |
| **2. Re-despliegue Normal** | ✅ Sí | ✅ Sí | **Reutilizar contraseña del Secret existente** $\rightarrow$ No se altera la contraseña en Postgres $\rightarrow$ Pod y BD continúan sincronizados. |
| **3. Rol Huérfano (Secret borrado)** | ✅ Sí | ❌ No | Generar contraseña nueva $\rightarrow$ `ALTER USER ... WITH ENCRYPTED PASSWORD` $\rightarrow$ Recrear objeto `Secret` en K8s con la nueva clave. |
| **4. BD recreada (Secret conservado)** | ❌ No | ✅ Sí | Extraer contraseña del Secret existente $\rightarrow$ `CREATE USER ... WITH ENCRYPTED PASSWORD '<secret_pass>'` $\rightarrow$ Pod reconecta sin cambios. |

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

	// 1. Verificar existencia de rol y base de datos
	var roleExists bool
	if err := masterDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", userName).Scan(&roleExists); err != nil {
		return nil, fmt.Errorf("error al verificar rol: %w", err)
	}

	var dbExists bool
	if err := masterDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&dbExists); err != nil {
		return nil, fmt.Errorf("error al verificar base de datos: %w", err)
	}

	// 2. Aplicar matriz de 4 casos para el rol y la contraseña
	password := existingPassword
	if password == "" {
		password = generateSecurePassword(32)
	}

	if !roleExists {
		// Casos 1 y 4: Crear rol con límite de conexiones
		query := fmt.Sprintf("CREATE USER %s WITH ENCRYPTED PASSWORD %s CONNECTION LIMIT 20", pq.QuoteIdentifier(userName), pq.QuoteLiteral(password))
		if _, err := masterDB.Exec(query); err != nil {
			return nil, fmt.Errorf("fallo al crear usuario: %w", err)
		}
	} else if existingPassword == "" {
		// Caso 3: Rol existía pero secret se perdió; forzar sincronización con nueva clave
		query := fmt.Sprintf("ALTER USER %s WITH ENCRYPTED PASSWORD %s", pq.QuoteIdentifier(userName), pq.QuoteLiteral(password))
		if _, err := masterDB.Exec(query); err != nil {
			return nil, fmt.Errorf("fallo al actualizar contraseña de usuario: %w", err)
		}
	}
	// Caso 2: roleExists && existingPassword != "": No se toca la contraseña

	// 3. Conceder rol a postgres para permitir asignación de ownership sin superusuario
	if _, err := masterDB.Exec(fmt.Sprintf("GRANT %s TO postgres", pq.QuoteIdentifier(userName))); err != nil {
		return nil, fmt.Errorf("fallo al conceder rol a postgres: %w", err)
	}

	// 4. Crear base de datos si no existe
	if !dbExists {
		query := fmt.Sprintf("CREATE DATABASE %s OWNER %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(userName))
		if _, err := masterDB.Exec(query); err != nil {
			return nil, fmt.Errorf("fallo al crear base de datos: %w", err)
		}
	}

	// 5. Aislamiento estricto de permisos (verificando errores)
	if _, err := masterDB.Exec(fmt.Sprintf("REVOKE ALL ON DATABASE %s FROM PUBLIC", pq.QuoteIdentifier(dbName))); err != nil {
		return nil, fmt.Errorf("fallo al revocar permisos públicos: %w", err)
	}

	if _, err := masterDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(userName))); err != nil {
		return nil, fmt.Errorf("fallo al otorgar privilegios al usuario: %w", err)
	}

	// Revocar explícitamente acceso a la base administrativa postgres
	if _, err := masterDB.Exec(fmt.Sprintf("REVOKE CONNECT ON DATABASE postgres FROM %s", pq.QuoteIdentifier(userName))); err != nil {
		// En Postgres puede requerir advertencia sin romper si el usuario no tenía permiso explícito
		_ = err
	}

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

## 4. 🧹 Política de Respaldos y Bases Huérfanas

* **Eliminación con `delete_db=false`:**  
  Deja la base intacta para fines de auditoría o migración.
* **Inventario y Purga de Huérfanas:**  
  El deployer registra el catálogo de proyectos activos en su store. Durante el arranque o mediante una tarea cron semanal, compara las bases de datos en PostgreSQL que coincidan con `*_db` contra los proyectos activos; las bases no asociadas a ningún proyecto se marcan como `huérfanas` en el log administrativo para purga manual o respaldo vía `pg_dump`.
