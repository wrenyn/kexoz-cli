package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindProjectFileWalksUp(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if path, file, err := findProjectFile(nested); err != nil || file != nil || path != "" {
		t.Fatalf("expected nothing before writing, got %q %v %v", path, file, err)
	}
	want := filepath.Join(root, projectFileName)
	if err := writeProjectFile(want, projectFile{Project: "Shop", Environment: "staging"}); err != nil {
		t.Fatal(err)
	}
	path, file, err := findProjectFile(nested)
	if err != nil || file == nil {
		t.Fatalf("findProjectFile: %v", err)
	}
	if path != want || file.Project != "Shop" || file.Environment != "staging" {
		t.Fatalf("got %q %+v", path, file)
	}
}

func TestFindProjectFileRejectsBadFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, projectFileName)
	if err := os.WriteFile(path, []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := findProjectFile(dir); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
	if err := os.WriteFile(path, []byte(`{"environment":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := findProjectFile(dir); err == nil {
		t.Fatal("expected a missing project to fail")
	}
}

func TestEnvironmentName(t *testing.T) {
	for raw, want := range map[string]string{"": "", "main": "", " Default ": "", "Staging": "staging", "pr-12": "pr-12"} {
		if got := environmentName(raw); got != want {
			t.Errorf("environmentName(%q) = %q, want %q", raw, got, want)
		}
	}
	if environmentLabel("") != "main" || environmentLabel("staging") != "staging" {
		t.Fatal("environmentLabel")
	}
}

func TestOverridePort(t *testing.T) {
	share := cliTunnel{ID: "t1", Name: "web", Subdomain: "shop-web", Port: 3000}
	var none *projectFile
	if got := none.overridePort(share, "", 3000); got != 3000 {
		t.Fatalf("nil file changed the port to %d", got)
	}
	file := &projectFile{Project: "Shop", Ports: map[string]int{
		"web":           5173,
		"shop-web /api": 8000,
		"t1 /admin":     70000,
		"other":         1111,
	}}
	cases := []struct {
		path string
		want int
	}{
		{"", 5173},
		{"/api", 8000},
		{"/admin", 9000},
		{"/missing", 9000},
	}
	for _, tc := range cases {
		if got := file.overridePort(share, tc.path, 9000); got != tc.want {
			t.Errorf("overridePort(%q) = %d, want %d", tc.path, got, tc.want)
		}
	}
}
