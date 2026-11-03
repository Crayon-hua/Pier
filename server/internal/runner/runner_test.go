package runner

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"pier/internal/store"
)

func TestContainerRejectsHostSystemdBeforeAnyDeployment(t *testing.T) {
	t.Setenv("PIER_CONTAINER", "1")
	for _, kind := range []string{"deploy", "rollback"} {
		err := Run(context.Background(), Input{Kind: kind, Project: store.Project{Type: "system"}, Root: t.TempDir()})
		if err == nil || !strings.Contains(err.Error(), "systemd") {
			t.Fatalf("expected explicit container limitation, got %v", err)
		}
	}
}

func testBundle(t *testing.T, manifest string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "bundle.zip")
	f, e := os.Create(path)
	if e != nil {
		t.Fatal(e)
	}
	w := zip.NewWriter(f)
	m, e := w.Create("upy.yaml")
	if e != nil {
		t.Fatal(e)
	}
	_, _ = m.Write([]byte(manifest))
	index, _ := w.Create("dist/index.html")
	_, _ = index.Write([]byte("ok"))
	if e = w.Close(); e != nil {
		t.Fatal(e)
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	return path
}
func TestBundleIdentityAndVersionValidation(t *testing.T) {
	p := store.Project{ID: "demo-app", Type: "static"}
	for _, tc := range []struct {
		name, manifest, tag string
		wantErr             bool
	}{
		{"valid", "name: demo-app\nversion: v1.0.0\ntype: static\n", "v1.0.0", false},
		{"wrong project", "name: other-app\nversion: v1.0.0\ntype: static\n", "", true},
		{"tag mismatch", "name: demo-app\nversion: v1.0.0\ntype: static\n", "v2.0.0", true},
		{"unsafe version", "name: demo-app\nversion: ../../escape\ntype: static\n", "", true},
		{"unsafe static path", "name: demo-app\nversion: v1\ntype: static\nstatic:\n  directory: ../outside\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := validate(testBundle(t, tc.manifest), p, tc.tag)
			if (e != nil) != tc.wantErr {
				t.Fatalf("unexpected validation result: %v", e)
			}
		})
	}
}
