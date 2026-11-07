package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kexoz/config"

	"github.com/spf13/cobra"
)

type cliInboxRequest struct {
	ID             string    `json:"id"`
	Method         string    `json:"method"`
	Path           string    `json:"path"`
	BodySize       int       `json:"bodySize"`
	ReceivedAt     time.Time `json:"receivedAt"`
	Delivery       string    `json:"delivery"`
	ResponseStatus int       `json:"responseStatus"`
	ResponseMs     int       `json:"responseMs"`
	ResponseError  string    `json:"responseError"`
	ResponseBody   string    `json:"responseBody"`
	ReplayCount    int       `json:"replayCount"`
}

type cliInbox struct {
	Inbox *struct {
		Token          string `json:"token"`
		TargetTunnelID string `json:"targetTunnelId"`
		Forward        bool   `json:"forward"`
	} `json:"inbox"`
	URL        string            `json:"url"`
	Requests   []cliInboxRequest `json:"requests"`
	Limit      int               `json:"limit"`
	TargetLive bool              `json:"targetLive"`
}

func inboxAPI(cfg *config.Config, method, path string, body any, into any) error {
	var reader *bytes.Reader
	if body != nil {
		raw, _ := json.Marshal(body)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, NormalizedServerURL()+path+"?clientId="+url.QueryEscape(cfg.ClientID), reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := config.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		var problem struct {
			Message string `json:"message"`
		}
		if json.NewDecoder(resp.Body).Decode(&problem) == nil && problem.Message != "" {
			return fmt.Errorf("%s", problem.Message)
		}
		return fmt.Errorf("server returned %s", resp.Status)
	}
	if into != nil {
		return json.NewDecoder(resp.Body).Decode(into)
	}
	return nil
}

func deliveryLabel(r cliInboxRequest) string {
	switch r.Delivery {
	case "forwarded", "replayed":
		return fmt.Sprintf("%s%d%s %s", statusColor(r.ResponseStatus), r.ResponseStatus, ansiReset, r.Delivery)
	case "failed":
		return ansiRed + "failed" + ansiReset
	}
	return ansiGray + "stored" + ansiReset
}

var (
	inboxCreate      bool
	inboxReplayShare string
)

