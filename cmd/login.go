package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"time"

	"kexoz/config"

	"github.com/spf13/cobra"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Link a Google account to unlock 1 GB free bandwidth",
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL := NormalizedServerURL()
		token, _ := cmd.Flags().GetString("token")
		if token != "" {
			os.Setenv("KEXOZ_TOKEN", token)
		}

		cfg, err := config.EnsureIdentity(serverURL)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		if token != "" || os.Getenv("KEXOZ_TOKEN") != "" {
			fmt.Printf("Authenticated successfully as %s (tier: %s) via API token.\n", cfg.ClientID, cfg.Tier())
			if cfg.OwnerEmail != "" {
				fmt.Printf("Account email: %s\n", cfg.OwnerEmail)
			}
			return nil
		}

		if cfg.OwnerEmail != "" {
			fmt.Printf("Already verified as %s (tier: %s).\n", cfg.OwnerEmail, cfg.Tier())
			return nil
		}

		// Fail fast when the server has no Google OAuth configured.
		statusResp, err := config.HTTPClient.Get(serverURL + "/auth/google/status")
		if err != nil {
			return fmt.Errorf("failed to reach server: %w", err)
		}
		var gauth struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(statusResp.Body).Decode(&gauth); err != nil {
			statusResp.Body.Close()
			return fmt.Errorf("failed to read server status: %w", err)
		}
		statusResp.Body.Close()
		if !gauth.Enabled {
			return fmt.Errorf("Google verification is not enabled on this server")
		}

		finish := serverURL + "/client/link-finish?clientId=" + url.QueryEscape(cfg.ClientID)
		link, code := startLink(serverURL, cfg.ClientID)
		if link != "" {
			finish += "&link=" + url.QueryEscape(link)
		}
		loginURL := serverURL + "/auth/google/login?next=" + url.QueryEscape(
			base64.RawURLEncoding.EncodeToString([]byte(finish)))

		fmt.Println("Open this URL in your browser to verify with Google:")
		fmt.Println()
		fmt.Printf("  %s\n", loginURL)
		fmt.Println()
		if code != "" {
			fmt.Printf("If the browser asks, check it shows the code %s.\n", code)
		}
		fmt.Println("Waiting for verification (up to 5 minutes)...")

		deadline := time.Now().Add(5 * time.Minute)
		for time.Now().Before(deadline) {
			time.Sleep(3 * time.Second)
			statusURL := serverURL + "/client/link-status?clientId=" + url.QueryEscape(cfg.ClientID)
			resp, err := config.HTTPClient.Get(statusURL)
			if err != nil {
				continue // transient — keep polling
			}
			var status struct {
				Linked         bool   `json:"linked"`
				Email          string `json:"email"`
				BandwidthLimit int64  `json:"bandwidthLimit"`
				SwitchTo       string `json:"switchTo"`
			}
			decodeErr := json.NewDecoder(resp.Body).Decode(&status)
			resp.Body.Close()
			if decodeErr != nil || !status.Linked {
				continue
			}
			if status.SwitchTo != "" {
				// The Google account already has an identity; this machine uses it.
				*cfg = config.Config{ClientID: status.SwitchTo}
				if err := config.SaveConfig(cfg); err != nil {
					return fmt.Errorf("signed in, but failed to save config: %w", err)
				}
				if _, err := config.EnsureIdentity(serverURL); err != nil {
					return fmt.Errorf("signed in, but failed to load your account: %w", err)
				}
				fmt.Printf("Signed in as %s. This machine now uses your Kexoz account.\n", status.Email)
				return nil
			}
			cfg.OwnerEmail = status.Email
			if err := config.SaveConfig(cfg); err != nil {
				return fmt.Errorf("verified, but failed to save config: %w", err)
			}
			fmt.Printf("Verified as %s! Bandwidth limit: %.0f MB.\n", status.Email, float64(status.BandwidthLimit)/1e6)
			return nil
		}
		return fmt.Errorf("verification timed out — run `kexoz login` to try again")
	},
}

// startLink asks the server for a sign-in link and the code the browser shows
// before moving this machine onto an existing account. Older servers return
// nothing and the plain link still works.
func startLink(serverURL, clientID string) (string, string) {
	body, _ := json.Marshal(map[string]string{"clientId": clientID})
	resp, err := config.HTTPClient.Post(serverURL+"/client/link-start", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", ""
	}
	var started struct {
		Link string `json:"link"`
		Code string `json:"code"`
	}
	if json.NewDecoder(resp.Body).Decode(&started) != nil {
		return "", ""
	}
	return started.Link, started.Code
}

func init() {
	loginCmd.Flags().String("token", "", "Authenticate via an API token (bypasses browser)")
	rootCmd.AddCommand(loginCmd)
}
