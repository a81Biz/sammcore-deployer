package services

import (
	"context"
	"os"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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
  labels:
    app.kubernetes.io/managed-by: sammcore-deployer
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

func TestDeployManager_ApplyManifestsYAML_ErrorOnUnknownKind(t *testing.T) {
	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	badYAML := `
apiVersion: v1
kind: SomeUnknownResource
metadata:
  name: should-fail
`

	err := dm.ApplyManifestsYAML(ctx, badYAML)
	if err == nil {
		t.Errorf("expected error for unknown kind, got nil")
	}
}

func TestDeployManager_ExecuteDeploy_PerService(t *testing.T) {
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
		ID:        "test-proj",
		Name:      "test-proj",
		Repo:      "https://github.com/org/test-proj",
		Branch:    "main",
		Type:      "compose",
		Namespace: "test-proj",
		Domain:    "test-proj.sammcore.local",
		APIDomain: "test-proj-api.sammcore.local",
		Status:    storage.StatusAnalyzed,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Sin commit → no ejecuta build pipeline, solo aplica manifiestos
	params := ProjectManifestParams{
		ProjectName: "test-proj",
		Namespace:   "test-proj",
		Type:        "compose",
		Domain:      "test-proj.sammcore.local",
		APIDomain:   "test-proj-api.sammcore.local",
		Services: []ServiceSpec{
			{Name: "web", Role: RoleWeb, Port: 80},
			{Name: "api", Role: RoleAPI, Port: 8080},
		},
		Images: map[string]string{
			"web": "localhost:30500/test-proj/web:abc123",
			"api": "localhost:30500/test-proj/api:abc123",
		},
	}

	err = dm.ExecuteDeploy(ctx, p, params)
	if err != nil {
		t.Fatalf("ExecuteDeploy failed: %v", err)
	}

	// Verificar creación de namespace con label managed-by
	ns, err := client.CoreV1().Namespaces().Get(ctx, "test-proj", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected namespace test-proj to exist: %v", err)
	}
	if ns.Labels["app.kubernetes.io/managed-by"] != "sammcore-deployer" {
		t.Errorf("expected managed-by label on namespace")
	}

	// Verificar deployments
	webDep, err := client.AppsV1().Deployments("test-proj").Get(ctx, "test-proj-web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected deployment test-proj-web: %v", err)
	}
	if webDep.Spec.Template.Spec.Containers[0].Image != "localhost:30500/test-proj/web:abc123" {
		t.Errorf("expected local registry image, got %s", webDep.Spec.Template.Spec.Containers[0].Image)
	}

	apiDep, err := client.AppsV1().Deployments("test-proj").Get(ctx, "test-proj-api", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected deployment test-proj-api: %v", err)
	}
	if apiDep.Spec.Template.Spec.Containers[0].Image != "localhost:30500/test-proj/api:abc123" {
		t.Errorf("expected local registry image, got %s", apiDep.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestDeployManager_DeleteProjectDeployment_OwnershipCheck(t *testing.T) {
	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	// Crear namespace CON label managed-by → debe permitir eliminar
	managedNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "managed-ns",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "sammcore-deployer",
			},
		},
	}
	_, _ = client.CoreV1().Namespaces().Create(ctx, managedNS, metav1.CreateOptions{})

	err := dm.DeleteProjectDeployment(ctx, "managed-ns", false)
	if err != nil {
		t.Fatalf("DeleteProjectDeployment should succeed for managed namespace: %v", err)
	}

	// Crear namespace SIN label managed-by → debe rechazar
	unmanagedNS := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "kube-system",
		},
	}
	_, _ = client.CoreV1().Namespaces().Create(ctx, unmanagedNS, metav1.CreateOptions{})

	err = dm.DeleteProjectDeployment(ctx, "kube-system", false)
	if err == nil {
		t.Errorf("DeleteProjectDeployment should fail for unmanaged namespace kube-system")
	}
}

func TestValidateProjectName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"backroom", false},
		{"my-app", false},
		{"app123", false},
		{"ab", true}, // too short
		{"abcdefghijklmnopqrstuvwxyz1234567890", true}, // too long
		{"My-App", true},            // uppercase
		{"-bad-start", true},        // starts with hyphen
		{"bad-end-", true},          // ends with hyphen
		{"has spaces", true},        // spaces
		{"kube-system", true},       // reserved prefix
		{"deployer", true},          // reserved name
		{"supabase", true},          // reserved name
		{"sammcore-registry", true}, // reserved name
		{"my-api", true},            // ends with -api
		{"my-docs", true},           // ends with -docs
	}

	for _, tt := range tests {
		err := ValidateProjectName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateProjectName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}

