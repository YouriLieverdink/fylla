package local

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type localTask struct {
	ID          int         `yaml:"id"`
	Summary     string      `yaml:"summary"`
	Project     string      `yaml:"project,omitempty"`
	Section     string      `yaml:"section,omitempty"`
	Priority    int         `yaml:"priority"`
	DueDate     string      `yaml:"dueDate,omitempty"`
	Estimate    string      `yaml:"estimate,omitempty"`
	Description string      `yaml:"description,omitempty"`
	Created     time.Time   `yaml:"created"`
	Completed   bool        `yaml:"completed"`
	Recurrence  *recurrence `yaml:"recurrence,omitempty"`
}

type recurrence struct {
	Freq string `yaml:"freq"`
	Days []int  `yaml:"days,omitempty"`
}

type store struct {
	NextID int         `yaml:"nextID"`
	Tasks  []localTask `yaml:"tasks"`
}

func defaultStorePath() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "fylla", "local_tasks.yaml"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "fylla", "local_tasks.yaml"), nil
}

func loadStore(path string) (*store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &store{NextID: 1}, nil
		}
		return nil, fmt.Errorf("read store: %w", err)
	}
	var s store
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse store: %w", err)
	}
	if s.NextID == 0 {
		s.NextID = 1
	}
	return &s, nil
}

func saveStore(path string, s *store) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create store dir: %w", err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal store: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func findByID(s *store, id int) *localTask {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			return &s.Tasks[i]
		}
	}
	return nil
}

func removeByID(s *store, id int) bool {
	for i := range s.Tasks {
		if s.Tasks[i].ID == id {
			s.Tasks = append(s.Tasks[:i], s.Tasks[i+1:]...)
			return true
		}
	}
	return false
}
