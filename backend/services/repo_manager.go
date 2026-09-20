package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"gopkg.in/yaml.v3"
)

type ProjectType int

const (
	ProjectUnknown ProjectType = iota
	ProjectCompose
	ProjectDockerfile
	ProjectStatic
)

func (t ProjectType) String() string {
	switch t {
	case ProjectCompose:
		return "compose"
	case ProjectDockerfile:
		return "dockerfile"
	case ProjectStatic:
		return "static"
	default:
		return "unknown"
	}
}

type DetectionResult struct {
	Type             ProjectType
	Evidence         []string
	RequiresDatabase bool
	DetectedPorts    []int
	ResolvedBranch   string
	Services         []ServiceSpec
	Commit           string
}

// ServiceRole describe la función de un servicio dentro del proyecto
type ServiceRole string

const (
	RoleWeb    ServiceRole = "web"
	RoleAPI    ServiceRole = "api"
	RoleWorker ServiceRole = "worker"
	RoleApp    ServiceRole = "app" // contenedor único (tipo dockerfile)
)

// ServiceSpec describe un servicio individual detectado en el compose o Dockerfile
type ServiceSpec struct {
	Name         string      `json:"name"`
	Role         ServiceRole `json:"role"`
	Port         int         `json:"port,omitempty"`
	BuildContext string      `json:"build_context,omitempty"`
	Dockerfile   string      `json:"dockerfile,omitempty"`
	IsDB         bool        `json:"-"` // true → se descarta del plan final
}

type RepoManager struct {
	RepoURL        string
	Branch         string
	Workdir        string
	Verbose        bool
	Username       string // opcional, para auth
	Password       string // opcional, puede ser token
	ResolvedBranch string
}

func NewRepoManager(repoURL, branch, workdir string, verbose bool) *RepoManager {
	return &RepoManager{
		RepoURL: strings.TrimSpace(repoURL),
		Branch:  strings.TrimSpace(branch),
		Workdir: workdir,
		Verbose: verbose,
	}
}

func (r *RepoManager) Clone() error {
	if r.Workdir == "" {
		tmp, err := os.MkdirTemp("", "sammcore-deployer-*")
		if err != nil {
			return fmt.Errorf("no se pudo crear temp dir: %v", err)
		}
		r.Workdir = tmp
	}
	target := r.Workdir

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	branch := r.Branch
	opts := &git.CloneOptions{
		URL:          r.RepoURL,
		Progress:     progressWriter(r.Verbose),
		SingleBranch: true,
		Depth:        1,
	}

	// Asignación de autenticación ANTES de clonar
	if r.Username != "" || r.Password != "" {
		opts.Auth = &http.BasicAuth{
			Username: r.Username,
			Password: r.Password,
		}
		log.Printf("[Clone] Usando credenciales para clonado de %s", r.RepoURL)
	}

	// 1. Si el usuario solicitó explícitamente una rama específica distinta de "main" y vacía
	if branch != "" && branch != "main" {
		opts.ReferenceName = plumbing.NewBranchReferenceName(branch)
		_, err := git.PlainCloneContext(ctx, target, false, opts)
		if err != nil {
			return fmt.Errorf("fallo al clonar repositorio en la rama solicitada '%s': %w", branch, err)
		}
		r.ResolvedBranch = branch
		return nil
	}

	// 2. Si se solicitó "main" o no se especificó rama: intentar primero "main"
	opts.ReferenceName = plumbing.NewBranchReferenceName("main")
	_, err := git.PlainCloneContext(ctx, target, false, opts)
	if err == nil {
		r.ResolvedBranch = "main"
		return nil
	}

	// 3. Fallback a master
	opts.ReferenceName = plumbing.NewBranchReferenceName("master")
	_, errMaster := git.PlainCloneContext(ctx, target, false, opts)
	if errMaster == nil {
		r.ResolvedBranch = "master"
		return nil
	}

	// 4. Si no se especificó rama en absoluto (branch == ""), fallback a la rama por defecto remota
	if branch == "" {
		opts.SingleBranch = false
		opts.ReferenceName = ""
		repo, errDefault := git.PlainCloneContext(ctx, target, false, opts)
		if errDefault == nil {
			ref, refErr := repo.Head()
			if refErr == nil {
				r.ResolvedBranch = ref.Name().Short()
			} else {
				r.ResolvedBranch = "default"
			}
			return nil
		}
	}

	return fmt.Errorf("fallo al clonar repositorio (rama solicitada='%s'): no se encontró 'main' ni 'master': %w", branch, err)
}

type composeServiceBuild struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

