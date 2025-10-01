package core

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"sammcore-deployer/secrets"
	"sammcore-deployer/services"
	"sammcore-deployer/storage"
)

type AnalyzeRequest struct {
	Repo     string `json:"repo"`
	Branch   string `json:"branch,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type AnalyzeResponse struct {
	Status   string   `json:"status"`
	Error    string   `json:"error,omitempty"`
	Workdir  string   `json:"workdir,omitempty"`
	Type     string   `json:"type,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

func generateID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func Analyze(req AnalyzeRequest) AnalyzeResponse {
	repo := strings.TrimSpace(req.Repo)
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = "main"
	}

	rm := services.NewRepoManager(repo, branch, "", true)

	// auth opcional
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
		return AnalyzeResponse{Status: "error", Error: err.Error()}
	}

	result, err := rm.DetectProjectType()
	if err != nil {
		return AnalyzeResponse{Status: "error", Error: err.Error()}
	}
	// Guardar en history
	p := storage.Project{
		ID:        generateID(),
		Repo:      repo,
		Branch:    branch,
		Type:      result.Type.String(),
		Status:    "analizado",
		Timestamp: time.Now(),
	}
	storage.AddProject(p)

	return AnalyzeResponse{
		Status:   "ok",
		Workdir:  rm.Workdir,
		Type:     result.Type.String(),
		Evidence: result.Evidence,
	}

}
