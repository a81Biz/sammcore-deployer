package storage

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

type Project struct {
	ID        string    `json:"id"`
	Repo      string    `json:"repo"`
	Branch    string    `json:"branch"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

var mu sync.Mutex
var historyFile = "history.json"

func LoadProjects() ([]Project, error) {
	mu.Lock()
	defer mu.Unlock()

	if _, err := os.Stat(historyFile); os.IsNotExist(err) {
		return []Project{}, nil
	}

	data, err := os.ReadFile(historyFile)
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

	return os.WriteFile(historyFile, data, 0644)
}

func AddProject(p Project) error {
	projects, _ := LoadProjects()
	projects = append(projects, p)
	return SaveProjects(projects)
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
