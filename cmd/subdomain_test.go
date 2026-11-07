package cmd

import (
	"strings"
	"testing"

	"kexoz/config"
)

func TestResolveSubdomainPrefersFlag(t *testing.T) {
	got, err := resolveSubdomain("My-App", &config.Config{Subdomain: "saved"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "my-app" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSubdomainUsesConfig(t *testing.T) {
	got, err := resolveSubdomain("", &config.Config{Subdomain: "saved-name"})
	if err != nil {
		t.Fatal(err)
	}
	if got != "saved-name" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveSubdomainGeneratesWhenEmpty(t *testing.T) {
	got, err := resolveSubdomain("", &config.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, "dev-") || len(got) != len("dev-")+8 {
		t.Fatalf("unexpected generated subdomain %q", got)
	}
}

func TestResolveSubdomainRejectsReserved(t *testing.T) {
	if _, err := resolveSubdomain("api", &config.Config{}); err == nil {
		t.Fatal("expected reserved error")
	}
}
