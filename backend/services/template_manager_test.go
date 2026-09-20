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
		"matchLabels:\n              kubernetes.io/metadata.name: ingress-nginx",
		"matchLabels:\n              kubernetes.io/metadata.name: supabase",
	}

	for _, str := range mustContain {
		if !strings.Contains(rendered, str) {
			t.Errorf("base manifests missing required string: %q", str)
		}
	}
}

func TestTemplateManager_ComposeWithDatabase(t *testing.T) {
	tm := NewTemplateManager()
	params := ProjectManifestParams{
		ProjectName:      "backroom",
		Namespace:        "backroom",
		Type:             "compose",
		Domain:           "backroom.sammcore.local",
		APIDomain:        "backroom-api.sammcore.local",
		RequiresDatabase: true,
		WebImage:         "ghcr.io/a81biz/backroom-web:latest",
		WebPort:          80,
		APIImage:         "ghcr.io/a81biz/backroom-api:latest",
		APIPort:          8000,
	}

	rendered, err := tm.RenderAppManifests(params)
	if err != nil {
		t.Fatalf("failed to render compose manifests: %v", err)
	}

	mustContain := []string{
		"name: backroom-web",
		"image: ghcr.io/a81biz/backroom-web:latest",
		"name: backroom-api",
		"image: ghcr.io/a81biz/backroom-api:latest",
		"name: sammcore-registry-secret",
		"name: wait-for-db",
		"name: backroom-db-secrets",
		"host: backroom.sammcore.local",
		"host: backroom-api.sammcore.local",
	}

	for _, str := range mustContain {
		if !strings.Contains(rendered, str) {
			t.Errorf("compose manifest missing required string: %q", str)
		}
	}
}

func TestTemplateManager_ComposeWithoutDatabase(t *testing.T) {
	tm := NewTemplateManager()
	params := ProjectManifestParams{
		ProjectName:      "nodata-app",
		Namespace:        "nodata-app",
		Type:             "compose",
		Domain:           "nodata.sammcore.local",
		APIDomain:        "nodata-api.sammcore.local",
		RequiresDatabase: false,
		WebImage:         "web:latest",
		WebPort:          80,
		APIImage:         "api:latest",
		APIPort:          3000,
	}

	rendered, err := tm.RenderAppManifests(params)
	if err != nil {
		t.Fatalf("failed to render compose manifests: %v", err)
	}

	if strings.Contains(rendered, "name: wait-for-db") {
		t.Errorf("expected no wait-for-db initContainer when RequiresDatabase is false")
	}
	if strings.Contains(rendered, "name: nodata-app-db-secrets") {
		t.Errorf("expected no secretRef when RequiresDatabase is false")
	}
}

func TestTemplateManager_DockerfileAndStatic(t *testing.T) {
	tm := NewTemplateManager()

	// 1. Dockerfile
	dfParams := ProjectManifestParams{
		ProjectName: "single-svc",
		Namespace:   "single-svc",
		Type:        "dockerfile",
		Domain:      "single-svc.sammcore.local",
		AppImage:    "my-service:1.0",
		AppPort:     8080,
	}

	dfRendered, err := tm.RenderAppManifests(dfParams)
	if err != nil {
		t.Fatalf("failed to render dockerfile manifests: %v", err)
	}
	if !strings.Contains(dfRendered, "name: single-svc-app") {
		t.Errorf("expected deployment single-svc-app")
	}

	// 2. Static
	staticParams := ProjectManifestParams{
		ProjectName: "landing-page",
		Namespace:   "landing-page",
		Type:        "static",
		Domain:      "landing.sammcore.local",
		StaticImage: "nginx:alpine",
	}

	staticRendered, err := tm.RenderAppManifests(staticParams)
	if err != nil {
		t.Fatalf("failed to render static manifests: %v", err)
	}
	if !strings.Contains(staticRendered, "name: landing-page-static") {
		t.Errorf("expected deployment landing-page-static")
	}
}
