package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// Version is injected at release time via:
// go build -ldflags "-X kexoz/cmd.Version=v1.2.3"
var Version = "dev"

var ServerURL string
var RootDomain string
var JSONOutput bool

var rootCmd = &cobra.Command{
	Use:   "kexoz",
	Short: "Share your local apps and services securely",
	Long:  `Kexoz shares local web apps and raw TCP/UDP services through secure, multiplexed tunnels.`,
	// Version enables `kexoz --version`, which the release smoke test and the
	// Homebrew formula test both rely on.
	Version: Version,
}

func Execute() {
	// Keep --version output identical to the `version` subcommand.
	rootCmd.SetVersionTemplate("kexoz {{.Version}}\n")
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// NormalizedServerURL returns ServerURL without a trailing slash.
func NormalizedServerURL() string {
	return strings.TrimSuffix(strings.TrimSpace(ServerURL), "/")
}

func defaultServerURL() string {
	if v := strings.TrimSpace(os.Getenv("KEXOZ_SERVER_URL")); v != "" {
		return strings.TrimSuffix(v, "/")
	}
	return "https://api.kexoz.dev"
}

func defaultRootDomain() string {
	if v := strings.TrimSpace(os.Getenv("KEXOZ_ROOT_DOMAIN")); v != "" {
		return v
	}
	return "kexoz.dev"
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&ServerURL, "server", "s", defaultServerURL(), "Kexoz server URL (or KEXOZ_SERVER_URL)")
	rootCmd.PersistentFlags().StringVarP(&RootDomain, "root-domain", "d", defaultRootDomain(), "Kexoz root domain for public URLs (or KEXOZ_ROOT_DOMAIN)")
	rootCmd.PersistentFlags().BoolVar(&JSONOutput, "json", false, "Print machine-readable JSON output")
}
