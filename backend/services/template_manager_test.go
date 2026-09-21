package services

import (
	"strings"
	"testing"
)

func TestTemplateManager_BaseManifests(t *testing.T) {
	tm := NewTemplateManager()
	params := ProjectManifestParams{
		ProjectName: "my-app",
		Namespace:   "my-app",
	}

	rendered, err := tm.RenderBaseManifests(params)
	if err != nil {
		t.Fatalf("failed to render base manifests: %v", err)
	}

	mustContain := []string{
		"kind: Namespace",
		"name: my-app",
		"pod-security.kubernetes.io/enforce: baseline",
		"kind: ResourceQuota",
		"name: my-app-quota",
		"kind: LimitRange",
		"name: my-app-limits",
		"kind: NetworkPolicy",
		"name: my-app-netpol",
	}

	for _, str := range mustContain {
		if !strings.Contains(rendered, str) {
			t.Errorf("base manifests missing required string: %q", str)
		}
	}
}

func TestTemplateManager_PerServiceCompose(t *testing.T) {
	tm := NewTemplateManager()
	params := ProjectManifestParams{
		ProjectName:      "backroom",
		Namespace:        "backroom",
		Type:             "compose",
		Domain:           "backroom.sammcore.local",
		APIDomain:        "backroom-api.sammcore.local",
		RequiresDatabase: true,
		Services: []ServiceSpec{
			{Name: "frontend", Role: RoleWeb, Port: 80, BuildContext: "./frontend", Dockerfile: "Dockerfile"},
			{Name: "backend", Role: RoleAPI, Port: 8080, BuildContext: "./backend", Dockerfile: "Dockerfile"},
			{Name: "worker", Role: RoleWorker, Port: 0, BuildContext: "./worker", Dockerfile: "Dockerfile"},
		},
		Images: map[string]string{
			"frontend": "localhost:30500/backroom/frontend:abc123",
			"backend":  "localhost:30500/backroom/backend:abc123",
			"worker":   "localhost:30500/backroom/worker:abc123",
		},
	}

	rendered, err := tm.RenderServiceManifests(params)
	if err != nil {
		t.Fatalf("failed to render service manifests: %v", err)
	}

	mustContain := []string{
		"name: backroom-frontend",
		"image: localhost:30500/backroom/frontend:abc123",
		"containerPort: 80",
		"name: backroom-backend",
		"image: localhost:30500/backroom/backend:abc123",
		"containerPort: 8080",
		"name: backroom-worker",
		"name: wait-for-db",
		"backroom-db-secrets",
		"host: backroom.sammcore.local",
		"host: backroom-api.sammcore.local",
		"number: 80",
		"number: 8080",
	}

	for _, str := range mustContain {
		if !strings.Contains(rendered, str) {
			t.Errorf("compose per-service manifest missing required string: %q\n\nFull output:\n%s", str, rendered)
		}
	}

	// El frontend (web) NO debe tener envFrom con db-secrets
	// Buscar el deployment del frontend y verificar que no tiene secretRef
	parts := strings.Split(rendered, "---")
	for _, part := range parts {
		if strings.Contains(part, "name: backroom-frontend") && strings.Contains(part, "kind: Deployment") {
			if strings.Contains(part, "db-secrets") {
				t.Errorf("frontend deployment should NOT have db-secrets envFrom")
			}
		}
	}

	// Verificar que NO hay imagePullSecrets
	if strings.Contains(rendered, "imagePullSecrets") {
		t.Errorf("should NOT contain imagePullSecrets (local registry)")
	}

	// Verificar que NO hay ghcr.io
	if strings.Contains(rendered, "ghcr.io") {
		t.Errorf("should NOT contain ghcr.io references")
	}

	// Worker NO debe tener Service (no tiene puerto)
	workerSvcFound := false
	for _, part := range parts {
		if strings.Contains(part, "kind: Service") && strings.Contains(part, "name: backroom-worker") {
			workerSvcFound = true
		}
	}
	if workerSvcFound {
		t.Errorf("worker should NOT have a Service (no port)")
	}

	// Verificar el alias "backend" Service
	if !strings.Contains(rendered, "name: backend\n") {
		t.Errorf("should contain backend alias Service")
	}
}

func TestTemplateManager_NoDBNoEnv(t *testing.T) {
	tm := NewTemplateManager()
	params := ProjectManifestParams{
		ProjectName:      "nodata-app",
		Namespace:        "nodata-app",
		Type:             "compose",
		Domain:           "nodata.sammcore.local",
		APIDomain:        "nodata-api.sammcore.local",
		RequiresDatabase: false,
		HasCustomEnv:     false,
		Services: []ServiceSpec{
			{Name: "web", Role: RoleWeb, Port: 3000},
			{Name: "api", Role: RoleAPI, Port: 4000},
		},
		Images: map[string]string{
			"web": "localhost:30500/nodata-app/web:def456",
			"api": "localhost:30500/nodata-app/api:def456",
		},
	}

	rendered, err := tm.RenderServiceManifests(params)
	if err != nil {
		t.Fatalf("failed to render manifests: %v", err)
	}

	if strings.Contains(rendered, "wait-for-db") {
		t.Errorf("expected no wait-for-db initContainer when RequiresDatabase is false")
	}
	if strings.Contains(rendered, "db-secrets") {
		t.Errorf("expected no db-secrets when RequiresDatabase is false")
	}
	if strings.Contains(rendered, "env-secrets") {
		t.Errorf("expected no env-secrets when HasCustomEnv is false")
	}
}

func TestTemplateManager_SingleAppDockerfile(t *testing.T) {
	tm := NewTemplateManager()
	params := ProjectManifestParams{
		ProjectName: "single-svc",
		Namespace:   "single-svc",
		Type:        "dockerfile",
		Domain:      "single-svc.sammcore.local",
		Services: []ServiceSpec{
			{Name: "app", Role: RoleApp, Port: 8080, BuildContext: ".", Dockerfile: "Dockerfile"},
		},
		Images: map[string]string{
			"app": "localhost:30500/single-svc/app:xyz789",
		},
	}

	rendered, err := tm.RenderServiceManifests(params)
	if err != nil {
		t.Fatalf("failed to render dockerfile manifests: %v", err)
	}

	if !strings.Contains(rendered, "name: single-svc-app") {
		t.Errorf("expected deployment single-svc-app")
	}
	if !strings.Contains(rendered, "containerPort: 8080") {
		t.Errorf("expected containerPort 8080")
	}
	if !strings.Contains(rendered, "localhost:30500/single-svc/app:xyz789") {
		t.Errorf("expected local registry image")
	}
	if !strings.Contains(rendered, "host: single-svc.sammcore.local") {
		t.Errorf("expected ingress host")
	}
}
