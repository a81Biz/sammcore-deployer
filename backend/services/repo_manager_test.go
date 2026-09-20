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
	content := `
services:
  web:
    image: nginx:alpine
    ports:
      - "80:80"
`
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
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
	if len(res.DetectedPorts) != 1 || res.DetectedPorts[0] != 80 {
		t.Errorf("expected detected port [80], got %v", res.DetectedPorts)
	}
}

func TestDetectProjectType_Compose_PortExtraction(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-compose-ports-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composeFile := filepath.Join(tmpDir, "compose.yaml")
	// Notice: host port is 3000, container port is 80!
	content := `
services:
  web:
    image: my-app:latest
    ports:
      - "3000:80"
      - "127.0.0.1:8080:8080/tcp"
`
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectProjectType()
	if err != nil {
		t.Fatalf("DetectProjectType returned error: %v", err)
	}

	if res.Type != ProjectCompose {
		t.Fatalf("expected ProjectCompose, got %v", res.Type)
	}

	// Should extract container ports 80 and 8080 (NOT 3000)
	has80 := false
	has8080 := false
	for _, p := range res.DetectedPorts {
		if p == 80 {
			has80 = true
		}
		if p == 8080 {
			has8080 = true
		}
		if p == 3000 {
			t.Errorf("detected host port 3000 instead of container port 80!")
		}
	}

	if !has80 || !has8080 {
		t.Errorf("expected container ports [80, 8080], got %v", res.DetectedPorts)
	}
}

func TestDetectProjectType_Compose_CommentNoFalsePositive(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-compose-comment-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composeFile := filepath.Join(tmpDir, "docker-compose.yaml")
	// Contains comments mentioning postgres/mysql/5432, but real service is just redis
	content := `
# NOTA: En este proyecto NO usamos postgres ni mysql
# La base de datos no corre en 5432
services:
  cache:
    image: redis:alpine
    ports:
      - "6379:6379"
`
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectProjectType()
	if err != nil {
		t.Fatalf("DetectProjectType returned error: %v", err)
	}

	if res.RequiresDatabase {
		t.Errorf("expected RequiresDatabase to be false when database words only exist in comments, but got true")
	}
}

func TestDetectProjectType_Compose_RealDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-compose-db-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	composeFile := filepath.Join(tmpDir, "compose.yml")
	content := `
services:
  app:
    image: node:18-alpine
    environment:
      - DATABASE_URL=postgresql://user:pass@db:5432/test
  db:
    image: postgres:15-alpine
`
	if err := os.WriteFile(composeFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectProjectType()
	if err != nil {
		t.Fatalf("DetectProjectType returned error: %v", err)
	}

	if !res.RequiresDatabase {
		t.Errorf("expected RequiresDatabase to be true for postgres service, got false")
	}
}

func TestDetectProjectType_Dockerfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-dockerfile-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dockerfile := filepath.Join(tmpDir, "Dockerfile")
	content := `
FROM alpine:latest
# EXPOSE 9999 (commented out)
EXPOSE 8080
`
	if err := os.WriteFile(dockerfile, []byte(content), 0644); err != nil {
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
	if len(res.DetectedPorts) != 1 || res.DetectedPorts[0] != 8080 {
		t.Errorf("expected detected port [8080], got %v", res.DetectedPorts)
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
