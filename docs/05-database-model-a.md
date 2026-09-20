# 📄 Plataforma de Base de Datos Horizontal — Modelo A (Supabase)

> **Motor Central:** Supabase PostgreSQL 15 (`namespace: supabase`)  
> **Host Interno:** `postgres.supabase.svc.cluster.local:5432`  
> **Credencial Maestra:** Inyectada desde el Secret de K8s `supabase-postgres-secret` como variable `SUPABASE_POSTGRES_PASSWORD`.  
> **Monitoreo:** Prometheus TSDB + Grafana (`postgres-exporter`).

---

## 1. ⚖️ Comparativa de Arquitectura: Modelo A vs Modelo B

Para evitar ambigüedades técnicas, se definen los dos modelos de datos posibles en PostgreSQL:

| Criterio | Modelo A (Bases de Datos Dedicadas) ⭐ **Elegido** | Modelo B (Esquemas Lógicos en `postgres`) |
| :--- | :--- | :--- |
| **Aislamiento** | **Total a nivel de motor:** Cada proyecto tiene su propia base de datos (`<proyecto>_db`) y usuario (`<proyecto>_user`). No hay riesgo de cruce de datos accidental. | **Lógico:** Todos los proyectos comparten la base `postgres` y solo se separan por `SCHEMA <proyecto>`. |
| **Operación y Respaldos** | Independiente: `pg_dump -d <proyecto>_db` respalda o restaura un único proyecto sin tocar los demás. `VACUUM` y tuning por base. | Complejo: `pg_dump` debe filtrar esquemas específicos; un bloqueo o consumo alto afecta a toda la base. |
| **Herramientas de Gestión** | Administrable vía `psql`, clientes estándar (DBeaver, DataGrip) y herramientas multi-base como **Adminer**. | Directamente visible en el selector de esquemas del **Supabase Studio** self-hosted. |
| **Limitación Técnica de Supabase Studio** | **Importante:** El Supabase Studio oficial self-hosted conecta a una única base de datos configurada en sus variables de entorno (`postgres`) y su selector visual intercambia entre **esquemas**, no entre bases físicas. Por ende, **el criterio de aceptación del Modelo A no es Supabase Studio**, sino la verificación vía cliente SQL y métricas de Grafana. | Permite ver tablas en Supabase Studio, pero sacrifica aislamiento físico y robustez de respaldos. |

**Decisión:** SAMMCORE adopta formalmente el **Modelo A** por seguridad, aislamiento y estabilidad operativa.

---

## 2. 🔐 Reglas de Aislamiento y Nomenclatura

1. **Nombre de Base de Datos:** `<proyecto>_db` (minúsculas, caracteres `[a-z0-9_]`).
2. **Nombre de Rol/Usuario:** `<proyecto>_user`.
3. **Contraseña:** Generada criptográficamente en memoria (32 caracteres alfanuméricos seguros).
4. **Política de Acceso Exclusivo:**
   ```sql
   REVOKE ALL ON DATABASE "<proyecto>_db" FROM PUBLIC;
   GRANT ALL PRIVILEGES ON DATABASE "<proyecto>_db" TO "<proyecto>_user";
   ALTER DATABASE "<proyecto>_db" OWNER TO "<proyecto>_user";
   ```
5. `<proyecto>_user` no posee privilegios `SUPERUSER`, no puede conectarse a la base `postgres` ni ver metadatos de otros proyectos.

---

## 3. 🧩 Implementación en Go (`DatabaseManager`)

En Go, `CREATE DATABASE` no puede ejecutarse dentro de un bloque de transacción ni dentro de un bloque `DO $$`. Por ello, `DatabaseManager` implementa consultas idempotentes estructuradas y sanitizadas:

```go
package services

import (
	"database/sql"
	"fmt"
	"regexp"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

var validIdentifier = regexp.MustCompile(`^[a-z0-9_]{3,40}$`)

type DBProvisionResult struct {
	Host        string
	Port        int
	Database    string
	Username    string
	Password    string
	DatabaseURL string
}

func ProvisionProjectDatabase(masterDB *sql.DB, project, existingPassword string) (*DBProvisionResult, error) {
	dbName := fmt.Sprintf("%s_db", project)
	userName := fmt.Sprintf("%s_user", project)

	if !validIdentifier.MatchString(dbName) || !validIdentifier.MatchString(userName) {
		return nil, fmt.Errorf("identificador inválido para base de datos o usuario")
	}

	// 1. Idempotencia de Contraseña: Si ya existía un Secret en K8s, reutilizar la contraseña
	password := existingPassword
	if password == "" {
		password = generateSecurePassword(32)
	}

	// 2. Crear o actualizar rol de forma segura
	var roleExists bool
	err := masterDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", userName).Scan(&roleExists)
	if err != nil {
		return nil, err
	}

	if !roleExists {
		_, err = masterDB.Exec(fmt.Sprintf("CREATE USER %s WITH ENCRYPTED PASSWORD %s", pq.QuoteIdentifier(userName), pq.QuoteLiteral(password)))
		if err != nil {
			return nil, err
		}
	} else if existingPassword == "" {
		// Si es nueva contraseña forzada
		_, err = masterDB.Exec(fmt.Sprintf("ALTER USER %s WITH ENCRYPTED PASSWORD %s", pq.QuoteIdentifier(userName), pq.QuoteLiteral(password)))
		if err != nil {
			return nil, err
		}
	}

	// 3. Crear base de datos si no existe
	var dbExists bool
	err = masterDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&dbExists)
	if err != nil {
		return nil, err
	}

	if !dbExists {
		_, err = masterDB.Exec(fmt.Sprintf("CREATE DATABASE %s OWNER %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(userName)))
		if err != nil {
			return nil, err
		}
	}

	// 4. Asegurar privilegios estrictos
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

## 4. 🔄 Ciclo de Vida y Re-despliegues

* **Re-despliegue (`redeploy`):**
  - El deployer consulta primero el objeto `Secret` `<proyecto>-db-secrets` en Kubernetes.
  - Si el Secret existe, extrae `DB_PASSWORD` y se la pasa a `ProvisionProjectDatabase`.
  - Esto garantiza que **la contraseña nunca se desfase** entre el rol de PostgreSQL y los pods activos.
* **Eliminación (`DELETE /api/projects/:id?delete_db=true|false`):**
  - Si `delete_db=false` (predeterminado): Solo se eliminan el Namespace, Deployments, Services y Secrets de K8s. La base de datos y sus datos persisten intactos en PostgreSQL.
  - Si `delete_db=true`: Se revoca la conexión y se ejecutan:
    ```sql
    SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = '<proyecto>_db';
    DROP DATABASE IF EXISTS "<proyecto>_db";
    DROP USER IF EXISTS "<proyecto>_user";
    ```
