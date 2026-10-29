package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const projectFileName = "kexoz.json"

// projectFile is kexoz.json: the saved project a folder belongs to, so
// `kexoz start` there needs no arguments. It holds no secrets and is meant to
// be committed with the code.
type projectFile struct {
	Project     string `json:"project"`
	Environment string `json:"environment,omitempty"`
	// Ports overrides local ports for people whose app runs elsewhere. Keys
	// are a share's name or address, optionally followed by a route path:
	// {"web": 5173, "web /api": 8000}.
	Ports map[string]int `json:"ports,omitempty"`
}

// findProjectFile looks for kexoz.json in dir and then each parent folder.
func findProjectFile(dir string) (string, *projectFile, error) {
	for {
		path := filepath.Join(dir, projectFileName)
		raw, err := os.ReadFile(path)
		if err == nil {
			var file projectFile
			if err := json.Unmarshal(raw, &file); err != nil {
				return path, nil, fmt.Errorf("%s is not valid JSON: %w", path, err)
			}
			if strings.TrimSpace(file.Project) == "" {
				return path, nil, fmt.Errorf("%s has no \"project\"", path)
			}
			return path, &file, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return path, nil, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil, nil
		}
		dir = parent
	}
}

func writeProjectFile(path string, file projectFile) error {
	raw, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}

// environmentName maps what people type to the stored name: "main" and ""
// both mean a project's main environment.
func environmentName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	if name == "main" || name == "default" {
		return ""
	}
	return name
}

func environmentLabel(name string) string {
	if name == "" {
		return "main"
	}
	return name
}

// overridePort returns the local port kexoz.json sets for a share (path "")
// or one of its routes, or fallback.
func (f *projectFile) overridePort(share cliTunnel, path string, fallback int) int {
	if f == nil || len(f.Ports) == 0 {
		return fallback
	}
	for _, key := range []string{share.Name, share.Subdomain, share.ID} {
		if key == "" {
			continue
		}
		if path != "" {
			key += " " + path
		}
		if port, ok := f.Ports[key]; ok && port > 0 && port < 65536 {
			return port
		}
	}
	return fallback
}
