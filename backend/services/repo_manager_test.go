package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectTypeString(t *testing.T) {
	tests := []struct {
		pt   ProjectType
		want string
	}{
		{ProjectCompose, "compose"},
		{ProjectDockerfile, "dockerfile"},
		{ProjectStatic, "static"},
		{ProjectUnknown, "unknown"},
	}

	for _, tt := range tests {
		if got := tt.pt.String(); got != tt.want {
			t.Errorf("ProjectType.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestDetectProjectType_Compose(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-compose-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composeFile := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composeFile, []byte("version: '3'"), 0644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectProjectType()
	if err != nil {
		t.Fatalf("DetectProjectType returned error: %v", err)
	}

	if res.Type != ProjectCompose {
		t.Errorf("expected ProjectCompose, got %v", res.Type)
	}
}

func TestDetectProjectType_Dockerfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-dockerfile-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	if err := os.WriteFile(dockerfile, []byte("FROM alpine:latest"), 0644); err != nil {
		t.Fatalf("failed to write dockerfile: %v", err)
	}

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectProjectType()
	if err != nil {
		t.Fatalf("DetectProjectType returned error: %v", err)
	}

	if res.Type != ProjectDockerfile {
		t.Errorf("expected ProjectDockerfile, got %v", res.Type)
	}
}

func TestDetectProjectType_Static(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-static-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	indexFile := filepath.Join(tmpDir, "index.html")
	if err := os.WriteFile(indexFile, []byte("<html><body>Hello</body></html>"), 0644); err != nil {
		t.Fatalf("failed to write index file: %v", err)
	}

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectProjectType()
	if err != nil {
		t.Fatalf("DetectProjectType returned error: %v", err)
	}

	if res.Type != ProjectStatic {
		t.Errorf("expected ProjectStatic, got %v", res.Type)
	}
}

func TestDetectProjectType_Unknown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-unknown-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectProjectType()
	if err != nil {
		t.Fatalf("DetectProjectType returned error: %v", err)
	}

	if res.Type != ProjectUnknown {
		t.Errorf("expected ProjectUnknown, got %v", res.Type)
	}
}
