package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"pier/internal/auth"
	"pier/internal/config"
	"pier/internal/engine/backup"
	"pier/internal/engine/bundle"
	"pier/internal/engine/cleaner"
	"pier/internal/engine/state"
	"pier/internal/project"
	"pier/internal/store"
	"pier/internal/task"
	"pier/internal/webembed"
)

type App struct {
	HTTP     *http.Server
	Store    *store.Store
	Auth     *auth.Service
	Projects *project.Service
	Tasks    *task.Service
	Config   config.Config
}

func New(c config.Config) (*App, error) {
	st, e := store.Open(c.DataDir)
	if e != nil {
		return nil, e
	}
	p := &project.Service{Store: st, DataDir: c.DataDir, RootDir: c.ProjectsDir}
	a := &App{Store: st, Projects: p, Auth: auth.New(st, c.SecureCookie), Config: c}
	a.Tasks = task.New(st, p, c.DataDir)
	if !a.Auth.Initialized() && os.Getenv("PIER_ADMIN_PASSWORD") != "" {
		name := os.Getenv("PIER_ADMIN_USERNAME")
		if name == "" {
			name = "admin"
		}
		if e = a.Auth.Setup(name, os.Getenv("PIER_ADMIN_PASSWORD")); e != nil {
			return nil, e
		}
	}
	a.HTTP = &http.Server{Addr: c.Listen, Handler: a.routes(), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	return a, nil
}
func (a *App) Listen() error {
	e := a.HTTP.ListenAndServe()
	if errors.Is(e, http.ErrServerClosed) {
		return nil
	}
	return e
}
func write(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
func fail(w http.ResponseWriter, status int, e error) {
	write(w, status, map[string]string{"message": e.Error()})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if e := d.Decode(v); e != nil {
		fail(w, 400, errors.New("请求格式无效"))
		return false
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		fail(w, 400, errors.New("请求只能包含一个 JSON 对象"))
		return false
	}
	return true
}
func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { write(w, 200, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/v1/auth/status", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]bool{"initialized": a.Auth.Initialized()})
	})
	mux.HandleFunc("POST /api/v1/webhooks/{id}", a.webhook)
	mux.HandleFunc("POST /api/v1/auth/setup", func(w http.ResponseWriter, r *http.Request) {
		if !a.Auth.Allow(r.RemoteAddr) {
			fail(w, 429, errors.New("请求过于频繁，请稍后重试"))
			return
		}
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !decode(w, r, &in) {
			return
		}
		if e := a.Auth.Setup(in.Username, in.Password); e != nil {
			fail(w, 400, e)
			return
		}
		write(w, 201, map[string]bool{"ok": true})
	})
	mux.HandleFunc("POST /api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		if !a.Auth.Allow(r.RemoteAddr) {
			fail(w, 429, errors.New("登录尝试过多，请一分钟后重试"))
			return
		}
		var in struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if !decode(w, r, &in) {
			return
		}
		if e := a.Auth.Login(w, in.Username, in.Password); e != nil {
			fail(w, 401, e)
			return
		}
		write(w, 200, map[string]string{"username": in.Username})
	})
	protect := func(pattern string, h http.HandlerFunc) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			if !a.Auth.Valid(r) {
				fail(w, 401, errors.New("请先登录"))
				return
			}
			h(w, r)
		})
	}
	protect("GET /api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]string{"username": a.Store.Snapshot().Admin.Username})
	})
	protect("POST /api/v1/auth/logout", func(w http.ResponseWriter, r *http.Request) {
		a.Auth.Logout(w, r)
		write(w, 200, map[string]bool{"ok": true})
	})
	protect("POST /api/v1/auth/password", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			Old string `json:"oldPassword"`
			New string `json:"newPassword"`
		}
		if !decode(w, r, &in) {
			return
		}
		if e := a.Auth.ChangePassword(in.Old, in.New); e != nil {
			fail(w, 400, e)
			return
		}
		a.Auth.Logout(w, r)
		write(w, 200, map[string]bool{"ok": true})
	})
	protect("GET /api/v1/system", func(w http.ResponseWriter, r *http.Request) {
		write(w, 200, map[string]any{"os": runtime.GOOS, "arch": runtime.GOARCH, "dataDir": a.Config.DataDir, "version": "0.1.0", "maxUploadMB": a.Config.MaxUploadMB})
	})
	protect("GET /api/v1/projects", a.listProjects)
	protect("POST /api/v1/projects", a.saveProject)
	protect("GET /api/v1/projects/{id}", a.getProject)
	protect("PUT /api/v1/projects/{id}", a.saveProject)
	protect("DELETE /api/v1/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		if e := a.Projects.Remove(r.PathValue("id")); e != nil {
			fail(w, 409, e)
			return
		}
		write(w, 200, map[string]bool{"ok": true})
	})
	protect("GET /api/v1/projects/{id}/releases", func(w http.ResponseWriter, r *http.Request) {
		p, e := a.Projects.Get(r.PathValue("id"))
		if e != nil {
			fail(w, 404, e)
			return
		}
		data, e := a.Tasks.Releases(r.Context(), p)
		if e != nil {
			fail(w, 502, e)
			return
		}
		write(w, 200, data)
	})
	protect("POST /api/v1/projects/{id}/tasks", a.startTask)
	protect("POST /api/v1/projects/{id}/upload", a.upload)
	protect("GET /api/v1/projects/{id}/backups/{name}", a.downloadBackup)
	protect("GET /api/v1/tasks", a.listTasks)
	protect("GET /api/v1/tasks/{id}", a.getTask)
	protect("GET /api/v1/tasks/{id}/logs", a.taskLogs)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { fail(w, 404, errors.New("API 不存在")) })
	mux.Handle("/", a.static())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if e := recover(); e != nil {
				log.Printf("HTTP panic: %v", e)
				fail(w, 500, errors.New("服务内部错误"))
			}
		}()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" {
			origin := r.Header.Get("Origin")
			if origin != "" {
				u, e := url.Parse(origin)
				if e != nil || u.Host != r.Host {
					fail(w, 403, errors.New("请求来源不匹配"))
					return
				}
			}
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				fail(w, 403, errors.New("不允许跨站请求"))
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}

