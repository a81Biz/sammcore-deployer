package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStore_AddGetDelete(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-store-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	p := Project{
		ID:        "test-proj-1",
		Name:      "test-proj",
		Repo:      "https://github.com/org/test-proj",
		Branch:    "main",
		Type:      "static",
		Namespace: "test-proj",
		Domain:    "test-proj.sammcore.local",
		Status:    StatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := AddOrUpdateProject(p); err != nil {
		t.Fatalf("AddOrUpdateProject failed: %v", err)
	}

	loaded, err := GetProject("test-proj-1")
	if err != nil {
		t.Fatalf("GetProject failed: %v", err)
	}
	if loaded.Name != "test-proj" {
		t.Errorf("expected name test-proj, got %s", loaded.Name)
	}

	if err := DeleteProject("test-proj-1"); err != nil {
		t.Fatalf("DeleteProject failed: %v", err)
	}

	_, err = GetProject("test-proj-1")
	if err == nil {
		t.Errorf("expected error when getting deleted project, got nil")
	}
}

func TestStore_CorruptBackup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "test-corrupt-store-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.Setenv("DATA_DIR", tmpDir)
	defer os.Unsetenv("DATA_DIR")

	// Write invalid JSON into history.json
	histFile := filepath.Join(tmpDir, "history.json")
	corruptContent := []byte("{ this is not valid json --- broken")
	if err := os.WriteFile(histFile, corruptContent, 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	// Loading should fail and produce a backup
	_, err = LoadProjects()
	if err == nil {
		t.Fatalf("expected error loading corrupt JSON, got nil")
	}

	// Check if .corrupt.* file was created
	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}

	foundCorruptBackup := false
	for _, f := range files {
		if filepath.Ext(f.Name()) != "" && filepath.Base(f.Name()) != "history.json" {
			foundCorruptBackup = true
			break
		}
	}

	if !foundCorruptBackup {
		t.Errorf("expected history.json.corrupt.<ts> backup file to be created")
	}

	// Attempting to AddOrUpdateProject should NOT overwrite corrupt file
	p := Project{ID: "new-proj", Name: "new-proj"}
	err = AddOrUpdateProject(p)
	if err == nil {
		t.Errorf("expected AddOrUpdateProject to fail instead of overwriting corrupt data, got nil")
	}
}
