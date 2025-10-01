package core

import (
	"strings"

	"sammcore-deployer/secrets"
	"sammcore-deployer/services"
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

	return AnalyzeResponse{
		Status:   "ok",
		Workdir:  rm.Workdir,
		Type:     result.Type.String(),
		Evidence: result.Evidence,
	}
}
