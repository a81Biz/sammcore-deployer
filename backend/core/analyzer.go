package core

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"regexp"
	"strings"

	"sammcore-deployer/secrets"
	"sammcore-deployer/services"
)

var RepoRegex = regexp.MustCompile(`^https://github\.com/[a-zA-Z0-9_.-]+/[a-zA-Z0-9_.-]+(\.git)?/?$`)
var CommitRegex = regexp.MustCompile(`^[0-9a-f]{7,40}$`)
var repoRegex = RepoRegex
var nonAlphanumericDash = regexp.MustCompile(`[^a-z0-9-]+`)

func CleanRepoURL(raw string) string {
	cleaned := strings.TrimSpace(raw)
	if strings.HasPrefix(cleaned, "git@github.com:") {
		cleaned = "https://github.com/" + strings.TrimPrefix(cleaned, "git@github.com:")
	}
	return strings.TrimRight(cleaned, "/")
}

var reservedNames = map[string]bool{
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
	Status           string                 `json:"status"`
	Error            string                 `json:"error,omitempty"`
	Code             string                 `json:"code,omitempty"`
	ID               string                 `json:"id,omitempty"`
	Name             string                 `json:"name,omitempty"`
	Type             string                 `json:"type,omitempty"`
	Branch           string                 `json:"branch,omitempty"`
	Domain           string                 `json:"domain,omitempty"`
	APIDomain        string                 `json:"api_domain,omitempty"`
	RequiresDatabase bool                   `json:"requires_database"`
	DetectedPorts    []int                  `json:"detected_ports,omitempty"`
	Evidence         []string               `json:"evidence,omitempty"`
	Services         []services.ServiceSpec `json:"services,omitempty"`
	Commit           string                 `json:"commit,omitempty"`
}

func generateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func DeriveDeterministicID(repoURL string) string {
	cleanURL := strings.ToLower(strings.TrimSpace(repoURL))
	cleanURL = strings.TrimSuffix(cleanURL, ".git")
	cleanURL = strings.TrimPrefix(cleanURL, "https://github.com/")

	parts := strings.Split(cleanURL, "/")
	if len(parts) >= 2 {
		owner := nonAlphanumericDash.ReplaceAllString(parts[0], "-")
		repo := nonAlphanumericDash.ReplaceAllString(parts[1], "-")
		id := fmt.Sprintf("%s-%s", strings.Trim(owner, "-"), strings.Trim(repo, "-"))
		if len(id) > 50 {
			id = id[:50]
		}
		return strings.Trim(id, "-")
	}
	return generateID()
}

func sanitizeProjectName(repoURL string) (string, error) {
	cleanURL := strings.ToLower(strings.TrimSpace(repoURL))
	cleanURL = strings.TrimSuffix(cleanURL, ".git")
	parts := strings.Split(cleanURL, "/")
	base := parts[len(parts)-1]

	clean := nonAlphanumericDash.ReplaceAllString(base, "-")
	clean = strings.Trim(clean, "-")

	if len(clean) < 3 || isReservedName(clean) {
		clean = "app-" + clean
	}
	clean = strings.Trim(clean, "-")
	if len(clean) < 3 {
		clean = "app-" + generateID()
	}

	if len(clean) > 35 {
		clean = clean[:35]
		clean = strings.TrimRight(clean, "-")
	}

	if strings.HasSuffix(clean, "-api") || strings.HasSuffix(clean, "-docs") {
		return "", fmt.Errorf("el nombre del proyecto '%s' no puede terminar en '-api' ni '-docs' para evitar colisiones de subdominios", clean)
	}

	return clean, nil
}

func Analyze(req AnalyzeRequest) AnalyzeResponse {
	rawRepo := CleanRepoURL(req.Repo)
	if !repoRegex.MatchString(rawRepo) {
		return AnalyzeResponse{
			Status: "error",
			Error:  "URL de repositorio inválida. Debe ser una URL HTTPS de GitHub (ej. https://github.com/org/repo)",
			Code:   "INVALID_REPO_URL",
		}
	}

	repoNormalized := strings.ToLower(rawRepo)
	projectName, err := sanitizeProjectName(repoNormalized)
	if err != nil {
		return AnalyzeResponse{
			Status: "error",
			Error:  err.Error(),
			Code:   "INVALID_PROJECT_NAME",
		}
	}

	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = "main"
	}

	rm := services.NewRepoManager(rawRepo, branch, "", false)

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
		return AnalyzeResponse{
			Status: "error",
			Error:  fmt.Sprintf("fallo al clonar: %v", err),
			Code:   "CLONE_FAILED",
		}
	}

	result, err := rm.DetectServicePlan()
	if err != nil {
		return AnalyzeResponse{
			Status: "error",
			Error:  err.Error(),
			Code:   "DETECTION_FAILED",
		}
	}

	resolvedBranch := rm.ResolvedBranch
	if resolvedBranch == "" {
		resolvedBranch = branch
	}

	projectID := DeriveDeterministicID(repoNormalized)

	baseDomain := os.Getenv("BASE_DOMAIN")
	if baseDomain == "" {
		baseDomain = "sammcore.local"
	}
	domain := fmt.Sprintf("%s.%s", projectName, baseDomain)
	var apiDomain string

	if result.Type == services.ProjectCompose {
		// Subdominio de nivel único compatible con el wildcard TLS *.sammcore.local
		apiDomain = fmt.Sprintf("%s-api.%s", projectName, baseDomain)
	}

	// NOTA ARQUITECTÓNICA: Analyze es estrictamente de sólo lectura.
	// NO persiste en history.json ni muta el estado de proyectos en ejecución.
	// La persistencia y el despliegue corresponden exclusivamente a POST /api/deploy.
	return AnalyzeResponse{
		Status:           "ok",
		ID:               projectID,
		Name:             projectName,
		Type:             result.Type.String(),
		Branch:           resolvedBranch,
		Domain:           domain,
		APIDomain:        apiDomain,
		RequiresDatabase: result.RequiresDatabase,
		DetectedPorts:    result.DetectedPorts,
		Evidence:         result.Evidence,
		Services:         result.Services,
		Commit:           result.Commit,
	}
}
