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
	"path/filepath"
	"sort"
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

// findProject matches a project by name, listing the choices when it can't.
func findProject(projects []cliProject, name string) (*cliProject, error) {
	for i := range projects {
		if strings.EqualFold(projects[i].Name, strings.TrimSpace(name)) {
			return &projects[i], nil
		}
	}
	names := make([]string, 0, len(projects))
	for _, p := range projects {
		names = append(names, p.Name)
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("you have no saved projects yet; create one in the desktop app")
	}
	return nil, fmt.Errorf("no project named %q; your projects: %s", name, strings.Join(names, ", "))
}

// projectShares returns a project's shares, keyed by environment.
func projectShares(project *cliProject, tunnels []cliTunnel) map[string][]cliTunnel {
	byID := map[string]cliTunnel{}
	for _, t := range tunnels {
		byID[t.ID] = t
	}
	byEnvironment := map[string][]cliTunnel{}
	for _, id := range project.TunnelIDs {
		if t, ok := byID[id]; ok {
			byEnvironment[t.Environment] = append(byEnvironment[t.Environment], t)
		}
	}
	return byEnvironment
}

func environmentNames(byEnvironment map[string][]cliTunnel) []string {
	names := make([]string, 0, len(byEnvironment))
	for name := range byEnvironment {
		names = append(names, environmentLabel(name))
	}
	sort.Strings(names)
	return names
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

func shareLabel(t cliTunnel) string {
	if t.Name != "" {
		return t.Name
	}
	if t.Subdomain != "" {
		return t.Subdomain
	}
	return t.TunnelType
}

func agentTCPAddr(serverURL string) string {
	host := strings.TrimPrefix(strings.TrimPrefix(serverURL, "https://"), "http://")
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}
	return host + ":8081"
}

// resolveProject works out which project and environment a command means:
// the argument and --env first, then kexoz.json in this folder or above.
func resolveProject(args []string, envFlag string, envSet bool) (string, string, *projectFile, error) {
	cwd, _ := os.Getwd()
	path, file, err := findProjectFile(cwd)
	if err != nil {
		return "", "", nil, err
	}
	name := ""
	if len(args) > 0 {
		name = args[0]
		if file != nil && !strings.EqualFold(file.Project, name) {
			file = nil
		}
	} else if file != nil {
		name = file.Project
		if !JSONOutput {
			fmt.Printf("%sUsing %s%s\n", ansiGray, path, ansiReset)
		}
	} else {
		return "", "", nil, fmt.Errorf("no %s in this folder or above; run kexoz init <project> here, or kexoz start <project>", projectFileName)
	}
	environment := ""
	switch {
	case envSet:
		environment = environmentName(envFlag)
	case file != nil:
		environment = environmentName(file.Environment)
	}
	return name, environment, file, nil
}

type inboxInfo struct {
	Inbox *struct {
		ID             string `json:"id"`
		TargetTunnelID string `json:"targetTunnelId"`
		Forward        bool   `json:"forward"`
	} `json:"inbox"`
	URL string `json:"url"`
}

func fetchInboxInfo(cfg *config.Config, projectID string) *inboxInfo {
	resp, err := config.HTTPClient.Get(NormalizedServerURL() + "/client/projects/" + url.PathEscape(projectID) + "/inbox?clientId=" + url.QueryEscape(cfg.ClientID))
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var info inboxInfo
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&info) != nil || info.Inbox == nil {
		return nil
	}
	return &info
}

var startEnv string

