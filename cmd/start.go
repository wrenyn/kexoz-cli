package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"kexoz/config"
	"kexoz/tunnel"

	"github.com/spf13/cobra"
)

type cliProject struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	TunnelIDs []string `json:"tunnelIds"`
}

func fetchProjects(cfg *config.Config) ([]cliProject, error) {
	resp, err := config.HTTPClient.Get(NormalizedServerURL() + "/client/projects?clientId=" + url.QueryEscape(cfg.ClientID))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, decodeAPIError(resp)
	}
	var result struct {
		Projects []cliProject `json:"projects"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Projects, nil
}

func setTunnelActive(cfg *config.Config, t cliTunnel, active bool) {
	payload, _ := json.Marshal(map[string]any{
		"clientId": cfg.ClientID, "subdomain": t.Subdomain, "customDomain": t.CustomDomain,
		"port": t.Port, "requireAuth": t.RequireAuth, "pathRoutes": t.PathRoutes, "active": active,
	})
	req, _ := http.NewRequest(http.MethodPut, NormalizedServerURL()+"/client/tunnels/"+url.PathEscape(t.ID)+"?clientId="+url.QueryEscape(cfg.ClientID), bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if resp, err := config.HTTPClient.Do(req); err == nil {
		resp.Body.Close()
	}
}

func shareAddress(t cliTunnel) string {
	switch {
	case t.CustomDomain != "":
		return "https://" + t.CustomDomain
	case t.TunnelType == "tcp" || t.TunnelType == "udp":
		return fmt.Sprintf("%s://%s:%d", t.TunnelType, RootDomain, t.PublicTCPPort)
	case t.Subdomain != "":
		return fmt.Sprintf("https://%s.%s", t.Subdomain, RootDomain)
	}
	return t.ID
}

func agentTCPAddr(serverURL string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(serverURL, "https://"), "http://")
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}
	return host + ":8081"
}

var startCmd = &cobra.Command{
	Use:   "start [project]",
	Short: "Start every share in a saved project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := clientConfig()
		if err != nil {
			return err
		}
		projects, err := fetchProjects(cfg)
		if err != nil {
			return err
		}
		var project *cliProject
		for i := range projects {
			if strings.EqualFold(projects[i].Name, strings.TrimSpace(args[0])) {
				project = &projects[i]
				break
			}
		}
		if project == nil {
			names := make([]string, 0, len(projects))
			for _, p := range projects {
				names = append(names, p.Name)
			}
			if len(names) == 0 {
				return fmt.Errorf("you have no saved projects yet; create one in the desktop app")
			}
			return fmt.Errorf("no project named %q; your projects: %s", args[0], strings.Join(names, ", "))
		}
		tunnels, err := fetchTunnels(cfg)
		if err != nil {
			return err
		}
		byID := map[string]cliTunnel{}
		for _, t := range tunnels {
			byID[t.ID] = t
		}
		var selected []cliTunnel
		for _, id := range project.TunnelIDs {
			if t, ok := byID[id]; ok {
				selected = append(selected, t)
			}
		}
		if len(selected) == 0 {
			return fmt.Errorf("project %q has no shares left", project.Name)
		}

		serverURL := NormalizedServerURL()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		fmt.Printf("Starting %s\n", project.Name)
		var wg sync.WaitGroup
		for _, share := range selected {
			if share.TunnelType == "folder" {
				fmt.Printf("  skipped folder share %s (folders are shared from the desktop app)\n", shareAddress(share))
				continue
			}
			if share.TunnelType == "pty" {
				fmt.Printf("  skipped terminal %s (start terminals with kexoz terminal)\n", shareAddress(share))
				continue
			}
			if share.Name != "" {
				fmt.Printf("  %s: %s -> localhost:%d\n", share.Name, shareAddress(share), share.Port)
			} else {
				fmt.Printf("  %s -> localhost:%d\n", shareAddress(share), share.Port)
			}
			setTunnelActive(cfg, share, true)
			t := &tunnel.Tunnel{
				ServerURL: serverURL,
				ClientID:  cfg.ClientID,
				TunnelID:  share.ID,
				LocalPort: share.Port,
				IsTCP:     share.TunnelType == "tcp",
				IsE2E:     share.TunnelType == "e2e",
				IsUDP:     share.TunnelType == "udp",
			}
			wg.Add(1)
			go func(share cliTunnel, t *tunnel.Tunnel) {
				defer wg.Done()
				defer setTunnelActive(cfg, share, false)
				var runErr error
				switch share.TunnelType {
				case "tcp", "e2e":
					runErr = t.StartMultiplexedWS(ctx)
				case "udp":
					runErr = t.StartTCP(ctx, agentTCPAddr(serverURL))
				default:
					runErr = t.Start(ctx)
				}
				if runErr != nil {
					fmt.Printf("  %s stopped: %v\n", shareAddress(share), runErr)
				}
			}(share, t)
		}
		fmt.Println("Press Ctrl+C to stop.")
		wg.Wait()
		fmt.Println("Project stopped.")
		return nil
	},
}

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List saved projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := clientConfig()
		if err != nil {
			return err
		}
		projects, err := fetchProjects(cfg)
		if err != nil {
			return err
		}
		if JSONOutput {
			fmt.Println(mustJSON(map[string]any{"projects": projects}))
			return nil
		}
		if len(projects) == 0 {
			fmt.Println("No saved projects. Create one in the desktop app, then run kexoz start <name>.")
			return nil
		}
		for _, p := range projects {
			label := "shares"
			if len(p.TunnelIDs) == 1 {
				label = "share"
			}
			fmt.Printf("%s\t%d %s\n", p.Name, len(p.TunnelIDs), label)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd, projectsCmd)
}
