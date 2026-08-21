package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/integration/crawler/a1688"
	"task-processor/internal/localagent"
	"task-processor/internal/localagent/client"
	"task-processor/internal/localagent/deviceauth"
)

type cliConfig struct {
	APIBaseURL  string
	IssuerURL   string
	ClientID    string
	ProjectID   string
	CreateURL   string
	OpenBrowser bool
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	cfg, err := parseConfig(args)
	if err != nil {
		return err
	}
	token, err := deviceauth.Authorize(ctx, deviceauth.Config{IssuerURL: cfg.IssuerURL, ClientID: cfg.ClientID, ProjectID: cfg.ProjectID}, terminalPresenter{openBrowser: cfg.OpenBrowser})
	if err != nil {
		return err
	}
	jobs, err := client.New(cfg.APIBaseURL, token, nil)
	if err != nil {
		return err
	}
	if cfg.CreateURL != "" {
		if _, err := jobs.CreateJob(ctx, cfg.CreateURL); err != nil {
			return err
		}
	}
	crawler := a1688.NewLegacyProcessor(config.NewDefaultConfig())
	outcome, err := (localagent.Runner{Jobs: jobs, Crawler: crawler}).RunOnce(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("local-agent outcome: %s", outcome.State)
	if outcome.JobID != "" {
		fmt.Printf(" job=%s", outcome.JobID)
	}
	fmt.Println()
	return nil
}

func parseConfig(args []string) (cliConfig, error) {
	flags := flag.NewFlagSet("1688-local-agent", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var cfg cliConfig
	flags.StringVar(&cfg.APIBaseURL, "api-base-url", "", "authenticated task-processor API base URL")
	flags.StringVar(&cfg.IssuerURL, "issuer-url", "", "same-origin OIDC issuer URL")
	flags.StringVar(&cfg.ClientID, "client-id", "", "OIDC device client ID")
	flags.StringVar(&cfg.ProjectID, "project-id", "", "ZITADEL project ID")
	flags.StringVar(&cfg.CreateURL, "url", "", "one public 1688 offer URL to create before claiming")
	flags.BoolVar(&cfg.OpenBrowser, "open-browser", false, "open the device verification page")
	if err := flags.Parse(args); err != nil {
		return cliConfig{}, err
	}
	if strings.TrimSpace(cfg.APIBaseURL) == "" || strings.TrimSpace(cfg.IssuerURL) == "" || strings.TrimSpace(cfg.ClientID) == "" || strings.TrimSpace(cfg.ProjectID) == "" {
		return cliConfig{}, errors.New("-api-base-url, -issuer-url, -client-id, and -project-id are required")
	}
	if err := validateCLIEndpoint(cfg.APIBaseURL, "-api-base-url"); err != nil {
		return cliConfig{}, err
	}
	if err := validateCLIEndpoint(cfg.IssuerURL, "-issuer-url"); err != nil {
		return cliConfig{}, err
	}
	if cfg.CreateURL != "" {
		if _, err := validateCLIOfferURL(cfg.CreateURL); err != nil {
			return cliConfig{}, err
		}
	}
	return cfg, nil
}

func validateCLIEndpoint(raw, name string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%s must be an absolute HTTPS URI", name)
	}
	if parsed.Scheme == "https" {
		return nil
	}
	if parsed.Scheme == "http" && isLoopback(parsed.Hostname()) {
		return nil
	}
	return fmt.Errorf("%s must use HTTPS unless it is a literal loopback test endpoint", name)
}

func validateCLIOfferURL(raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	parsed, err := url.Parse(clean)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Hostname(), "detail.1688.com") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" || !strings.HasPrefix(parsed.Path, "/offer/") || strings.TrimSpace(localagentURLID(parsed.Path)) == "" {
		return "", errors.New("-url must be a public HTTPS detail.1688.com offer URL")
	}
	return clean, nil
}

func localagentURLID(path string) string {
	const prefix = "/offer/"
	if !strings.HasPrefix(path, prefix) {
		return ""
	}
	value := strings.TrimSuffix(strings.TrimPrefix(path, prefix), ".html")
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}

func isLoopback(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

type terminalPresenter struct{ openBrowser bool }

func (p terminalPresenter) Show(verificationURI, userCode string) error {
	fmt.Printf("Open %s and enter code %s\n", verificationURI, userCode)
	if p.openBrowser {
		if err := exec.Command("cmd", "/c", "start", "", verificationURI).Start(); err != nil {
			return errors.New("could not open device verification page")
		}
	}
	return nil
}
