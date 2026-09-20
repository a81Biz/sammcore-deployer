package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"sammcore-deployer/secrets"
	"sammcore-deployer/services"
	"sammcore-deployer/storage"
)

var repoRegex = regexp.MustCompile(`^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\.git)?$`)
var nonAlphanumericDash = regexp.MustCompile(`[^a-z0-9-]+`)

type AnalyzeRequest struct {
	Repo     string `json:"repo"`
	Branch   string `json:"branch,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type AnalyzeResponse struct {
	Status           string   `json:"status"`
	Error            string   `json:"error,omitempty"`
	ID               string   `json:"id,omitempty"`
	Name             string   `json:"name,omitempty"`
	Type             string   `json:"type,omitempty"`
	RequiresDatabase bool     `json:"requires_database"`
	Evidence         []string `json:"evidence,omitempty"`
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func sanitizeProjectName(repoURL string) string {
	parts := strings.Split(strings.TrimSuffix(repoURL, ".git"), "/")
	base := strings.ToLower(parts[len(parts)-1])
	clean := nonAlphanumericDash.ReplaceAllString(base, "-")
	clean = strings.Trim(clean, "-")
	if len(clean) > 35 {
		clean = clean[:35]
	}
	if clean == "" {
		clean = "app-" + generateID()
	}
	return clean
}

func Analyze(req AnalyzeRequest) AnalyzeResponse {
	repo := strings.TrimSpace(req.Repo)
	if !repoRegex.MatchString(repo) {
		return AnalyzeResponse{
			Status: "error",
			Error:  "URL de repositorio inválida. Debe ser una URL HTTPS de GitHub (ej. https://github.com/org/repo)",
		}
	}

	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = "main"
	}

	rm := services.NewRepoManager(repo, branch, "", false)
	if err := rm.Clone(); err != nil {
		return AnalyzeResponse{Status: "error", Error: fmt.Sprintf("fallo al clonar: %v", err)}
	}
	// Higiene de disco: garantizar borrado del directorio temporal al finalizar
	defer func() {
		if rm.Workdir != "" {
			_ = os.RemoveAll(rm.Workdir)
		}
	}()

	// Autenticación opcional si fue provista
	if req.Username != "" || req.Password != "" {
		rm.Username = req.Username
		rm.Password = req.Password
	} else {
		token := secrets.GetGithubToken()
		if token != "" {
			rm.Username = "git"
			rm.Password = token
		}
	}

	result, err := rm.DetectProjectType()
	if err != nil {
		return AnalyzeResponse{Status: "error", Error: err.Error()}
	}

	projectName := sanitizeProjectName(repo)
	projectID := generateID()
	requiresDB := result.Type == services.ProjectCompose

	p := storage.Project{
		ID:               projectID,
		Name:             projectName,
		Repo:             repo,
		Branch:           branch,
		Type:             result.Type.String(),
		Namespace:        projectName,
		Domain:           fmt.Sprintf("%s.sammcore.local", projectName),
		RequiresDatabase: requiresDB,
		Status:           storage.StatusAnalyzed,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if result.Type == services.ProjectCompose {
		p.APIDomain = fmt.Sprintf("api.%s.sammcore.local", projectName)
	}

	_ = storage.AddProject(p)

	return AnalyzeResponse{
		Status:           "ok",
		ID:               projectID,
		Name:             projectName,
		Type:             result.Type.String(),
		RequiresDatabase: requiresDB,
		Evidence:         result.Evidence,
	}
}
