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

func LoadProjects() ([]Project, error) {
	mu.Lock()
	defer mu.Unlock()

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

func SaveProjects(projects []Project) error {
	mu.Lock()
	defer mu.Unlock()

	data, err := json.MarshalIndent(projects, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(getHistoryFile(), data, 0644)
}

func AddProject(p Project) error {
	projects, _ := LoadProjects()
	projects = append(projects, p)
	return SaveProjects(projects)
}

func GetProject(id string) (*Project, error) {
	projects, _ := LoadProjects()
	for _, p := range projects {
		if p.ID == id {
			return &p, nil
		}
	}
	return nil, errors.New("proyecto no encontrado")
}

func DeleteProject(id string) error {
	projects, _ := LoadProjects()
	filtered := []Project{}
	for _, p := range projects {
		if p.ID != id {
			filtered = append(filtered, p)
		}
	}
	return SaveProjects(filtered)
}
