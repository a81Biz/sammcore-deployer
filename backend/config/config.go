package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config centraliza toda la configuración del deployer, cargada desde variables de entorno con defaults.
// Ningún otro archivo del proyecto debe hardcodear dominios, namespaces ni imágenes.
type Config struct {
	// Dominio base del clúster (ej. sammcore.local)
	BaseDomain string

	// Namespace donde corren los Jobs de Kaniko
	BuildsNamespace string

	// URL interna del registry Docker (sin esquema)
	RegistryURL string

	// Namespace del registry en el clúster
	RegistryNamespace string

	// Host del PostgreSQL/Supabase accesible desde el deployer (gestión de BD)
	DBAdminHost string
	// Puerto del PostgreSQL
	DBAdminPort string
	// Usuario administrador de PostgreSQL
	DBAdminUser string
	// Base de datos admin de PostgreSQL
	DBAdminName string

	// Host que los pods de las apps usan para conectarse a la BD
	// (puede ser un Service interno de K8s distinto del host admin)
	DBAppHost string

	// Clase de Ingress a usar (ej. nginx)
	IngressClass string

	// Namespace donde vive el IngressController
	IngressNamespace string

	// Imagen del initContainer git-clone (con versión fija, no 'latest')
	GitImage string

	// Imagen del ejecutor Kaniko (con versión fija)
	KanikoImage string

	// Imagen auxiliar de busybox
	BusyboxImage string

	// Límite de memoria para el contenedor Kaniko
	KanikoMemoryLimit string

	// Timeout para el build de Kaniko
	BuildTimeout time.Duration

	// Timeout para monitorear el rollout de pods
	RolloutTimeout time.Duration

	// CORS: orígenes permitidos (separados por coma)
	AllowedOrigins string

	// Cuotas de recursos por proyecto (Bolsa compartida del namespace)
	QuotaRequestCPU    string
	QuotaRequestMemory string
	QuotaLimitCPU      string
	QuotaLimitMemory   string
	QuotaMaxPods       string
}

// Nombres de namespaces reservados por el sistema. Ningún proyecto puede usar estos nombres.
var ReservedNamespaces = map[string]bool{
	"deployer":          true,
	"supabase":          true,
	"monitoring":        true,
	"ingress-nginx":     true,
	"default":           true,
	"api":               true,
	"docs":              true,
	"admin":             true,
	"grafana":           true,
	"prometheus":        true,
	"traefik":           true,
	"portainer":         true,
	"sammcore-registry": true,
	"deployer-builds":   true,
	"kube-system":       true,
	"kube-public":       true,
	"kube-node-lease":   true,
}

// IsReservedNamespace devuelve true si el nombre está reservado o empieza con 'kube-'
func IsReservedNamespace(name string) bool {
	if len(name) > 5 && name[:5] == "kube-" {
		return true
	}
	return ReservedNamespaces[name]
}

