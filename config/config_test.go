package config

// config_test.go — the config file holds the client identity (a bearer
// capability) and the linked account email, so it must round-trip faithfully and
// stay owner-readable only.

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// useTempConfig points the package at a scratch config file for one test.
func useTempConfig(t *testing.T) string {
	t.Helper()
	original := configPath
	path := filepath.Join(t.TempDir(), "kexoz", "config.yaml")
	configPath = path
	t.Cleanup(func() { configPath = original })
	return path
}

func TestSaveConfigRoundTrips(t *testing.T) {
	useTempConfig(t)
	want := &Config{
		ClientID:     "abc123",
		Subdomain:    "demo",
		CustomDomain: "app.example.com",
		Plan:         "pro",
		OwnerEmail:   "dev@example.com",
	}
	if err := SaveConfig(want); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got, err := LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got.ClientID != want.ClientID || got.Subdomain != want.Subdomain ||
		got.CustomDomain != want.CustomDomain || got.Plan != want.Plan || got.OwnerEmail != want.OwnerEmail {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, want)
	}
}

func TestSaveConfigUsesOwnerOnlyPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	path := useTempConfig(t)
	if err := SaveConfig(&Config{ClientID: "abc123"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}

	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Fatalf("config permissions = %o, want 600", perm)
	}

	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatalf("stat config dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Fatalf("config directory permissions = %o, want 700", perm)
	}
}

func TestSaveConfigTightensAnExistingLooseFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose POSIX permission bits")
	}
	path := useTempConfig(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("clientId: old\n"), 0o644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	if err := SaveConfig(&Config{ClientID: "new"}); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("permissions after update = %o, want 600", perm)
	}
}