type composeServiceDefinition struct {
	Image       string        `yaml:"image"`
	Build       interface{}   `yaml:"build"` // string o map con context/dockerfile
	Ports       []interface{} `yaml:"ports"`
	Expose      []interface{} `yaml:"expose"`
	Environment interface{}   `yaml:"environment"`
	DependsOn   interface{}   `yaml:"depends_on"`
}

func (c *composeServiceDefinition) ParseBuild() (context string, dockerfile string) {
	switch v := c.Build.(type) {
	case string:
		return v, "Dockerfile"
	case map[string]interface{}:
		ctx, _ := v["context"].(string)
		df, _ := v["dockerfile"].(string)
		if ctx == "" {
			ctx = "."
		}
		if df == "" {
			df = "Dockerfile"
		}
		return ctx, df
	}
	return "", ""
}

type composeFileStructure struct {
	Services map[string]composeServiceDefinition `yaml:"services"`
}

func parseContainerPort(raw interface{}) (int, bool) {
	switch v := raw.(type) {
	case int:
		if v > 0 && v <= 65535 {
			return v, true
		}
	case string:
		// Remover posibles protocolos tipo "/tcp" o "/udp"
		clean := strings.Split(strings.TrimSpace(v), "/")[0]
		// Separar por ":" para casos "host:container", "ip:host:container" o solo "container"
		tokens := strings.Split(clean, ":")
		if len(tokens) > 0 {
			targetStr := tokens[len(tokens)-1]
			if p, err := strconv.Atoi(targetStr); err == nil && p > 0 && p <= 65535 {
				return p, true
			}
		}
	case map[string]interface{}:
		// Formato expandido Compose v2+: target: 80, published: 8080
		if targetVal, ok := v["target"]; ok {
			return parseContainerPort(targetVal)
		}
	}
	return 0, false
}

func isDBEnvKey(key string) bool {
	upper := strings.ToUpper(key)
	return strings.Contains(upper, "POSTGRES") ||
		strings.Contains(upper, "DATABASE_URL") ||
		strings.Contains(upper, "DB_HOST") ||
		strings.Contains(upper, "DB_NAME") ||
		strings.Contains(upper, "DB_USER") ||
		strings.Contains(upper, "DB_PASSWORD") ||
		strings.Contains(upper, "MYSQL_")
}

func hasDBClientEnv(svc composeServiceDefinition) bool {
	switch env := svc.Environment.(type) {
	case []interface{}:
		for _, item := range env {
			if s, ok := item.(string); ok {
				parts := strings.SplitN(s, "=", 2)
				if isDBEnvKey(parts[0]) {
					return true
				}
			}
		}
	case map[string]interface{}:
		for k := range env {
			if isDBEnvKey(k) {
				return true
			}
		}
	}
	return false
}

func isDBService(svc composeServiceDefinition) bool {
	// Si tiene Build, es una aplicación compilada (backend, worker, etc.), NO un motor de BD
	if ctx, _ := svc.ParseBuild(); ctx != "" {
		return false
	}

	// 1. Imagen del contenedor (sin falsos positivos por comentarios)
	img := strings.ToLower(svc.Image)
	if strings.Contains(img, "postgres") ||
		strings.Contains(img, "mysql") ||
		strings.Contains(img, "mariadb") ||
		strings.Contains(img, "cockroach") ||
		strings.Contains(img, "timescale") {
		return true
	}

	// 2. Variables de entorno de inicialización del motor (sin DB_HOST)
	// Si contiene DB_HOST o DATABASE_HOST, es un cliente conectándose a un host externo, NO el servidor
	hasDBInitVar := false
	hasDBHost := false

	checkKey := func(k string) {
		upper := strings.ToUpper(k)
		if upper == "DB_HOST" || upper == "DATABASE_HOST" {
			hasDBHost = true
		}
		if upper == "POSTGRES_PASSWORD" || upper == "MYSQL_ROOT_PASSWORD" || upper == "MARIADB_ROOT_PASSWORD" {
			hasDBInitVar = true
		}
	}

	switch env := svc.Environment.(type) {
	case []interface{}:
		for _, item := range env {
			if s, ok := item.(string); ok {
				parts := strings.SplitN(s, "=", 2)
				checkKey(parts[0])
			}
		}
	case map[string]interface{}:
		for k := range env {
			checkKey(k)
		}
	}

	if hasDBHost {
		return false
	}

	return hasDBInitVar
}

var exposeRegex = regexp.MustCompile(`(?i)^\s*EXPOSE\s+(\d+)`)

