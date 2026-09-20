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

// === Tests para DetectServicePlan ===

func TestDetectServicePlan_Compose_BackroomLike(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-serviceplan-backroom-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Crear estructura similar a Backroom
	composeContent := `
services:
  db:
    image: postgres:15-alpine
    ports:
      - "5432:5432"
    environment:
      POSTGRES_DB: backroom
      POSTGRES_USER: user
      POSTGRES_PASSWORD: pass
  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    depends_on:
      - db
  worker:
    build:
      context: ./worker
      dockerfile: Dockerfile
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    ports:
      - "8443:443"
      - "80:80"
`
	if err := os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(composeContent), 0644); err != nil {
		t.Fatalf("failed to write compose file: %v", err)
	}

	// Crear Dockerfiles en los subdirectorios
	for _, dir := range []string{"backend", "worker", "frontend"} {
		os.MkdirAll(filepath.Join(tmpDir, dir), 0755)
	}
	os.WriteFile(filepath.Join(tmpDir, "backend", "Dockerfile"), []byte("FROM golang:1.24\nEXPOSE 8080\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "worker", "Dockerfile"), []byte("FROM python:3.12\n"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "frontend", "Dockerfile"), []byte("FROM node:20 AS build\nFROM nginx:alpine\nEXPOSE 80 443\n"), 0644)

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectServicePlan()
	if err != nil {
		t.Fatalf("DetectServicePlan returned error: %v", err)
	}

	if res.Type != ProjectCompose {
		t.Fatalf("expected ProjectCompose, got %v", res.Type)
	}

	// Debe haber 3 servicios (db filtrado)
	if len(res.Services) != 3 {
		t.Fatalf("expected 3 services (db filtered), got %d: %+v", len(res.Services), res.Services)
	}

	// Verificar que están ordenados por nombre: backend, frontend, worker
	expected := []struct {
		name string
		role ServiceRole
		port int
	}{
		{"backend", RoleAPI, 8080},
		{"frontend", RoleWeb, 80},
		{"worker", RoleWorker, 0},
	}

	for i, exp := range expected {
		svc := res.Services[i]
		if svc.Name != exp.name {
			t.Errorf("service[%d] name: expected %q, got %q", i, exp.name, svc.Name)
		}
		if svc.Role != exp.role {
			t.Errorf("service[%d] %s role: expected %v, got %v", i, exp.name, exp.role, svc.Role)
		}
		if svc.Port != exp.port {
			t.Errorf("service[%d] %s port: expected %d, got %d", i, exp.name, exp.port, svc.Port)
		}
		if svc.IsDB {
			t.Errorf("service[%d] %s should not be marked as DB", i, exp.name)
		}
	}

	// Verificar que RequiresDatabase se detectó
	if !res.RequiresDatabase {
		t.Errorf("expected RequiresDatabase to be true")
	}
}

func TestDetectServicePlan_Compose_PortFromDockerfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-serviceplan-port-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Compose sin ports definidos pero con Dockerfile que tiene EXPOSE
	composeContent := `
services:
  api:
    build: ./api
`
	os.WriteFile(filepath.Join(tmpDir, "docker-compose.yml"), []byte(composeContent), 0644)
	os.MkdirAll(filepath.Join(tmpDir, "api"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "api", "Dockerfile"), []byte("FROM golang:1.24\nEXPOSE 3000\nCMD [\"./server\"]\n"), 0644)

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectServicePlan()
	if err != nil {
		t.Fatalf("DetectServicePlan error: %v", err)
	}

	if len(res.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(res.Services))
	}

	svc := res.Services[0]
	if svc.Port != 3000 {
		t.Errorf("expected port 3000 from Dockerfile EXPOSE, got %d", svc.Port)
	}
	if svc.BuildContext != "./api" {
		t.Errorf("expected build_context './api', got %q", svc.BuildContext)
	}
}

func TestDetectServicePlan_Dockerfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-serviceplan-df-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte("FROM alpine\nEXPOSE 9090\n"), 0644)

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectServicePlan()
	if err != nil {
		t.Fatalf("DetectServicePlan error: %v", err)
	}

	if res.Type != ProjectDockerfile {
		t.Fatalf("expected ProjectDockerfile, got %v", res.Type)
	}
	if len(res.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(res.Services))
	}
	if res.Services[0].Name != "app" {
		t.Errorf("expected service name 'app', got %q", res.Services[0].Name)
	}
	if res.Services[0].Role != RoleApp {
		t.Errorf("expected role 'app', got %v", res.Services[0].Role)
	}
	if res.Services[0].Port != 9090 {
		t.Errorf("expected port 9090, got %d", res.Services[0].Port)
	}
}

func TestDetectServicePlan_Static(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-serviceplan-static-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.WriteFile(filepath.Join(tmpDir, "index.html"), []byte("<html></html>"), 0644)

	rm := NewRepoManager("", "", tmpDir, false)
	res, err := rm.DetectServicePlan()
	if err != nil {
		t.Fatalf("DetectServicePlan error: %v", err)
	}

	if res.Type != ProjectStatic {
		t.Fatalf("expected ProjectStatic, got %v", res.Type)
	}
	if len(res.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(res.Services))
	}
	if res.Services[0].Role != RoleWeb {
		t.Errorf("expected role 'web', got %v", res.Services[0].Role)
	}
	if res.Services[0].Port != 80 {
		t.Errorf("expected port 80, got %d", res.Services[0].Port)
	}
}

func TestInferServiceRole(t *testing.T) {
	tests := []struct {
		name     string
		expected ServiceRole
	}{
		{"frontend", RoleWeb},
		{"web-app", RoleWeb},
		{"ui", RoleWeb},
		{"client-panel", RoleWeb},
		{"backend", RoleAPI},
		{"api-gateway", RoleAPI},
		{"server", RoleAPI},
		{"worker", RoleWorker},
		{"queue-processor", RoleWorker},
		{"cron-jobs", RoleWorker},
		{"consumer", RoleWorker},
		{"unknown-thing", RoleAPI}, // default
	}

	for _, tt := range tests {
		got := inferServiceRole(tt.name)
		if got != tt.expected {
			t.Errorf("inferServiceRole(%q) = %v, want %v", tt.name, got, tt.expected)
		}
	}
}