func public(p store.Project) map[string]any {
	return map[string]any{"id": p.ID, "name": p.Name, "description": p.Description, "type": p.Type, "repository": p.Repository, "asset": p.Asset, "configYaml": p.ConfigYAML, "hasToken": p.GitHubToken != "", "createdAt": p.CreatedAt}
}
func (a *App) view(p store.Project) map[string]any {
	v := public(p)
	v["root"] = a.Projects.Root(p.ID)
	st, e := state.Read(a.Projects.Root(p.ID))
	if e != nil {
		v["stateError"] = e.Error()
	} else {
		v["state"] = st
	}
	v["busy"] = a.Projects.Busy(p.ID)
	v["diskBytes"] = dirSize(a.Projects.Root(p.ID))
	return v
}
func (a *App) listProjects(w http.ResponseWriter, r *http.Request) {
	list := []map[string]any{}
	for _, p := range a.Store.Snapshot().Projects {
		list = append(list, a.view(p))
	}
	sort.Slice(list, func(i, j int) bool { return list[i]["id"].(string) < list[j]["id"].(string) })
	write(w, 200, list)
}
func (a *App) saveProject(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ID          string `json:"id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		Repository  string `json:"repository"`
		Asset       string `json:"asset"`
		ConfigYAML  string `json:"configYaml"`
		GitHubToken string `json:"githubToken"`
		ClearToken  bool   `json:"clearToken"`
	}
	if !decode(w, r, &in) {
		return
	}
	create := r.Method == "POST"
	if !create {
		in.ID = r.PathValue("id")
	}
	p, e := a.Projects.Save(store.Project{ID: in.ID, Name: in.Name, Description: in.Description, Type: in.Type, Repository: in.Repository, Asset: in.Asset, ConfigYAML: in.ConfigYAML, GitHubToken: in.GitHubToken}, create, in.ClearToken)
	if e != nil {
		fail(w, 400, e)
		return
	}
	status := 200
	if create {
		status = 201
	}
	write(w, status, a.view(p))
}
func (a *App) getProject(w http.ResponseWriter, r *http.Request) {
	p, e := a.Projects.Get(r.PathValue("id"))
	if e != nil {
		fail(w, 404, e)
		return
	}
	v := a.view(p)
	root := a.Projects.Root(p.ID)
	versions := []map[string]any{}
	entries, readErr := os.ReadDir(filepath.Join(root, "releases"))
	if readErr != nil && !os.IsNotExist(readErr) {
		v["versionsError"] = readErr.Error()
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		m, e := bundle.ReadManifest(filepath.Join(root, "releases", entry.Name()))
		if e != nil {
			continue
		}
		info, e := entry.Info()
		if e != nil {
			continue
		}
		versions = append(versions, map[string]any{"version": m.Version, "type": m.Type, "createdAt": info.ModTime(), "bytes": dirSize(filepath.Join(root, "releases", entry.Name()))})
	}
	sort.Slice(versions, func(i, j int) bool {
		return versions[i]["createdAt"].(time.Time).After(versions[j]["createdAt"].(time.Time))
	})
	v["versions"] = versions
	files, e := backup.ScanBackups(filepath.Join(root, "backups"))
	if e != nil {
		v["backupsError"] = e.Error()
	}
	backups := []map[string]any{}
	for _, f := range files {
		backups = append(backups, map[string]any{"name": f.Name, "bytes": f.Size, "createdAt": f.ModTime})
	}
	v["backups"] = backups
	stats, e := cleaner.AnalyzeProject(root)
	if e == nil {
		v["cleanup"] = stats
	} else {
		v["cleanupError"] = e.Error()
	}
	write(w, 200, v)
}
func (a *App) startTask(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Kind   string `json:"kind"`
		Target string `json:"target"`
		Force  bool   `json:"force"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Kind != "deploy" && in.Kind != "rollback" && in.Kind != "backup" && in.Kind != "clean" {
		fail(w, 400, errors.New("不支持的任务类型"))
		return
	}
	if in.Kind == "clean" && in.Target != "bundles" && in.Target != "releases" && in.Target != "tmp" {
		fail(w, 400, errors.New("无效清理目标"))
		return
	}
	if in.Kind == "rollback" && (in.Target == "" || in.Target == "." || in.Target == ".." || strings.ContainsAny(in.Target, "/\\:")) {
		fail(w, 400, errors.New("无效回退版本"))
		return
	}
	t, e := a.Tasks.Start(r.PathValue("id"), in.Kind, in.Target, "", in.Force)
	if e != nil {
		fail(w, 409, e)
		return
	}
	write(w, 202, t)
}
func (a *App) upload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, e := a.Projects.Get(id); e != nil {
		fail(w, 404, e)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, a.Config.MaxUploadMB<<20)
	if e := r.ParseMultipartForm(8 << 20); e != nil {
		if r.MultipartForm != nil {
			r.MultipartForm.RemoveAll()
		}
		fail(w, 400, fmt.Errorf("上传失败，文件上限 %d MB", a.Config.MaxUploadMB))
		return
	}
	defer r.MultipartForm.RemoveAll()
	file, _, e := r.FormFile("file")
	if e != nil {
		fail(w, 400, errors.New("请选择部署包"))
		return
	}
	defer file.Close()
	dir := filepath.Join(a.Config.DataDir, "uploads")
	if e = os.MkdirAll(dir, 0700); e != nil {
		fail(w, 500, e)
		return
	}
	dest, e := os.CreateTemp(dir, "bundle-*")
	if e != nil {
		fail(w, 500, e)
		return
	}
	name := dest.Name()
	_, e = io.Copy(dest, file)
	ce := dest.Close()
	if e != nil || ce != nil {
		os.Remove(name)
		fail(w, 500, errors.New("保存上传文件失败"))
		return
	}
	t, e := a.Tasks.Start(id, "deploy", "local", name, r.FormValue("force") == "true")
	if e != nil {
		os.Remove(name)
		fail(w, 409, e)
		return
	}
	write(w, 202, t)
}
func (a *App) listTasks(w http.ResponseWriter, r *http.Request) {
	list := []store.Task{}
	for _, t := range a.Store.Snapshot().Tasks {
		if id := r.URL.Query().Get("projectId"); id == "" || id == t.ProjectID {
			list = append(list, t)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt.After(list[j].CreatedAt) })
	if len(list) > 200 {
		list = list[:200]
	}
	write(w, 200, list)
}
func (a *App) getTask(w http.ResponseWriter, r *http.Request) {
	t, ok := a.Store.Snapshot().Tasks[r.PathValue("id")]
	if !ok {
		fail(w, 404, errors.New("任务不存在"))
		return
	}
	write(w, 200, t)
}
func (a *App) taskLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, ok := a.Store.Snapshot().Tasks[id]; !ok {
		fail(w, 404, errors.New("任务不存在"))
		return
	}
	f, e := os.Open(a.Tasks.LogPath(id))
	if os.IsNotExist(e) {
		write(w, 200, map[string]string{"text": "等待任务开始…"})
		return
	}
	if e != nil {
		fail(w, 500, e)
		return
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		fail(w, 500, e)
		return
	}
	const max = 2 << 20
	if info.Size() > max {
		_, _ = f.Seek(-max, io.SeekEnd)
	}
	b, e := io.ReadAll(io.LimitReader(f, max))
	if e != nil {
		fail(w, 500, e)
		return
	}
	write(w, 200, map[string]string{"text": string(b)})
}
func (a *App) downloadBackup(w http.ResponseWriter, r *http.Request) {
	p, e := a.Projects.Get(r.PathValue("id"))
	if e != nil {
		fail(w, 404, e)
		return
	}
	name := r.PathValue("name")
	if name == "" || strings.ContainsAny(name, "/\\:") || strings.HasPrefix(name, ".") {
		fail(w, 400, errors.New("无效文件名"))
		return
	}
	path := filepath.Join(a.Projects.Root(p.ID), "backups", name)
	info, e := os.Lstat(path)
	if e != nil || !info.Mode().IsRegular() {
		fail(w, 404, errors.New("归档不存在"))
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(name)))
	http.ServeFile(w, r, path)
}
func dirSize(root string) int64 {
	var n int64
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return nil
		}
		if !d.IsDir() && d.Type()&os.ModeSymlink == 0 {
			if info, e := d.Info(); e == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}
func (a *App) static() http.Handler {
	files := webembed.Files()
	if _, e := os.Stat(filepath.Join(a.Config.WebDir, "index.html")); e == nil {
		files = os.DirFS(a.Config.WebDir)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		if !fs.ValidPath(name) {
			http.NotFound(w, r)
			return
		}
		if _, e := fs.Stat(files, name); e != nil {
			if filepath.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
			name = "index.html"
		}
		if _, e := fs.Stat(files, name); e != nil {
			http.Error(w, "前端尚未构建，请在 web/ 执行 npm install && npm run build", 503)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + name
		// FileServer redirects /index.html to /. Serve SPA HTML explicitly instead.
		if name == "index.html" {
			b, e := fs.ReadFile(files, name)
			if e != nil {
				http.Error(w, "read frontend failed", 500)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-cache")
			_, _ = w.Write(b)
			return
		}
		http.FileServer(http.FS(files)).ServeHTTP(w, r2)
	})
}
