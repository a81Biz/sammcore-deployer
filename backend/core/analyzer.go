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

var reservedNames = map[string]bool{
	"deployer":      true,
	"supabase":      true,
	"monitoring":    true,
	"ingress-nginx": true,
	"default":       true,
	"api":           true,
	"docs":          true,
	"admin":         true,
	"grafana":       true,
	"prometheus":    true,
	"traefik":       true,
	"portainer":     true,
}

func isReservedName(name string) bool {
	if strings.HasPrefix(name, "kube-") {
		return true
	}
	return reservedNames[name]
}

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
	Branch           string   `json:"branch,omitempty"`
	RequiresDatabase bool     `json:"requires_database"`
	DetectedPorts    []int    `json:"detected_ports,omitempty"`
	Evidence         []string `json:"evidence,omitempty"`
}

func generateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func sanitizeProjectName(repoURL string) string {
	parts := strings.Split(strings.TrimSuffix(repoURL, ".git"), "/")
	base := strings.ToLower(parts[len(parts)-1])
	clean := nonAlphanumericDash.ReplaceAllString(base, "-")
	clean = strings.Trim(clean, "-")

	// Cumplir con longitud mínima de 3 caracteres y máxima de 35
	if len(clean) < 3 || isReservedName(clean) {
		clean = "app-" + clean
	}
	if len(clean) > 35 {
		clean = clean[:35]
		clean = strings.TrimRight(clean, "-")
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

	// Garantizar limpieza de disco SIEMPRE mediante defer
	defer func() {
		if rm.Workdir != "" {
			_ = os.RemoveAll(rm.Workdir)
		}
	}()

	// Asignación estricta de credenciales ANTES de llamar a Clone()
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

	if err := rm.Clone(); err != nil {
		return AnalyzeResponse{Status: "error", Error: fmt.Sprintf("fallo al clonar: %v", err)}
	}

	result, err := rm.DetectProjectType()
	if err != nil {
		return AnalyzeResponse{Status: "error", Error: err.Error()}
	}

	projectName := sanitizeProjectName(repo)
	resolvedBranch := rm.ResolvedBranch
	if resolvedBranch == "" {
		resolvedBranch = branch
	}

	// Unicidad: verificar si el proyecto ya existía para actualizarlo o crearlo
	existingProjects, _ := storage.LoadProjects()
	var projectID string
	for _, ep := range existingProjects {
		if ep.Repo == repo {
			projectID = ep.ID
			break
		}
	}
	if projectID == "" {
		projectID = generateID()
	}

	p := storage.Project{
		ID:               projectID,
		Name:             projectName,
		Repo:             repo,
		Branch:           resolvedBranch,
		Type:             result.Type.String(),
		Namespace:        projectName,
		Domain:           fmt.Sprintf("%s.sammcore.local", projectName),
		RequiresDatabase: result.RequiresDatabase,
		Status:           storage.StatusAnalyzed,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if result.Type == services.ProjectCompose {
		p.APIDomain = fmt.Sprintf("api.%s.sammcore.local", projectName)
	}

	_ = storage.AddOrUpdateProject(p)

	return AnalyzeResponse{
		Status:           "ok",
		ID:               projectID,
		Name:             projectName,
		Type:             result.Type.String(),
		Branch:           resolvedBranch,
		RequiresDatabase: result.RequiresDatabase,
		DetectedPorts:    result.DetectedPorts,
		Evidence:         result.Evidence,
	}
}