func TestDeployManager_Rollback_FirstDeploy(t *testing.T) {
	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	p := storage.Project{
		Name:      "test-app",
		Namespace: "test-app",
	}

	// Simular recursos creados durante un primer deploy que falló
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-app-web", Namespace: "test-app"},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "test-app-web", Namespace: "test-app"},
	}
	_, _ = client.AppsV1().Deployments("test-app").Create(ctx, dep, metav1.CreateOptions{})
	_, _ = client.CoreV1().Services("test-app").Create(ctx, svc, metav1.CreateOptions{})

	// Ejecutar rollback para primer deploy (isUpdate = false)
	dm.RollbackDeployment(ctx, p, "build failed", false, false, nil)

	// Verificar que los recursos fueron limpiados
	_, err := client.AppsV1().Deployments("test-app").Get(ctx, "test-app-web", metav1.GetOptions{})
	if err == nil {
		t.Errorf("expected deployment test-app-web to be deleted on first deploy rollback")
	}
	_, err = client.CoreV1().Services("test-app").Get(ctx, "test-app-web", metav1.GetOptions{})
	if err == nil {
		t.Errorf("expected service test-app-web to be deleted on first deploy rollback")
	}
}

func TestDeployManager_Rollback_UpdatePreApply(t *testing.T) {
	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	p := storage.Project{
		Name:      "test-app",
		Namespace: "test-app",
	}

	// Simular app en ejecución sana
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-app-web", Namespace: "test-app"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "web", Image: "localhost:30500/test-app/web:v1"}},
				},
			},
		},
	}
	_, _ = client.AppsV1().Deployments("test-app").Create(ctx, dep, metav1.CreateOptions{})

	previousImages := map[string]map[string]string{
		"test-app-web": {"web": "localhost:30500/test-app/web:v1"},
	}

	// Fallo antes de aplicar manifiestos en un redeploy (ej. build falló)
	dm.RollbackDeployment(ctx, p, "kaniko build failed", true, false, previousImages)

	// Los recursos DEBEN seguir existiendo intactos
	gotDep, err := client.AppsV1().Deployments("test-app").Get(ctx, "test-app-web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected deployment test-app-web to remain untouched on pre-apply failure: %v", err)
	}
	if gotDep.Spec.Template.Spec.Containers[0].Image != "localhost:30500/test-app/web:v1" {
		t.Errorf("expected original image preserved, got %s", gotDep.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestDeployManager_Rollback_UpdatePostApply(t *testing.T) {
	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	p := storage.Project{
		Name:      "test-app",
		Namespace: "test-app",
	}

	// Simular deployment que fue actualizado a v2 (pero falló en rollout)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-app-web", Namespace: "test-app"},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "web", Image: "localhost:30500/test-app/web:v2"}},
				},
			},
		},
	}
	_, _ = client.AppsV1().Deployments("test-app").Create(ctx, dep, metav1.CreateOptions{})

	previousImages := map[string]map[string]string{
		"test-app-web": {"web": "localhost:30500/test-app/web:v1"},
	}

	// Rollback post-apply debe revertir la imagen a v1 en lugar de borrar el deployment
	dm.RollbackDeployment(ctx, p, "rollout timeout", true, true, previousImages)

	gotDep, err := client.AppsV1().Deployments("test-app").Get(ctx, "test-app-web", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected deployment test-app-web to still exist after rollback: %v", err)
	}
	if gotDep.Spec.Template.Spec.Containers[0].Image != "localhost:30500/test-app/web:v1" {
		t.Errorf("expected image reverted to v1, got %s", gotDep.Spec.Template.Spec.Containers[0].Image)
	}
}

func TestDeployManager_PruneObsoleteResources(t *testing.T) {
	client := fake.NewSimpleClientset()
	dm := NewDeployManager(client)
	ctx := context.Background()

	labels := map[string]string{
		"app.kubernetes.io/managed-by": "sammcore-deployer",
		"project":                      "my-proj",
	}

	// Crear 2 deployments: uno activo y otro obsoleto (worker eliminado)
	depActive := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "my-proj-web", Namespace: "my-proj", Labels: labels},
	}
	depObsolete := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "my-proj-worker", Namespace: "my-proj", Labels: labels},
	}
	_, _ = client.AppsV1().Deployments("my-proj").Create(ctx, depActive, metav1.CreateOptions{})
	_, _ = client.AppsV1().Deployments("my-proj").Create(ctx, depObsolete, metav1.CreateOptions{})

	desiredDeployments := map[string]bool{"my-proj-web": true}
	desiredServices := map[string]bool{"my-proj-web": true}

	dm.pruneObsoleteResources(ctx, "my-proj", "my-proj", desiredDeployments, desiredServices)

	// El activo debe persistir
	_, err := client.AppsV1().Deployments("my-proj").Get(ctx, "my-proj-web", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected active deployment to remain: %v", err)
	}

	// El obsoleto debe ser eliminado
	_, err = client.AppsV1().Deployments("my-proj").Get(ctx, "my-proj-worker", metav1.GetOptions{})
	if err == nil {
		t.Errorf("expected obsolete deployment my-proj-worker to be pruned")
	}
}
