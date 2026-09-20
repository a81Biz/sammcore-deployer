package services

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
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

	branch := r.Branch
	if branch == "" {
		branch = "main"
	}

	opts := &git.CloneOptions{
		URL:           r.RepoURL,
		Progress:      progressWriter(r.Verbose),
		SingleBranch:  true,
		Depth:         1,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	}

	// Asignación de autenticación ANTES de clonar
	if r.Username != "" || r.Password != "" {
		opts.Auth = &http.BasicAuth{
			Username: r.Username,
			Password: r.Password,
		}
		log.Printf("[Clone] Usando credenciales para clonado de %s", r.RepoURL)
	}

	// 1. Intentar con la rama solicitada
	_, err := git.PlainClone(target, false, opts)
	if err == nil {
		r.ResolvedBranch = branch
		return nil
	}

	// 2. Fallback a master si se solicitó main
	if branch == "main" {
		opts.ReferenceName = plumbing.NewBranchReferenceName("master")
		_, errMaster := git.PlainClone(target, false, opts)
		if errMaster == nil {
			r.ResolvedBranch = "master"
			return nil
		}
	}

	// 3. Fallback a la rama por defecto del repositorio remoto
	opts.SingleBranch = false
	opts.ReferenceName = ""
	repo, errDefault := git.PlainClone(target, false, opts)
	if errDefault == nil {
		ref, refErr := repo.Head()
		if refErr == nil {
			r.ResolvedBranch = ref.Name().Short()
		} else {
			r.ResolvedBranch = "default"
		}
		return nil
	}

	return fmt.Errorf("fallo al clonar repositorio (rama=%s): %v", branch, err)
}

var portRegex = regexp.MustCompile(`(?m)^\s*-\s*["']?(\d+):(\d+)["']?`)
var exposeRegex = regexp.MustCompile(`(?i)^\s*EXPOSE\s+(\d+)`)
var dbImageRegex = regexp.MustCompile(`(?i)(postgres|mysql|mariadb|cockroach|timescale)`)

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
		case "docker-compose.yml", "docker-compose.yaml":
			hasComposeFile = entry.Name()
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

	// 1. Detección Compose
	if hasComposeFile != "" {
		res.Type = ProjectCompose
		res.Evidence = append(res.Evidence, hasComposeFile)

		content, err := os.ReadFile(filepath.Join(r.Workdir, hasComposeFile))
		if err == nil {
			text := string(content)
			// Verificar si declara un servicio de base de datos relacional
			if dbImageRegex.MatchString(text) || strings.Contains(text, "5432") || strings.Contains(text, "3306") || strings.Contains(text, "DB_HOST") {
				res.RequiresDatabase = true
			}
			// Extraer puertos expuestos
			matches := portRegex.FindAllStringSubmatch(text, -1)
			seenPorts := make(map[int]bool)
			for _, m := range matches {
				if len(m) > 1 {
					if p, err := strconv.Atoi(m[1]); err == nil && !seenPorts[p] {
						seenPorts[p] = true
						res.DetectedPorts = append(res.DetectedPorts, p)
					}
				}
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
			matches := exposeRegex.FindAllStringSubmatch(string(content), -1)
			for _, m := range matches {
				if len(m) > 1 {
					if p, err := strconv.Atoi(m[1]); err == nil {
						res.DetectedPorts = append(res.DetectedPorts, p)
					}
				}
			}
		}
		return res, nil
	}

	// 3. Detección Sitio Estático
	if hasIndexHTML != "" {
		// Si tiene package.json, verificar si es un proyecto de build (Vite/React) o estático puro
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

func progressWriter(verbose bool) *os.File {
	if verbose {
		return os.Stdout
	}
	return nil
}
