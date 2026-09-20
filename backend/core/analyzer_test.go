package core

import (
	"testing"
)

func TestSanitizeProjectName(t *testing.T) {
	tests := []struct {
		repoURL string
		want    string
		wantErr bool
	}{
		{"https://github.com/org/normal-app", "normal-app", false},
		{"https://github.com/org/Normal_App.git", "normal-app", false},
		{"https://github.com/org/api", "", true},                   // reserved name "api" -> rejected because ends with -api
		{"https://github.com/org/service-api", "", true},           // ends in -api -> error
		{"https://github.com/org/project-docs", "", true},          // ends in -docs -> error
		{"https://github.com/org/deployer", "app-deployer", false}, // "deployer" -> "app-deployer" (ok)
		{"https://github.com/org/ab", "app-ab", false},             // < 3 chars -> app-ab
		{"https://github.com/org/___", "", false},                  // all underscores -> sanitized
	}

	for _, tt := range tests {
		got, err := sanitizeProjectName(tt.repoURL)
		if tt.wantErr {
			if err == nil {
				t.Errorf("sanitizeProjectName(%q) expected error, got %q", tt.repoURL, got)
			}
		} else {
			if err != nil {
				t.Errorf("sanitizeProjectName(%q) unexpected error: %v", tt.repoURL, err)
			}
			if tt.want != "" && got != tt.want {
				t.Errorf("sanitizeProjectName(%q) = %q, want %q", tt.repoURL, got, tt.want)
			}
		}
	}
}

func TestDeriveDeterministicID(t *testing.T) {
	id1 := deriveDeterministicID("https://github.com/a81Biz/sammcore-deployer.git")
	id2 := deriveDeterministicID("https://github.com/a81biz/sammcore-deployer")

	if id1 != "a81biz-sammcore-deployer" {
		t.Errorf("deriveDeterministicID expected a81biz-sammcore-deployer, got %s", id1)
	}
	if id1 != id2 {
		t.Errorf("expected deterministic IDs to match: %s != %s", id1, id2)
	}
}

func TestAnalyze_InvalidRepo(t *testing.T) {
	resp := Analyze(AnalyzeRequest{Repo: "https://gitlab.com/invalid/repo"})
	if resp.Status != "error" || resp.Code != "INVALID_REPO_URL" {
		t.Errorf("expected INVALID_REPO_URL, got %v", resp)
	}
}