// Load lee las variables de entorno y retorna una Config con los valores correspondientes.
// Todos los campos tienen defaults razonables para producción.
func Load() *Config {
	buildTimeoutSec := getEnvInt("BUILD_TIMEOUT_SEC", 900)     // 15 minutos
	rolloutTimeoutSec := getEnvInt("ROLLOUT_TIMEOUT_SEC", 300) // 5 minutos

	return &Config{
		BaseDomain:        getEnv("BASE_DOMAIN", "sammcore.local"),
		BuildsNamespace:   getEnv("BUILDS_NAMESPACE", "deployer-builds"),
		RegistryURL:       getEnv("REGISTRY_URL", "registry.sammcore-registry.svc.cluster.local:5000"),
		RegistryNamespace: getEnv("REGISTRY_NAMESPACE", "sammcore-registry"),
		DBAdminHost:       getEnv("SUPABASE_HOST", "192.168.68.107"),
		DBAdminPort:       getEnv("SUPABASE_PORT", "5432"),
		DBAdminUser:       getEnv("SUPABASE_USER", "postgres"),
		DBAdminName:       getEnv("SUPABASE_DB", "postgres"),
		DBAppHost:         getEnv("DB_APP_HOST", "postgres.supabase.svc.cluster.local"),
		IngressClass:      getEnv("INGRESS_CLASS", "nginx"),
		IngressNamespace:  getEnv("INGRESS_NAMESPACE", "ingress-nginx"),
		GitImage:          getEnv("GIT_IMAGE", "alpine/git:2.43.0"),
		KanikoImage:       getEnv("KANIKO_IMAGE", "gcr.io/kaniko-project/executor:v1.23.2"),
		BusyboxImage:      getEnv("BUSYBOX_IMAGE", "busybox:1.36"),
		KanikoMemoryLimit: getEnv("KANIKO_MEMORY_LIMIT", "7500Mi"),
		BuildTimeout:      time.Duration(buildTimeoutSec) * time.Second,
		RolloutTimeout:    time.Duration(rolloutTimeoutSec) * time.Second,
		AllowedOrigins:    getEnv("ALLOWED_ORIGINS", ""),
		QuotaRequestCPU:    getEnv("QUOTA_REQUEST_CPU", "500m"),
		QuotaRequestMemory: getEnv("QUOTA_REQUEST_MEM", "512Mi"),
		QuotaLimitCPU:      getEnv("QUOTA_LIMIT_CPU", "4000m"),
		QuotaLimitMemory:   getEnv("QUOTA_LIMIT_MEM", "3Gi"),
		QuotaMaxPods:       getEnv("QUOTA_MAX_PODS", "10"),
	}
}

// RoleResources define las peticiones (reserva/piso) y límites (techo de ráfaga) para un contenedor.
type RoleResources struct {
	RequestCPU    string `json:"request_cpu"`
	RequestMemory string `json:"request_memory"`
	LimitCPU      string `json:"limit_cpu"`
	LimitMemory   string `json:"limit_memory"`
}

// ResourcesForRole retorna la asignación de recursos elásticos según el rol del servicio.
// - web: frontend estático / SPA (huella mínima en reposo: 16Mi, techo: 128Mi).
// - api: servidores backend (huella mínima: 32Mi, techo: 512Mi con ráfaga de hasta 1 core).
// - worker: procesos batch / ML / OCR (huella mínima: 64Mi, techo: 1536Mi y 2 cores para evitar OOM).
// - default: equivalente a api.
func (c *Config) ResourcesForRole(role string) RoleResources {
	switch strings.ToLower(role) {
	case "web":
		return RoleResources{
			RequestCPU:    "10m",
			RequestMemory: "16Mi",
			LimitCPU:      "250m",
			LimitMemory:   "128Mi",
		}
	case "api":
		return RoleResources{
			RequestCPU:    "20m",
			RequestMemory: "32Mi",
			LimitCPU:      "1000m",
			LimitMemory:   "512Mi",
		}
	case "worker":
		return RoleResources{
			RequestCPU:    "20m",
			RequestMemory: "64Mi",
			LimitCPU:      "2000m",
			LimitMemory:   "1536Mi",
		}
	default:
		return RoleResources{
			RequestCPU:    "20m",
			RequestMemory: "32Mi",
			LimitCPU:      "1000m",
			LimitMemory:   "512Mi",
		}
	}
}

// ProjectDomains devuelve el dominio principal y el dominio de API para un proyecto.
// type puede ser 'compose', 'static', etc. Solo los proyectos compose tienen api_domain.
func (c *Config) ProjectDomains(projectName, projectType string) (domain, apiDomain string) {
	domain = fmt.Sprintf("%s.%s", projectName, c.BaseDomain)
	if projectType == "compose" {
		apiDomain = fmt.Sprintf("%s-api.%s", projectName, c.BaseDomain)
	}
	return
}

// DeployerOrigin retorna el origen HTTPS del deployer frontend.
func (c *Config) DeployerOrigin() string {
	return fmt.Sprintf("https://deployer.%s", c.BaseDomain)
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}