func (r *RepoManager) DetectProjectType() (DetectionResult, error) {
	if r.Workdir == "" {
		return DetectionResult{}, errors.New("Workdir no establecido")
	}

	// Inspección acotada a la raíz del repositorio para evitar falsos positivos
	entries, err := os.ReadDir(r.Workdir)
	if err != nil {
		return DetectionResult{}, err
	}

	var hasComposeFile string
	var hasDockerfile string
	var hasIndexHTML string
	var hasPackageJSON string

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		switch name {
		case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
			// Preferir compose si se encuentra primero
			if hasComposeFile == "" {
				hasComposeFile = entry.Name()
			}
		case "dockerfile":
			hasDockerfile = entry.Name()
		case "index.html":
			hasIndexHTML = entry.Name()
		case "package.json":
			hasPackageJSON = entry.Name()
		}
	}

	res := DetectionResult{
		ResolvedBranch: r.ResolvedBranch,
	}

	// 1. Detección Compose estructurada con YAML parser
	if hasComposeFile != "" {
		res.Type = ProjectCompose
		res.Evidence = append(res.Evidence, hasComposeFile)

		content, err := os.ReadFile(filepath.Join(r.Workdir, hasComposeFile))
		if err == nil {
			var composeData composeFileStructure
			if err := yaml.Unmarshal(content, &composeData); err == nil {
				seenPorts := make(map[int]bool)

				for _, svc := range composeData.Services {
					if isDBService(svc) || hasDBClientEnv(svc) {
						res.RequiresDatabase = true
					}

					// Extraer puertos de contenedor reales (target)
					for _, pRaw := range svc.Ports {
						if p, ok := parseContainerPort(pRaw); ok && !seenPorts[p] {
							seenPorts[p] = true
							res.DetectedPorts = append(res.DetectedPorts, p)
						}
					}
					// Extraer puertos de expose
					for _, expRaw := range svc.Expose {
						if p, ok := parseContainerPort(expRaw); ok && !seenPorts[p] {
							seenPorts[p] = true
							res.DetectedPorts = append(res.DetectedPorts, p)
						}
					}
				}
			} else {
				res.Evidence = append(res.Evidence, "advertencia: archivo compose no es YAML válido")
			}
		}
		return res, nil
	}

	// 2. Detección Dockerfile
	if hasDockerfile != "" {
		res.Type = ProjectDockerfile
		res.Evidence = append(res.Evidence, hasDockerfile)

		content, err := os.ReadFile(filepath.Join(r.Workdir, hasDockerfile))
		if err == nil {
			lines := strings.Split(string(content), "\n")
			seenPorts := make(map[int]bool)
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "#") {
					continue // Ignorar comentarios
				}
				matches := exposeRegex.FindAllStringSubmatch(trimmed, -1)
				for _, m := range matches {
					if len(m) > 1 {
						if p, err := strconv.Atoi(m[1]); err == nil && !seenPorts[p] {
							seenPorts[p] = true
							res.DetectedPorts = append(res.DetectedPorts, p)
						}
					}
				}
			}
		}
		return res, nil
	}

	// 3. Detección Sitio Estático
	if hasIndexHTML != "" {
		if hasPackageJSON != "" {
			pkgContent, _ := os.ReadFile(filepath.Join(r.Workdir, hasPackageJSON))
			if strings.Contains(string(pkgContent), "vite") || strings.Contains(string(pkgContent), "react-scripts") {
				res.Evidence = append(res.Evidence, "package.json (Vite/React frontend)")
			}
		}
		res.Type = ProjectStatic
		res.Evidence = append(res.Evidence, hasIndexHTML)
		res.DetectedPorts = []int{80}
		return res, nil
	}

	res.Type = ProjectUnknown
	return res, nil
}

func progressWriter(verbose bool) io.Writer {
	if verbose {
		return os.Stdout
	}
	return nil
}

// inferServiceRole deduce el rol de un servicio basándose en su nombre
func inferServiceRole(name string) ServiceRole {
	lower := strings.ToLower(name)
	// Patrones de frontend/web
	for _, p := range []string{"front", "web", "ui", "client", "app-web", "nginx"} {
		if strings.Contains(lower, p) {
			return RoleWeb
		}
	}
	// Patrones de backend/API
	for _, p := range []string{"back", "api", "server", "gateway"} {
		if strings.Contains(lower, p) {
			return RoleAPI
		}
	}
	// Patrones de worker
	for _, p := range []string{"work", "queue", "cron", "job", "process", "consumer"} {
		if strings.Contains(lower, p) {
			return RoleWorker
		}
	}
	return RoleAPI // default: tratar como API
}

