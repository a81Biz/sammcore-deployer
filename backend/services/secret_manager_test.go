package services

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestSecretManager_EnsureAndGetPassword(t *testing.T) {
	client := fake.NewSimpleClientset()
	sm := NewSecretManager(client)
	ctx := context.Background()

	namespace := "test-ns"
	projectName := "test-proj"

	// 1. Verificar que al inicio no hay contraseña
	pass, err := sm.GetExistingDBPassword(ctx, namespace, projectName)
	if err != nil {
		t.Fatalf("unexpected error getting password: %v", err)
	}
	if pass != "" {
		t.Fatalf("expected empty password, got %s", pass)
	}

	// 2. Crear Secret
	provisionRes := &DBProvisionResult{
		Host:        "postgres.supabase.svc.cluster.local",
		Port:        5432,
		Database:    "test_proj_db",
		Username:    "test_proj_user",
		Password:    "super-secret-pw-12345",
		DatabaseURL: "postgresql://test_proj_user:super-secret-pw-12345@postgres.supabase.svc.cluster.local:5432/test_proj_db?sslmode=disable",
	}

	err = sm.EnsureDBSecret(ctx, namespace, projectName, provisionRes)
	if err != nil {
		t.Fatalf("failed to ensure db secret: %v", err)
	}

	// 3. Recuperar contraseña existente
	savedPass, err := sm.GetExistingDBPassword(ctx, namespace, projectName)
	if err != nil {
		t.Fatalf("failed to get existing password: %v", err)
	}
	if savedPass != "super-secret-pw-12345" {
		t.Errorf("expected password 'super-secret-pw-12345', got %q", savedPass)
	}

	// 4. Eliminar Secret
	err = sm.DeleteDBSecret(ctx, namespace, projectName)
	if err != nil {
		t.Fatalf("failed to delete db secret: %v", err)
	}

	// 5. Verificar que ya no existe
	afterDeletePass, err := sm.GetExistingDBPassword(ctx, namespace, projectName)
	if err != nil {
		t.Fatalf("unexpected error after delete: %v", err)
	}
	if afterDeletePass != "" {
		t.Errorf("expected empty password after deletion, got %q", afterDeletePass)
	}
}
