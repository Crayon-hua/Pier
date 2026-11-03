package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"pier/internal/config"
	"pier/internal/store"
)

func TestAuthenticationAndProjectLifecycle(t *testing.T) {
	app, e := New(config.Config{DataDir: t.TempDir(), MaxUploadMB: 1})
	if e != nil {
		t.Fatal(e)
	}
	call := func(method, path string, payload any, cookie *http.Cookie) *httptest.ResponseRecorder {
		b, _ := json.Marshal(payload)
		req := httptest.NewRequest(method, path, bytes.NewReader(b))
		req.RemoteAddr = "127.0.0.1:1234"
		if cookie != nil {
			req.AddCookie(cookie)
		}
		res := httptest.NewRecorder()
		app.HTTP.Handler.ServeHTTP(res, req)
		return res
	}
	if r := call("GET", "/api/v1/projects", nil, nil); r.Code != 401 {
		t.Fatalf("unauthenticated projects: %d", r.Code)
	}
	credentials := map[string]string{"username": "operator", "password": "correct-password-123"}
	if r := call("POST", "/api/v1/auth/setup", credentials, nil); r.Code != 201 {
		t.Fatal(r.Body.String())
	}
	if r := call("POST", "/api/v1/auth/setup", credentials, nil); r.Code == 201 {
		t.Fatal("setup may only run once")
	}
	if r := call("POST", "/api/v1/auth/login", map[string]string{"username": "operator", "password": "wrong"}, nil); r.Code != 401 {
		t.Fatal("invalid password accepted")
	}
	login := call("POST", "/api/v1/auth/login", credentials, nil)
	if login.Code != 200 {
		t.Fatal(login.Body.String())
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatal("invalid session cookie")
	}
	cookie := cookies[0]
	bad := call("POST", "/api/v1/projects", map[string]any{"id": "../escape", "name": "Bad", "type": "static"}, cookie)
	if bad.Code != 400 {
		t.Fatal("unsafe project id accepted")
	}
	payload := map[string]any{"id": "demo-app", "name": "Demo", "type": "static", "githubToken": "private-test-token", "configYaml": "bundles:\n  keep: 3\n"}
	r := call("POST", "/api/v1/projects", payload, cookie)
	if r.Code != 201 {
		t.Fatal(r.Body.String())
	}
	if bytes.Contains(r.Body.Bytes(), []byte("private-test-token")) {
		t.Fatal("credential leaked")
	}
	if r = call("GET", "/api/v1/projects/demo-app", nil, cookie); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	delete(payload, "githubToken")
	payload["name"] = "Renamed"
	if r = call("PUT", "/api/v1/projects/demo-app", payload, cookie); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if app.Store.Snapshot().Projects["demo-app"].GitHubToken != "private-test-token" {
		t.Fatal("blank token erased stored credential")
	}
	req := httptest.NewRequest("DELETE", "http://example.com/api/v1/projects/demo-app", nil)
	req.AddCookie(cookie)
	req.Header.Set("Origin", "https://evil.test")
	res := httptest.NewRecorder()
	app.HTTP.Handler.ServeHTTP(res, req)
	if res.Code != 403 {
		t.Fatal("cross-origin mutation accepted")
	}
	if r = call("POST", "/api/v1/projects/demo-app/tasks", map[string]string{"kind": "rollback", "target": "../escape"}, cookie); r.Code != 400 {
		t.Fatal("unsafe rollback accepted")
	}
	if r = call("POST", "/api/v1/auth/logout", nil, cookie); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r = call("GET", "/api/v1/auth/me", nil, cookie); r.Code != 401 {
		t.Fatal("logout did not revoke session")
	}
	persisted, e := os.ReadFile(filepath.Join(app.Config.DataDir, "pier.json"))
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(persisted, []byte(credentials["password"])) {
		t.Fatal("plaintext password persisted")
	}
}

func TestPasswordChangeRevokesSessions(t *testing.T) {
	app, e := New(config.Config{DataDir: t.TempDir(), MaxUploadMB: 1})
	if e != nil {
		t.Fatal(e)
	}
	if e = app.Auth.Setup("admin", "first-password-123"); e != nil {
		t.Fatal(e)
	}
	w := httptest.NewRecorder()
	if e = app.Auth.Login(w, "admin", "first-password-123"); e != nil {
		t.Fatal(e)
	}
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(w.Result().Cookies()[0])
	if e = app.Auth.ChangePassword("first-password-123", "second-password-456"); e != nil {
		t.Fatal(e)
	}
	if app.Auth.Valid(req) {
		t.Fatal("old session remains valid")
	}
	if e = app.Auth.Login(httptest.NewRecorder(), "admin", "first-password-123"); e == nil {
		t.Fatal("old password accepted")
	}
	if e = app.Auth.Login(httptest.NewRecorder(), "admin", "second-password-456"); e != nil {
		t.Fatal(e)
	}
}

func TestBusyProjectCannotChangeOrRemove(t *testing.T) {
	app, e := New(config.Config{DataDir: t.TempDir(), MaxUploadMB: 1})
	if e != nil {
		t.Fatal(e)
	}
	p, e := app.Projects.Save(store.Project{ID: "demo-app", Name: "Demo", Type: "static"}, true, false)
	if e != nil {
		t.Fatal(e)
	}
	if e = app.Store.Update(func(d *store.Data) error {
		d.Tasks["busy"] = store.Task{ID: "busy", ProjectID: p.ID, Status: "running"}
		return nil
	}); e != nil {
		t.Fatal(e)
	}
	if _, e = app.Projects.Save(p, false, false); e == nil {
		t.Fatal("changed a busy project")
	}
	if e = app.Projects.Remove(p.ID); e == nil {
		t.Fatal("removed a busy project")
	}
}

func TestWebhookRequiresProjectSecretAndValidHMAC(t *testing.T) {
	app, e := New(config.Config{DataDir: t.TempDir(), MaxUploadMB: 1})
	if e != nil {
		t.Fatal(e)
	}
	p, e := app.Projects.Save(store.Project{ID: "demo-app", Name: "Demo", Type: "static"}, true, false)
	if e != nil {
		t.Fatal(e)
	}
	ping := func(signature string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", "/api/v1/webhooks/demo-app", bytes.NewBufferString(`{}`))
		req.Header.Set("X-GitHub-Event", "ping")
		req.Header.Set("X-Hub-Signature-256", signature)
		w := httptest.NewRecorder()
		app.HTTP.Handler.ServeHTTP(w, req)
		return w
	}
	if r := ping(""); r.Code != 403 {
		t.Fatal("unconfigured webhook is not disabled")
	}
	secret := "random-project-secret-for-testing-only"
	p.ConfigYAML = "webhook:\n  secret: " + secret + "\n"
	if _, e = app.Projects.Save(p, false, false); e != nil {
		t.Fatal(e)
	}
	if r := ping("sha256=00"); r.Code != 401 {
		t.Fatal("invalid signature accepted")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(`{}`))
	if r := ping("sha256=" + hex.EncodeToString(mac.Sum(nil))); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if len(app.Store.Snapshot().Tasks) != 0 {
		t.Fatal("ping created a deploy task")
	}
}
