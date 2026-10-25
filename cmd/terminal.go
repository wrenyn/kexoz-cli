package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"kexoz/config"
	"kexoz/tunnel"

	"github.com/spf13/cobra"
)

var terminalSubdomain string
var terminalPassword string

const terminalPasswordAlphabet = "abcdefghjkmnpqrstuvwxyz23456789"

func generatePassword(length int) (string, error) {
	var b strings.Builder
	limit := big.NewInt(int64(len(terminalPasswordAlphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", err
		}
		b.WriteByte(terminalPasswordAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// createdTunnelID reads the share ID from a create-tunnel response.
func createdTunnelID(body []byte) string {
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(body, &created)
	return created.ID
}

var terminalCmd = &cobra.Command{
	Use:   "terminal",
	Short: "Share a browser terminal, protected by a password",
	RunE: func(cmd *cobra.Command, args []string) error {
		serverURL := NormalizedServerURL()

		password := terminalPassword
		if password == "" {
			generated, err := generatePassword(12)
			if err != nil {
				return fmt.Errorf("could not generate a password: %w", err)
			}
			password = generated
		} else if len([]rune(password)) < 4 {
			return fmt.Errorf("--password must be at least 4 characters")
		}

		fmt.Printf("Authenticating with %s...\n", serverURL)
		cfg, err := config.EnsureIdentity(serverURL)
		if err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}

		if strings.TrimSpace(terminalSubdomain) != "" {
			if err := claimSubdomain(serverURL, cfg.ClientID, strings.ToLower(strings.TrimSpace(terminalSubdomain))); err != nil {
				return err
			}
			cfg.Subdomain = strings.ToLower(strings.TrimSpace(terminalSubdomain))
			if err := config.SaveConfig(cfg); err != nil {
				return fmt.Errorf("failed to save config: %w", err)
			}
		}
		if cfg.Subdomain == "" {
			return fmt.Errorf("terminal shares need an address; pass --subdomain <name>")
		}

		payload := map[string]interface{}{
			"clientId":   cfg.ClientID,
			"tunnelType": "pty",
			"port":       1,
			"subdomain":  cfg.Subdomain,
			"password":   password,
			"access":     "private",
		}

		payloadBytes, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, serverURL+"/client/tunnels", bytes.NewReader(payloadBytes))
		req.Header.Set("Content-Type", "application/json")
		resp, err := config.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to create terminal tunnel on server: %w", err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			return fmt.Errorf("failed to create terminal tunnel, status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
		}

		fmt.Printf("Client ID: %s\n", cfg.ClientID)
		fmt.Printf("Terminal URL: https://%s.%s\n", cfg.Subdomain, RootDomain)
		fmt.Printf("Password:     %s\n", password)
		fmt.Println("Anyone with the URL and password can type into this terminal. Press Ctrl+C to stop sharing.")

		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		t := &tunnel.Tunnel{
			ServerURL: serverURL,
			ClientID:  cfg.ClientID,
			TunnelID:  createdTunnelID(body),
			IsTCP:     false,
			IsPTY:     true,
		}

		tcpAddr, _ := cmd.Flags().GetString("tcp")
		if tcpAddr == "" {
			host := strings.TrimPrefix(strings.TrimPrefix(serverURL, "https://"), "http://")
			if strings.Contains(host, ":") {
				host = strings.Split(host, ":")[0]
			}
			tcpAddr = host + ":8081"
		}
		if err := t.StartTCP(ctx, tcpAddr); err != nil {
			return err
		}

		fmt.Println("Terminal stopped.")
		return nil
	},
}

func init() {
	terminalCmd.Flags().StringVar(&terminalSubdomain, "subdomain", "", "Claim this subdomain before starting the tunnel")
	terminalCmd.Flags().StringVar(&terminalPassword, "password", "", "Password visitors must enter (a random one is generated if omitted)")
	terminalCmd.Flags().String("tcp", "", "Use raw TCP transport (specify server address, e.g. localhost:8081)")
	rootCmd.AddCommand(terminalCmd)
}
