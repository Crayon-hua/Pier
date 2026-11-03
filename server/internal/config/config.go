package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Listen       string `yaml:"listen"`
	DataDir      string `yaml:"data_dir"`
	ProjectsDir  string `yaml:"projects_dir"`
	WebDir       string `yaml:"web_dir"`
	SecureCookie bool   `yaml:"secure_cookie"`
	MaxUploadMB  int64  `yaml:"max_upload_mb"`
}

func Load(path string) (Config, error) {
	c := Config{Listen: "127.0.0.1:8099", DataDir: "./data", ProjectsDir: "./apps", WebDir: "../web/dist", MaxUploadMB: 256}
	if path != "" {
		b, e := os.ReadFile(path)
		if e != nil {
			return c, e
		}
		if e = yaml.Unmarshal(b, &c); e != nil {
			return c, e
		}
	}
	if v := os.Getenv("PIER_LISTEN"); v != "" {
		c.Listen = v
	}
	if v := os.Getenv("PIER_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("PIER_WEB_DIR"); v != "" {
		c.WebDir = v
	}
	if c.MaxUploadMB < 1 || c.MaxUploadMB > 4096 {
		return c, fmt.Errorf("max_upload_mb must be between 1 and 4096")
	}
	if v := os.Getenv("PIER_PROJECTS_DIR"); v != "" {
		c.ProjectsDir = v
	}
	var e error
	c.DataDir, e = filepath.Abs(c.DataDir)
	if e != nil {
		return c, e
	}
	c.ProjectsDir, e = filepath.Abs(c.ProjectsDir)
	return c, e
}
