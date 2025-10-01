package services

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
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
)

func (t ProjectType) String() string {
	switch t {
	case ProjectCompose:
		return "compose"
	case ProjectDockerfile:
		return "dockerfile"
	default:
		return "unknown"
	}
}

type DetectionResult struct {
	Type     ProjectType
	Evidence []string
}

type RepoManager struct {
	RepoURL  string
	Branch   string
	Workdir  string
	Verbose  bool
	Username string // opcional, para auth
	Password string // opcional, puede ser token
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

	log.Printf("[Clone] Clonando repo %s en %s (branch=%s)", r.RepoURL, target, branch)

	opts := &git.CloneOptions{
		URL:           r.RepoURL,
		Progress:      progressWriter(r.Verbose),
		SingleBranch:  true,
		ReferenceName: plumbing.NewBranchReferenceName(branch),
	}

	// Si se configuró auth
	if r.Username != "" || r.Password != "" {
		opts.Auth = &http.BasicAuth{
			Username: r.Username,
			Password: r.Password,
		}
		log.Printf("[Clone] Usando autenticación para %s", r.Username)
	}

	// Primer intento
	_, err := git.PlainClone(target, false, opts)
	if err == nil {
		log.Printf("[Clone] Éxito: rama %s", branch)
		return nil
	}
	log.Printf("[Clone] Falló rama %s: %v", branch, err)

	// Fallback a master si era main
	if branch == "main" {
		log.Printf("[Clone] Intentando fallback -> master")
		opts.ReferenceName = plumbing.NewBranchReferenceName("master")
		_, errMaster := git.PlainClone(target, false, opts)
		if errMaster == nil {
			log.Printf("[Clone] Éxito: rama master")
			return nil
		}
		log.Printf("[Clone] Falló master: %v", errMaster)
	}

	// Fallback sin rama (default)
	log.Printf("[Clone] Intentando fallback -> default (sin rama)")
	opts.SingleBranch = false
	opts.ReferenceName = ""
	_, errDefault := git.PlainClone(target, false, opts)
	if errDefault == nil {
		log.Printf("[Clone] Éxito: rama default")
		return nil
	}
	log.Printf("[Clone] Falló default: %v", errDefault)

	return fmt.Errorf("no se pudo clonar repo (branch=%s): %v", branch, err)
}

func (r *RepoManager) DetectProjectType() (DetectionResult, error) {
	if r.Workdir == "" {
		return DetectionResult{}, errors.New("Workdir no establecido")
	}

	var evidence []string
	hasCompose := false
	hasDockerfile := false

	err := filepath.WalkDir(r.Workdir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		name := strings.ToLower(d.Name())
		switch {
		case name == "docker-compose.yml" || name == "docker-compose.yaml":
			hasCompose = true
			evidence = append(evidence, rel(r.Workdir, path))
		case name == "dockerfile":
			hasDockerfile = true
			evidence = append(evidence, rel(r.Workdir, path))
		}
		return nil
	})
	if err != nil {
		return DetectionResult{}, err
	}

	switch {
	case hasCompose:
		return DetectionResult{Type: ProjectCompose, Evidence: evidence}, nil
	case hasDockerfile:
		return DetectionResult{Type: ProjectDockerfile, Evidence: evidence}, nil
	default:
		return DetectionResult{Type: ProjectUnknown, Evidence: evidence}, nil
	}
}

func rel(root, p string) string {
	if q, err := filepath.Rel(root, p); err == nil {
		return q
	}
	return p
}

func progressWriter(verbose bool) *os.File {
	if verbose {
		return os.Stdout
	}
	return nil
}
