package cmd

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	"kexoz/config"
)

// resolveSubdomain picks the public hostname label for a new HTTP/E2E share.
// Preference: --subdomain flag, then a previously claimed config subdomain,
// then a random unused-looking label (server still enforces uniqueness).
func resolveSubdomain(flagValue string, cfg *config.Config) (string, error) {
	if name := strings.ToLower(strings.TrimSpace(flagValue)); name != "" {
		if isReservedSubdomain(name) {
			return "", fmt.Errorf("%q is reserved for Kexoz infrastructure; choose another subdomain", name)
		}
		return name, nil
	}
	if name := strings.ToLower(strings.TrimSpace(cfg.Subdomain)); name != "" {
		return name, nil
	}
	return randomSubdomain(), nil
}

func randomSubdomain() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return "dev-" + hex.EncodeToString(b)
}
