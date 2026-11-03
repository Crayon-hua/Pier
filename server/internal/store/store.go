// Package store provides a small transactional, single-process JSON repository.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Admin struct {
	Username     string `json:"username"`
	PasswordHash string `json:"passwordHash"`
}
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Type        string    `json:"type"`
	Repository  string    `json:"repository"`
	Asset       string    `json:"asset"`
	ConfigYAML  string    `json:"configYaml"`
	GitHubToken string    `json:"githubToken,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}
type Task struct {
	ID          string     `json:"id"`
	ProjectID   string     `json:"projectId"`
	ProjectName string     `json:"projectName"`
	Kind        string     `json:"kind"`
	Target      string     `json:"target"`
	Status      string     `json:"status"`
	Force       bool       `json:"force"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	FinishedAt  *time.Time `json:"finishedAt,omitempty"`
}
type Data struct {
	Admin    *Admin             `json:"admin"`
	Projects map[string]Project `json:"projects"`
	Tasks    map[string]Task    `json:"tasks"`
}
type Store struct {
	mu   sync.RWMutex
	path string
	data Data
}

func Open(dir string) (*Store, error) {
	if e := os.MkdirAll(dir, 0700); e != nil {
		return nil, e
	}
	s := &Store{path: filepath.Join(dir, "pier.json"), data: Data{Projects: map[string]Project{}, Tasks: map[string]Task{}}}
	b, e := os.ReadFile(s.path)
	if e == nil {
		if e = json.Unmarshal(b, &s.data); e != nil {
			return nil, fmt.Errorf("read database: %w", e)
		}
	} else if !os.IsNotExist(e) {
		return nil, e
	}
	if s.data.Projects == nil {
		s.data.Projects = map[string]Project{}
	}
	if s.data.Tasks == nil {
		s.data.Tasks = map[string]Task{}
	}
	// The controller never guesses whether an interrupted deployment succeeded.
	e = s.Update(func(d *Data) error {
		for id, t := range d.Tasks {
			if t.Status == "running" || t.Status == "queued" {
				now := time.Now()
				t.Status = "interrupted"
				t.Error = "服务重启，任务结果待确认。请检查实际运行版本后重新操作。"
				t.FinishedAt = &now
				d.Tasks[id] = t
			}
		}
		return nil
	})
	return s, e
}
func (s *Store) Snapshot() Data {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, _ := json.Marshal(s.data)
	var d Data
	_ = json.Unmarshal(b, &d)
	return d
}
func (s *Store) Update(fn func(*Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, e := json.Marshal(s.data)
	if e != nil {
		return e
	}
	var next Data
	if e = json.Unmarshal(b, &next); e != nil {
		return e
	}
	if e = fn(&next); e != nil {
		return e
	}
	b, e = json.MarshalIndent(next, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(s.path), ".pier-*")
	if e != nil {
		return e
	}
	name := f.Name()
	defer os.Remove(name)
	if e = f.Chmod(0600); e != nil {
		f.Close()
		return e
	}
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	if e = os.Rename(name, s.path); e != nil {
		return e
	}
	s.data = next
	return nil
}
