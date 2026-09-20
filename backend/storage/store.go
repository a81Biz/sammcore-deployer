package storage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type ProjectStatus string

const (
	StatusAnalyzed     ProjectStatus = "analyzed"
	StatusProvisioning ProjectStatus = "provisioning_db"
	StatusBuilding     ProjectStatus = "building_image"
	StatusDeploying    ProjectStatus = "deploying_k8s"
	StatusRunning      ProjectStatus = "running"
	StatusFailed       ProjectStatus = "failed"
)

type Project struct {
	ID               string        `json:"id"`
	Name             string        `json:"name"`
	Repo             string        `json:"repo"`
	Branch           string        `json:"branch"`
	Type             string        `json:"type"`
	Namespace        string        `json:"namespace"`
	Domain           string        `json:"domain"`
	APIDomain        string        `json:"api_domain,omitempty"`
	RequiresDatabase bool          `json:"requires_database"`
	Status           ProjectStatus `json:"status"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

var mu sync.Mutex

func getHistoryFile() string {
	dir := os.Getenv("DATA_DIR")
	if dir == "" {
		dir = "."
	}
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "history.json")
}

// loadProjectsUnsafe lee sin bloquear; debe ser llamado con mu.Lock activo
func loadProjectsUnsafe() ([]Project, error) {
	filePath := getHistoryFile()
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return []Project{}, nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var projects []Project
	if err := json.Unmarshal(data, &projects); err != nil {
		return nil, err
	}

	return projects, nil
}

// saveProjectsAtomic guarda de forma atómica usando archivo temporal y rename
func saveProjectsAtomic(projects []Project) error {
	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}

	targetFile := getHistoryFile()
	tmpFile := targetFile + ".tmp"

	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}

	// Rename es atómico en POSIX y reemplaza el archivo destino de forma segura
	return os.Rename(tmpFile, targetFile)
}

func LoadProjects() ([]Project, error) {
	mu.Lock()
	defer mu.Unlock()
	return loadProjectsUnsafe()
}

func SaveProjects(projects []Project) error {
	mu.Lock()
	defer mu.Unlock()
	return saveProjectsAtomic(projects)
}

func AddOrUpdateProject(p Project) error {
	mu.Lock()
	defer mu.Unlock()

	projects, err := loadProjectsUnsafe()
	if err != nil {
		projects = []Project{}
	}

	updated := false
	for i, existing := range projects {
		if existing.ID == p.ID || existing.Repo == p.Repo {
			p.ID = existing.ID
			p.CreatedAt = existing.CreatedAt
			p.UpdatedAt = time.Now()
			projects[i] = p
			updated = true
			break
		}
	}

	if !updated {
		projects = append(projects, p)
	}

	return saveProjectsAtomic(projects)
}

func GetProject(id string) (*Project, error) {
	mu.Lock()
	defer mu.Unlock()

	projects, err := loadProjectsUnsafe()
	if err != nil {
		return nil, err
	}

	for _, p := range projects {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, errors.New("proyecto no encontrado")
}

func DeleteProject(id string) error {
	mu.Lock()
	defer mu.Unlock()

	projects, err := loadProjectsUnsafe()
	if err != nil {
		return err
	}

	filtered := []Project{}
	for _, p := range projects {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}

	return saveProjectsAtomic(filtered)
}
