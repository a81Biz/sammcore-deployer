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

type composeServiceDefinition struct {
	Image       string        `yaml:"image"`
	Ports       []interface{} `yaml:"ports"`
	Expose      []interface{} `yaml:"expose"`
	Environment interface{}   `yaml:"environment"`
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

func isDBService(svc composeServiceDefinition) bool {
	// 1. Imagen del contenedor (sin falsos positivos por comentarios)
	img := strings.ToLower(svc.Image)
	if strings.Contains(img, "postgres") ||
		strings.Contains(img, "mysql") ||
		strings.Contains(img, "mariadb") ||
		strings.Contains(img, "cockroach") ||
		strings.Contains(img, "timescale") {
		return true
	}

	// 2. Variables de entorno estructuradas
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
					if isDBService(svc) {
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