var inboxCmd = &cobra.Command{
	Use:   "inbox [project]",
	Short: "Show a project's webhook inbox and its latest requests",
	Long: `Every project can have a permanent inbox URL for webhooks and OAuth
callbacks. Kexoz stores each request even while your app is offline, forwards
it live while the inbox's share runs, and lets you replay any of them.

Use the project from kexoz.json, or name it. --create makes the inbox if the
project has none yet.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		name, _, _, err := resolveProject(args, "", false)
		if err != nil {
			return err
		}
		cfg, err := clientConfig()
		if err != nil {
			return err
		}
		projects, err := fetchProjects(cfg)
		if err != nil {
			return err
		}
		project, err := findProject(projects, name)
		if err != nil {
			return err
		}
		base := "/client/projects/" + url.PathEscape(project.ID) + "/inbox"
		var inbox cliInbox
		if err := inboxAPI(cfg, http.MethodGet, base, nil, &inbox); err != nil {
			return err
		}
		if inbox.Inbox == nil {
			if !inboxCreate {
				return fmt.Errorf("%s has no inbox yet; run kexoz inbox --create", project.Name)
			}
			if err := inboxAPI(cfg, http.MethodPost, base, map[string]string{"clientId": cfg.ClientID}, &inbox); err != nil {
				return err
			}
		}
		if JSONOutput {
			fmt.Println(mustJSON(inbox))
			return nil
		}
		fmt.Printf("\n%s%s inbox%s\n  %s%s%s\n", ansiBold, project.Name, ansiReset, ansiBold+ansiCyan, inbox.URL, ansiReset)
		switch {
		case inbox.Inbox.Forward && inbox.TargetLive:
			fmt.Printf("  %sForwarding live to your app. Keeps the last %d requests.%s\n\n", ansiGray, inbox.Limit, ansiReset)
		case inbox.Inbox.Forward:
			fmt.Printf("  %sYour app isn't running, so requests are stored for replay. Keeps the last %d.%s\n\n", ansiGray, inbox.Limit, ansiReset)
		default:
			fmt.Printf("  %sLive forwarding is off; requests are stored for replay. Keeps the last %d.%s\n\n", ansiGray, inbox.Limit, ansiReset)
		}
		if len(inbox.Requests) == 0 {
			fmt.Println("  No requests yet. Point a webhook or OAuth callback at the URL above, adding your app's path after it.")
			return nil
		}
		shown := inbox.Requests
		if len(shown) > 15 {
			shown = shown[:15]
		}
		for _, r := range shown {
			path := r.Path
			if len(path) > 48 {
				path = path[:45] + "..."
			}
			fmt.Printf("  %s%-8s%s %s%-7s%s %-48s %s\n", ansiGray, r.ReceivedAt.Local().Format("15:04:05"), ansiReset, methodColor(r.Method), r.Method, ansiReset, path, deliveryLabel(r))
			fmt.Printf("  %s%s%s\n", ansiDim, r.ID, ansiReset)
		}
		fmt.Printf("\n  %sReplay one with: kexoz inbox replay <id>%s\n", ansiGray, ansiReset)
		return nil
	},
}

var inboxReplayCmd = &cobra.Command{
	Use:   "replay <request-id>",
	Short: "Send a stored inbox request to your app again",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cfg, err := clientConfig()
		if err != nil {
			return err
		}
		var result struct {
			Request cliInboxRequest `json:"request"`
		}
		payload := map[string]string{"clientId": cfg.ClientID, "tunnelId": strings.TrimSpace(inboxReplayShare)}
		if err := inboxAPI(cfg, http.MethodPost, "/client/inbox/requests/"+url.PathEscape(args[0])+"/replay", payload, &result); err != nil {
			return err
		}
		if JSONOutput {
			fmt.Println(mustJSON(result.Request))
			return nil
		}
		r := result.Request
		if r.Delivery == "failed" {
			return fmt.Errorf("replay failed: %s", r.ResponseError)
		}
		fmt.Printf("%s %s -> %s%d%s in %dms\n", r.Method, r.Path, statusColor(r.ResponseStatus), r.ResponseStatus, ansiReset, r.ResponseMs)
		if body := strings.TrimSpace(r.ResponseBody); body != "" {
			if len(body) > 600 {
				body = body[:600] + "…"
			}
			fmt.Println(body)
		}
		return nil
	},
}

var envFrom string

var envCmd = &cobra.Command{Use: "env", Short: "Manage a project's preview environments"}

var envNewCmd = &cobra.Command{
	Use:   "new <name> [project]",
	Short: "Copy an environment's shares into a new environment",
	Long: `Copy every share in an environment (main by default) into a new one, such
as staging or a feature branch. Each copy gets its own address and keeps the
original's ports, routes and access rules. Start it with kexoz start --env <name>.`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		name, _, _, err := resolveProject(args[1:], "", false)
		if err != nil {
			return err
		}
		cfg, err := clientConfig()
		if err != nil {
			return err
		}
		projects, err := fetchProjects(cfg)
		if err != nil {
			return err
		}
		project, err := findProject(projects, name)
		if err != nil {
			return err
		}
		var result struct {
			Tunnels []cliTunnel `json:"tunnels"`
		}
		payload := map[string]string{"clientId": cfg.ClientID, "name": args[0], "from": environmentName(envFrom)}
		if err := inboxAPI(cfg, http.MethodPost, "/client/projects/"+url.PathEscape(project.ID)+"/environments", payload, &result); err != nil {
			return err
		}
		if JSONOutput {
			fmt.Println(mustJSON(result))
			return nil
		}
		fmt.Printf("Created %s in %s:\n", environmentName(args[0]), project.Name)
		for _, share := range result.Tunnels {
			fmt.Printf("  %-12s %s%s%s -> localhost:%d\n", shareLabel(share), ansiCyan, shareAddress(share), ansiReset, share.Port)
		}
		fmt.Printf("Start it with %skexoz start --env %s%s\n", ansiBold, environmentName(args[0]), ansiReset)
		return nil
	},
}

func init() {
	inboxCmd.Flags().BoolVar(&inboxCreate, "create", false, "Create the inbox if the project has none")
	inboxReplayCmd.Flags().StringVar(&inboxReplayShare, "share", "", "Share ID to send it to (defaults to the inbox's share)")
	inboxCmd.AddCommand(inboxReplayCmd)
	envNewCmd.Flags().StringVar(&envFrom, "from", "main", "Environment to copy")
	envCmd.AddCommand(envNewCmd)
	rootCmd.AddCommand(inboxCmd, envCmd)
}
