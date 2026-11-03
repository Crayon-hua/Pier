package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	engineconfig "pier/internal/engine/config"
	"pier/internal/engine/state"
	"pier/internal/store"
)

var slug = regexp.MustCompile(`^[a-z][a-z0-9-]{1,47}$`)
var repo = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type Service struct {
	Store   *store.Store
	DataDir string
	RootDir string
}

func (s *Service) Root(id string) string {
	if s.RootDir != "" {
		return filepath.Join(s.RootDir, id)
	}
	return filepath.Join(s.DataDir, "projects", id)
}
func (s *Service) Get(id string) (store.Project, error) {
	p, ok := s.Store.Snapshot().Projects[id]
	if !ok {
		return p, errors.New("项目不存在")
	}
	return p, nil
}
func (s *Service) Busy(id string) bool {
	for _, t := range s.Store.Snapshot().Tasks {
		if t.ProjectID == id && (t.Status == "running" || t.Status == "queued") {
			return true
		}
	}
	return false
}

// Config returns the server-side deployment configuration. Package manifests still
// determine executable commands; they must agree with the registered project type.
func Config(p store.Project) ([]byte, error) {
	cfg := engineconfig.ProjectConfig{}
	if strings.TrimSpace(p.ConfigYAML) != "" {
		if e := yaml.Unmarshal([]byte(p.ConfigYAML), &cfg); e != nil {
			return nil, fmt.Errorf("高级 YAML 无效: %w", e)
		}
	}
	cfg.Name = p.ID
	if p.Repository != "" {
		cfg.Release = &engineconfig.ProjectRelease{Provider: "github", Repository: p.Repository, Asset: p.Asset}
	} else {
		cfg.Release = nil
	}
	if cfg.BundlesKeep() < 0 || cfg.BundlesKeep() > 100 {
		return nil, errors.New("bundles.keep 需在 0–100 之间")
	}
	if cfg.Backup != nil {
		if cfg.Backup.Dir != "" && cfg.Backup.Dir != "backups" {
			return nil, errors.New("第一版备份目录固定为 backups")
		}
		if filepath.IsAbs(cfg.Backup.Script) || strings.Contains(filepath.ToSlash(cfg.Backup.Script), "..") {
			return nil, errors.New("备份脚本必须在项目目录内")
		}
	}
	return yaml.Marshal(cfg)
}
func (s *Service) Save(p store.Project, create bool, clearToken bool) (store.Project, error) {
	p.ID = strings.TrimSpace(p.ID)
	p.Name = strings.TrimSpace(p.Name)
	p.Repository = strings.TrimSpace(p.Repository)
	if !slug.MatchString(p.ID) {
		return p, errors.New("项目标识需为 2–48 位小写字母、数字或连字符，以字母开头")
	}
	if p.Name == "" || len(p.Name) > 100 {
		return p, errors.New("请填写项目名称（最多 100 字节）")
	}
	if p.Type != "static" && p.Type != "docker" && p.Type != "system" {
		return p, errors.New("不支持的项目类型")
	}
	if p.Repository != "" && !repo.MatchString(p.Repository) {
		return p, errors.New("仓库格式应为 owner/repo")
	}
	if p.Asset == "" {
		p.Asset = "bundle.zip"
	}
	if _, e := Config(p); e != nil {
		return p, e
	}
	if st, e := state.Read(s.Root(p.ID)); e != nil {
		return p, e
	} else if st.CurrentVersion != "" && st.Type != "" && st.Type != p.Type {
		return p, errors.New("项目已有部署，不能更改部署类型")
	}
	e := s.Store.Update(func(d *store.Data) error {
		old, exists := d.Projects[p.ID]
		if create && exists {
			return errors.New("项目标识已存在")
		}
		if !create && !exists {
			return errors.New("项目不存在")
		}
		for _, t := range d.Tasks {
			if t.ProjectID == p.ID && (t.Status == "running" || t.Status == "queued") {
				return errors.New("项目正在执行任务，请稍后修改")
			}
		}
		if exists {
			p.CreatedAt = old.CreatedAt
			if p.GitHubToken == "" && !clearToken {
				p.GitHubToken = old.GitHubToken
			}
		} else {
			p.CreatedAt = time.Now()
		}
		d.Projects[p.ID] = p
		return nil
	})
	return p, e
}
func (s *Service) Remove(id string) error {
	return s.Store.Update(func(d *store.Data) error {
		if _, ok := d.Projects[id]; !ok {
			return errors.New("项目不存在")
		}
		for _, t := range d.Tasks {
			if t.ProjectID == id && (t.Status == "running" || t.Status == "queued") {
				return errors.New("项目正在执行任务")
			}
		}
		delete(d.Projects, id)
		return nil
	})
}
func (s *Service) WriteConfig(p store.Project) error {
	b, e := Config(p)
	if e != nil {
		return e
	}
	root := s.Root(p.ID)
	if e = os.MkdirAll(root, 0755); e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(root, "deploy.yaml"), b, 0600)
}