// extractPortFromDockerfile lee un Dockerfile y extrae el primer EXPOSE válido
func extractPortFromDockerfile(dockerfilePath string) int {
	content, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return 0
	}
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		matches := exposeRegex.FindAllStringSubmatch(trimmed, -1)
		for _, m := range matches {
			if len(m) > 1 {
				if p, err := strconv.Atoi(m[1]); err == nil && p > 0 && p <= 65535 {
					return p
				}
			}
		}
	}
	return 0
}

// getCommitSHA obtiene el SHA del HEAD del repositorio clonado
func (r *RepoManager) getCommitSHA() string {
	if r.Workdir == "" {
		return ""
	}
	repo, err := git.PlainOpen(r.Workdir)
	if err != nil {
		return ""
	}
	ref, err := repo.Head()
	if err != nil {
		return ""
	}
	return ref.Hash().String()
}

// DetectServicePlan analiza el repositorio y devuelve un plan estructurado por servicio.
// Para tipo compose: parsea cada servicio, descarta DBs, resuelve puertos y contextos de build.
// Para tipo dockerfile: retorna un servicio único de tipo "app".
// Para tipo static: retorna un servicio único de tipo "web" con puerto 80.
func (r *RepoManager) DetectServicePlan() (DetectionResult, error) {
	res, err := r.DetectProjectType()
	if err != nil {
		return res, err
	}

	res.Commit = r.getCommitSHA()

	switch res.Type {
	case ProjectCompose:
		res.Services = r.extractComposeServices()
	case ProjectDockerfile:
		port := 8080 // default para contenedor único
		dfPath := filepath.Join(r.Workdir, "Dockerfile")
		if p := extractPortFromDockerfile(dfPath); p > 0 {
			port = p
		}
		res.Services = []ServiceSpec{{
			Name:         "app",
			Role:         RoleApp,
			Port:         port,
			BuildContext: ".",
			Dockerfile:   "Dockerfile",
		}}
	case ProjectStatic:
		res.Services = []ServiceSpec{{
			Name:         "static",
			Role:         RoleWeb,
			Port:         80,
			BuildContext: ".",
			Dockerfile:   "",
		}}
	}

	return res, nil
}

// extractComposeServices parsea el docker-compose.yml y genera ServiceSpec por cada servicio
func (r *RepoManager) extractComposeServices() []ServiceSpec {
	// Buscar archivo compose
	var composeFile string
	entries, err := os.ReadDir(r.Workdir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch strings.ToLower(e.Name()) {
		case "docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml":
			if composeFile == "" {
				composeFile = e.Name()
			}
		}
	}
	if composeFile == "" {
		return nil
	}

	content, err := os.ReadFile(filepath.Join(r.Workdir, composeFile))
	if err != nil {
		return nil
	}

	var composeData composeFileStructure
	if err := yaml.Unmarshal(content, &composeData); err != nil {
		return nil
	}

	// Recopilar nombres ordenados para iteración determinista
	names := make([]string, 0, len(composeData.Services))
	for name := range composeData.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	var services []ServiceSpec
	for _, name := range names {
		svc := composeData.Services[name]

		spec := ServiceSpec{
			Name: name,
		}

		// 1. Detectar si es servicio de BD
		if isDBService(svc) {
			spec.IsDB = true
			// No incluir en el plan final
			continue
		}

		// 2. Inferir rol
		spec.Role = inferServiceRole(name)

		// 3. Extraer build context
		buildCtx, buildDf := svc.ParseBuild()
		spec.BuildContext = buildCtx
		spec.Dockerfile = buildDf

		// 4. Resolver puerto: primero compose ports/expose
		port := 0
		for _, pRaw := range svc.Ports {
			if p, ok := parseContainerPort(pRaw); ok {
				// Ignorar puertos 443 (TLS interno que el Ingress maneja) y 5432 (DB)
				if p != 443 && p != 5432 {
					port = p
					break
				}
			}
		}
		if port == 0 {
			for _, expRaw := range svc.Expose {
				if p, ok := parseContainerPort(expRaw); ok && p != 443 && p != 5432 {
					port = p
					break
				}
			}
		}

		// 5. Si no hay puerto en compose, buscar EXPOSE en el Dockerfile del contexto de build
		if port == 0 && buildCtx != "" {
			dfPath := filepath.Join(r.Workdir, buildCtx, buildDf)
			if p := extractPortFromDockerfile(dfPath); p > 0 && p != 443 && p != 5432 {
				port = p
			}
		}

		// 6. Defaults según rol
		if port == 0 {
			switch spec.Role {
			case RoleWeb:
				port = 80
			case RoleAPI:
				port = 8080
			}
			// Workers no necesitan puerto
		}

		spec.Port = port
		services = append(services, spec)
	}

	return services
}