var startCmd = &cobra.Command{
	Use:   "start [project]",
	Short: "Start a project's shares and print their links",
	Long: `Start every share in a saved project, or in one of its environments.

Run it with no arguments in a folder that has a kexoz.json (see kexoz init)
and it starts that project. --env picks an environment such as staging; the
default is kexoz.json's environment, or the project's main one.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		name, environment, file, err := resolveProject(args, startEnv, cmd.Flags().Changed("env"))
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
		tunnels, err := fetchTunnels(cfg)
		if err != nil {
			return err
		}
		byEnvironment := projectShares(project, tunnels)
		selected := byEnvironment[environment]
		if len(selected) == 0 {
			if len(byEnvironment) == 0 {
				return fmt.Errorf("project %q has no shares left", project.Name)
			}
			return fmt.Errorf("project %q has no %s environment; it has: %s", project.Name, environmentLabel(environment), strings.Join(environmentNames(byEnvironment), ", "))
		}

		serverURL := NormalizedServerURL()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()

		title := project.Name
		if len(byEnvironment) > 1 || environment != "" {
			title += " (" + environmentLabel(environment) + ")"
		}
		fmt.Printf("\n%sStarting %s%s\n", ansiBold, title, ansiReset)
		labelWidth := 0
		for _, share := range selected {
			labelWidth = max(labelWidth, len(shareLabel(share)))
		}
		labelWidth = max(labelWidth, len("inbox"))
		type running struct {
			share  cliTunnel
			agent  *tunnel.Tunnel
			runner func(context.Context) error
		}
		var toRun []running
		for _, share := range selected {
			label := fmt.Sprintf("%-*s", labelWidth, shareLabel(share))
			if share.Paused {
				fmt.Printf("  %s  %spaused in the app; resume it there to include it%s\n", label, ansiGray, ansiReset)
				continue
			}
			port := file.overridePort(share, "", share.Port)
			var routes []tunnel.Route
			for _, route := range share.PathRoutes {
				if strings.TrimRight(route.Path, "/") == "" {
					continue
				}
				routes = append(routes, tunnel.Route{Path: route.Path, Port: file.overridePort(share, route.Path, route.Port)})
			}
			fmt.Printf("  %s  %s%s%s  %s-> localhost:%d%s\n", label, ansiBold+ansiCyan, shareAddress(share), ansiReset, ansiGray, port, ansiReset)
			for _, route := range routes {
				fmt.Printf("  %-*s  %s%s -> localhost:%d%s\n", labelWidth, "", ansiGray, route.Path, route.Port, ansiReset)
			}
			agent := &tunnel.Tunnel{
				ServerURL: serverURL,
				ClientID:  cfg.ClientID,
				TunnelID:  share.ID,
				LocalPort: port,
				Routes:    routes,
				IsTCP:     share.TunnelType == "tcp",
				IsE2E:     share.TunnelType == "e2e",
				IsUDP:     share.TunnelType == "udp",
				Quiet:     true,
			}
			shareName := shareLabel(share)
			agent.OnRequest = func(method, path string, status int, durationMs int64) {
				fmt.Printf("%s%-*s%s", ansiGray, labelWidth, shareName, ansiReset)
				PrintRequest(method, path, status, durationMs)
			}
			runner := agent.Start
			switch share.TunnelType {
			case "tcp", "e2e":
				runner = agent.StartMultiplexedWS
			case "udp":
				runner = func(ctx context.Context) error { return agent.StartTCP(ctx, agentTCPAddr(serverURL)) }
			}
			toRun = append(toRun, running{share: share, agent: agent, runner: runner})
		}
		if len(toRun) == 0 {
			return fmt.Errorf("every share in %s is paused", title)
		}
		if info := fetchInboxInfo(cfg, project.ID); info != nil {
			target := ""
			for _, share := range selected {
				if share.ID == info.Inbox.TargetTunnelID {
					target = shareLabel(share)
				}
			}
			note := "stores webhooks; replay them with kexoz inbox replay"
			if info.Inbox.Forward && target != "" {
				note = "forwards to " + target + " while it runs"
			}
			fmt.Printf("  %-*s  %s%s%s  %s%s%s\n", labelWidth, "inbox", ansiCyan, info.URL, ansiReset, ansiGray, note, ansiReset)
		}

		var wg sync.WaitGroup
		for _, item := range toRun {
			setTunnelActive(cfg, item.share, true)
			wg.Add(1)
			go func(item running) {
				defer wg.Done()
				defer setTunnelActive(cfg, item.share, false)
				if err := item.runner(ctx); err != nil {
					fmt.Printf("  %s stopped: %v\n", shareAddress(item.share), err)
				}
			}(item)
		}
		fmt.Printf("\n%sPress Ctrl+C to stop.%s\n\n", ansiGray, ansiReset)
		wg.Wait()
		fmt.Println("Project stopped.")
		return nil
	},
}

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List saved projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
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
		tunnels, err := fetchTunnels(cfg)
		if err != nil {
			return err
		}
		for _, p := range projects {
			label := "shares"
			if len(p.TunnelIDs) == 1 {
				label = "share"
			}
			line := fmt.Sprintf("%s\t%d %s", p.Name, len(p.TunnelIDs), label)
			if environments := projectShares(&p, tunnels); len(environments) > 1 {
				line += "\t" + strings.Join(environmentNames(environments), ", ")
			}
			fmt.Println(line)
		}
		return nil
	},
}

var initEnv string
var initForce bool

var initCmd = &cobra.Command{
	Use:   "init [project]",
	Short: "Link this folder to a project with a kexoz.json",
	Long: `Write a kexoz.json here so that kexoz start in this folder starts the
project without arguments. Commit it so everyone on the team gets the same
command. With one saved project, the name can be left out.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage, cmd.SilenceErrors = true, true
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		path := filepath.Join(cwd, projectFileName)
		if _, err := os.Stat(path); err == nil && !initForce {
			return fmt.Errorf("%s already exists; use --force to replace it", path)
		}
		cfg, err := clientConfig()
		if err != nil {
			return err
		}
		projects, err := fetchProjects(cfg)
		if err != nil {
			return err
		}
		var project *cliProject
		switch {
		case len(args) == 1:
			if project, err = findProject(projects, args[0]); err != nil {
				return err
			}
		case len(projects) == 1:
			project = &projects[0]
		default:
			_, err := findProject(projects, "")
			return fmt.Errorf("say which project: kexoz init <project>\n%v", err)
		}
		environment := environmentName(initEnv)
		if environment != "" {
			tunnels, err := fetchTunnels(cfg)
			if err != nil {
				return err
			}
			if byEnvironment := projectShares(project, tunnels); len(byEnvironment[environment]) == 0 {
				return fmt.Errorf("project %q has no %s environment; it has: %s", project.Name, environment, strings.Join(environmentNames(byEnvironment), ", "))
			}
		}
		if err := writeProjectFile(path, projectFile{Project: project.Name, Environment: environment}); err != nil {
			return err
		}
		fmt.Printf("Wrote %s. Run %skexoz start%s here to bring %s online.\n", path, ansiBold, ansiReset, project.Name)
		return nil
	},
}

func init() {
	startCmd.Flags().StringVarP(&startEnv, "env", "e", "", "Environment to start, such as staging (default: kexoz.json's, else main)")
	initCmd.Flags().StringVarP(&initEnv, "env", "e", "", "Environment kexoz start uses by default")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Replace an existing kexoz.json")
	rootCmd.AddCommand(startCmd, projectsCmd, initCmd)
}
