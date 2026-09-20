package services

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

var validPostgresIdentifier = regexp.MustCompile(`^[a-z0-9_]{3,50}$`)

type DBProvisionResult struct {
	Host        string `json:"host"`
	Port        int    `json:"port"`
	Database    string `json:"database"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	DatabaseURL string `json:"database_url"`
}

func ToPostgresName(k8sProject string) string {
	clean := strings.ReplaceAll(k8sProject, "-", "_")
	return strings.ToLower(clean)
}

func GenerateSecurePassword(length int) string {
	if length < 16 {
		length = 32
	}
	bytes := make([]byte, length/2)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback_secret_pass_" + strconv.FormatInt(123456789, 10)
	}
	return hex.EncodeToString(bytes)
}

func GetSupabaseConfig() (host string, port int, user string, password string, dbname string) {
	host = os.Getenv("SUPABASE_HOST")
	if host == "" {
		host = "postgres.supabase.svc.cluster.local"
	}

	portStr := os.Getenv("SUPABASE_PORT")
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 {
		port = 5432
	}

	user = os.Getenv("SUPABASE_POSTGRES_USER")
	if user == "" {
		user = "postgres"
	}

	password = os.Getenv("SUPABASE_POSTGRES_PASSWORD")
	dbname = "postgres"
	return
}

func OpenMasterDB(host string, port int, user, password, dbname string) (*sql.DB, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable connect_timeout=5",
		host, port, user, password, dbname)
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("error al abrir conexión con PostgreSQL maestro: %w", err)
	}

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("error al hacer ping a PostgreSQL maestro (%s:%d): %w", host, port, err)
	}

	return db, nil
}

// ProvisionProjectDatabase aprovisiona de forma idempotente una base de datos y usuario dedicado (Modelo A)
func ProvisionProjectDatabase(masterDB *sql.DB, project, existingPassword string) (*DBProvisionResult, error) {
	pgBase := ToPostgresName(project)
	dbName := fmt.Sprintf("%s_db", pgBase)
	userName := fmt.Sprintf("%s_user", pgBase)

	if !validPostgresIdentifier.MatchString(dbName) || !validPostgresIdentifier.MatchString(userName) {
		return nil, fmt.Errorf("identificador PostgreSQL inválido (db: %s, user: %s)", dbName, userName)
	}

	// 1. Verificar existencia de rol y base de datos
	var roleExists bool
	err := masterDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", userName).Scan(&roleExists)
	if err != nil {
		return nil, fmt.Errorf("error al verificar existencia de rol %s: %w", userName, err)
	}

	var dbExists bool
	err = masterDB.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", dbName).Scan(&dbExists)
	if err != nil {
		return nil, fmt.Errorf("error al verificar existencia de base de datos %s: %w", dbName, err)
	}

	// 2. Matriz de idempotencia de 4 casos
	password := existingPassword
	if password == "" {
		password = GenerateSecurePassword(32)
	}

	if !roleExists {
		// Casos 1 y 4: Rol no existe -> crear con límite de conexiones
		query := fmt.Sprintf("CREATE USER %s WITH ENCRYPTED PASSWORD %s CONNECTION LIMIT 20",
			pq.QuoteIdentifier(userName), pq.QuoteLiteral(password))
		if _, err := masterDB.Exec(query); err != nil {
			return nil, fmt.Errorf("fallo al crear usuario %s: %w", userName, err)
		}
	} else if existingPassword == "" {
		// Caso 3: Rol existe pero secret se perdió -> sincronizar nueva clave
		query := fmt.Sprintf("ALTER USER %s WITH ENCRYPTED PASSWORD %s CONNECTION LIMIT 20",
			pq.QuoteIdentifier(userName), pq.QuoteLiteral(password))
		if _, err := masterDB.Exec(query); err != nil {
			return nil, fmt.Errorf("fallo al actualizar contraseña de usuario %s: %w", userName, err)
		}
	}
	// Caso 2: roleExists && existingPassword != "" -> mantener clave actual sin tocar Postgres

	// 3. Conceder rol a postgres para permitir asignación de ownership sin superusuario
	if _, err := masterDB.Exec(fmt.Sprintf("GRANT %s TO postgres", pq.QuoteIdentifier(userName))); err != nil {
		return nil, fmt.Errorf("fallo al conceder rol %s a postgres: %w", userName, err)
	}

	// 4. Crear base de datos si no existe
	if !dbExists {
		query := fmt.Sprintf("CREATE DATABASE %s OWNER %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(userName))
		if _, err := masterDB.Exec(query); err != nil {
			return nil, fmt.Errorf("fallo al crear base de datos %s: %w", dbName, err)
		}
	}

	// 5. Aislamiento estricto de permisos
	if _, err := masterDB.Exec(fmt.Sprintf("REVOKE ALL ON DATABASE %s FROM PUBLIC", pq.QuoteIdentifier(dbName))); err != nil {
		return nil, fmt.Errorf("fallo al revocar permisos públicos en %s: %w", dbName, err)
	}

	if _, err := masterDB.Exec(fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", pq.QuoteIdentifier(dbName), pq.QuoteIdentifier(userName))); err != nil {
		return nil, fmt.Errorf("fallo al otorgar privilegios al usuario %s: %w", userName, err)
	}

	// Revocar acceso explícito a la base administrativa postgres para este usuario
	if _, err := masterDB.Exec(fmt.Sprintf("REVOKE CONNECT ON DATABASE postgres FROM %s", pq.QuoteIdentifier(userName))); err != nil {
		// No fatal si el usuario no tenía permiso explícito previo
		_ = err
	}

	host, port, _, _, _ := GetSupabaseConfig()

	return &DBProvisionResult{
		Host:        host,
		Port:        port,
		Database:    dbName,
		Username:    userName,
		Password:    password,
		DatabaseURL: fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?sslmode=disable", userName, password, host, port, dbName),
	}, nil
}

// DeprovisionProjectDatabase ejecuta la secuencia atómica de purga si deleteDB es verdadero
func DeprovisionProjectDatabase(masterDB *sql.DB, project string, deleteDB bool) error {
	if !deleteDB {
		return nil
	}

	pgBase := ToPostgresName(project)
	dbName := fmt.Sprintf("%s_db", pgBase)
	userName := fmt.Sprintf("%s_user", pgBase)

	if !validPostgresIdentifier.MatchString(dbName) || !validPostgresIdentifier.MatchString(userName) {
		return fmt.Errorf("identificador PostgreSQL inválido para purga: %s", dbName)
	}

	// 1. Cerrar conexiones activas
	_, _ = masterDB.Exec("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND pid <> pg_backend_pid()", dbName)

	// 2. Eliminar base de datos
	if _, err := masterDB.Exec(fmt.Sprintf("DROP DATABASE IF EXISTS %s", pq.QuoteIdentifier(dbName))); err != nil {
		return fmt.Errorf("fallo al eliminar base de datos %s: %w", dbName, err)
	}

	// 3. Revocar membresía concedida a postgres
	_, _ = masterDB.Exec(fmt.Sprintf("REVOKE %s FROM postgres", pq.QuoteIdentifier(userName)))

	// 4. Eliminar usuario
	if _, err := masterDB.Exec(fmt.Sprintf("DROP USER IF EXISTS %s", pq.QuoteIdentifier(userName))); err != nil {
		return fmt.Errorf("fallo al eliminar usuario %s: %w", userName, err)
	}

	return nil
}
