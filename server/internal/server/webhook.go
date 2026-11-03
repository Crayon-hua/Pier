package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
	ec "pier/internal/engine/config"
)

// A webhook is disabled until a project-specific secret has been configured.
// GitHub release events are supported; other CI systems may send {"target":"v1"}.
func (a *App) webhook(w http.ResponseWriter, r *http.Request) {
	p, e := a.Projects.Get(r.PathValue("id"))
	if e != nil {
		fail(w, 404, e)
		return
	}
	var cfg ec.ProjectConfig
	if e = yaml.Unmarshal([]byte(p.ConfigYAML), &cfg); e != nil {
		fail(w, 400, errors.New("项目配置无效"))
		return
	}
	secret := cfg.InboundSecret()
	if len(secret) < 24 {
		fail(w, 403, errors.New("Webhook 未启用，需配置至少 24 字节的 webhook.secret"))
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	body, e := io.ReadAll(r.Body)
	if e != nil {
		fail(w, 400, errors.New("Webhook 内容过大"))
		return
	}
	authenticated := false
	if signature := r.Header.Get("X-Hub-Signature-256"); strings.HasPrefix(signature, "sha256=") {
		got, e := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		authenticated = e == nil && hmac.Equal(got, mac.Sum(nil))
	} else {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		authenticated = subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
	}
	if !authenticated {
		fail(w, 401, errors.New("Webhook 鉴权失败"))
		return
	}
	if r.Header.Get("X-GitHub-Event") == "ping" {
		write(w, 200, map[string]bool{"ok": true})
		return
	}
	var target string
	force := false
	if event := r.Header.Get("X-GitHub-Event"); event != "" {
		if event != "release" {
			write(w, 200, map[string]string{"message": "忽略非 Release 事件"})
			return
		}
		var in struct {
			Action  string `json:"action"`
			Release struct {
				Tag        string `json:"tag_name"`
				Draft      bool   `json:"draft"`
				Prerelease bool   `json:"prerelease"`
			} `json:"release"`
		}
		if e = json.Unmarshal(body, &in); e != nil {
			fail(w, 400, errors.New("Webhook JSON 无效"))
			return
		}
		if in.Action != "published" || in.Release.Draft || in.Release.Prerelease {
			write(w, 200, map[string]string{"message": "忽略非正式发布事件"})
			return
		}
		target = in.Release.Tag
	} else {
		var in struct {
			Target string `json:"target"`
			Force  bool   `json:"force"`
		}
		if len(body) > 0 {
			if e = json.Unmarshal(body, &in); e != nil {
				fail(w, 400, errors.New("Webhook JSON 无效"))
				return
			}
		}
		target = in.Target
		force = in.Force
	}
	if target == "" {
		target = "latest"
	}
	t, e := a.Tasks.Start(p.ID, "deploy", target, "", force)
	if e != nil {
		fail(w, 409, e)
		return
	}
	write(w, 202, t)
}
