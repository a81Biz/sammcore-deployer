package services

import (
	"context"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"sammcore-deployer/storage"
)

func TestDeployManager_ApplyManifestsYAML(t *testing.T) {
	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	sampleYAML := `
apiVersion: v1
kind: Namespace
metadata:
  name: test-app
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-app-web
  namespace: test-app
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-app-web
  template:
    metadata:
      labels:
        app: test-app-web
    spec:
      containers:
        - name: web
          image: nginx:alpine
---
apiVersion: v1
kind: Service
metadata:
  name: test-app-web
  namespace: test-app
spec:
  ports:
    - port: 80
`

	err := dm.ApplyManifestsYAML(ctx, sampleYAML)
	if err != nil {
		t.Fatalf("ApplyManifestsYAML returned error: %v", err)
	}

	// Verificar creación de Namespace
	_, err = client.CoreV1().Namespaces().Get(ctx, "test-app", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Namespace test-app was not created: %v", err)
	}

	// Verificar creación de Deployment
	_, err = client.AppsV1().Deployments("test-app").Get(ctx, "test-app-web", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Deployment test-app-web was not created: %v", err)
	}

	// Verificar creación de Service
	_, err = client.CoreV1().Services("test-app").Get(ctx, "test-app-web", metav1.GetOptions{})
	if err != nil {
		t.Errorf("Service test-app-web was not created: %v", err)
	}
}

func TestDeployManager_ExecuteDeploy_Static(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-dm-store-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	p := storage.Project{
		ID:        "static-proj",
		Name:      "static-proj",
		Repo:      "https://github.com/org/static-proj",
		Branch:    "main",
		Type:      "static",
		Namespace: "static-proj",
		Domain:    "static-proj.sammcore.local",
		Status:    storage.StatusAnalyzed,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	params := ProjectManifestParams{
		ProjectName: "static-proj",
		Namespace:   "static-proj",
		Type:        "static",
		Domain:      "static-proj.sammcore.local",
		StaticImage: "nginx:alpine",
	}

	err = dm.ExecuteDeploy(ctx, p, params)
	if err != nil {
		t.Fatalf("ExecuteDeploy failed: %v", err)
	}

	// Verificar que el deployment existe en el cliente fake
	dep, err := client.AppsV1().Deployments("static-proj").Get(ctx, "static-proj-static", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected deployment static-proj-static to exist: %v", err)
	}
	if dep.Name != "static-proj-static" {
		t.Errorf("expected deployment name static-proj-static, got %s", dep.Name)
	}
}

func TestDeployManager_DeleteProjectDeployment(t *testing.T) {
	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	// Crear namespace primero
	nsYAML := `
apiVersion: v1
kind: Namespace
metadata:
  name: to-delete
`
	_ = dm.ApplyManifestsYAML(ctx, nsYAML)

	// Eliminar
	err := dm.DeleteProjectDeployment(ctx, "to-delete", false)
	if err != nil {
		t.Fatalf("DeleteProjectDeployment failed: %v", err)
	}
}
