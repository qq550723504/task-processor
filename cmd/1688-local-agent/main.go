package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
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
	Scopes      string
	CreateURL   string
	BrowserPath string
	OpenBrowser bool
}

type jobCreator interface {
	CreateJob(context.Context, string) (localagent.Job, error)
}

type crawlerPreparer interface {
	Prepare(context.Context) error
}

type authorizeDeviceFunc func(context.Context, deviceauth.Config, deviceauth.Presenter) (string, error)

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
	scopes := strings.TrimSpace(cfg.Scopes)
	if scopes == "" {
		scopes = strings.TrimSpace(os.Getenv("ZITADEL_SCOPES"))
	}
	crawlerConfig := config.NewDefaultConfig()
	if strings.TrimSpace(cfg.BrowserPath) != "" {
		crawlerConfig.Browser.BrowserPath = strings.TrimSpace(cfg.BrowserPath)
	} else if installed := detectInstalledChrome(); installed != "" {
		crawlerConfig.Browser.BrowserPath = installed
	}
	crawler := a1688.NewLegacyProcessor(crawlerConfig)
	authConfig := deviceauth.Config{IssuerURL: cfg.IssuerURL, ClientID: cfg.ClientID, ProjectID: cfg.ProjectID, Scopes: scopes}
	var preparationErr error
	var token string
	if cfg.CreateURL != "" {
		token, err = validateAndPrepareAndAuthorize(ctx, crawler, deviceauth.Authorize, authConfig, terminalPresenter{openBrowser: cfg.OpenBrowser})
	} else {
		token, preparationErr, err = preparePendingAndAuthorize(ctx, crawler, deviceauth.Authorize, authConfig, terminalPresenter{openBrowser: cfg.OpenBrowser})
	}
	if err != nil {
		return err
	}
	jobs, err := client.New(cfg.APIBaseURL, token, nil)
	if err != nil {
		return err
	}
	var createdJobID string
	crawlerPrepared := preparationErr == nil
	if cfg.CreateURL != "" {
		createdJobID, err = createJob(ctx, jobs, cfg.CreateURL)
		if err != nil {
			return err
		}
		crawlerPrepared = true
	}
	outcome, err := (localagent.Runner{Jobs: jobs, Crawler: crawler, JobID: createdJobID, CrawlerPrepared: crawlerPrepared, PreparationError: preparationErr}).RunOnce(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("local-agent outcome: %s", outcome.State)
	if outcome.JobID != "" {
		fmt.Printf(" job=%s", outcome.JobID)
	}
	if outcome.EnvelopeSummary != nil {
		encodedSummary, marshalErr := json.Marshal(outcome.EnvelopeSummary)
		if marshalErr != nil {
			return fmt.Errorf("encode envelope summary: %w", marshalErr)
		}
		fmt.Printf(" envelope_summary=%s", encodedSummary)
	}
	fmt.Println()
	if outcome.State != localagent.OutcomeSucceeded {
		return fmt.Errorf("local-agent outcome was %s", outcome.State)
	}
	if outcome.EnvelopeSummary == nil {
		return errors.New("local-agent success response did not include an envelope summary")
	}
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
	flags.StringVar(&cfg.Scopes, "scopes", "", "OIDC scopes override (defaults to ListingKit roles; ZITADEL_SCOPES is also supported)")
	flags.StringVar(&cfg.CreateURL, "url", "", "one public 1688 offer URL to create before claiming")
	flags.StringVar(&cfg.BrowserPath, "browser-path", "", "local Chrome executable path (auto-detected when omitted)")
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

func detectInstalledChrome() string {
	for _, base := range []string{os.Getenv("LOCALAPPDATA"), os.Getenv("PROGRAMFILES"), os.Getenv("PROGRAMFILES(X86)")} {
		base = strings.TrimSpace(base)
		if base == "" || !filepath.IsAbs(base) {
			continue
		}
		candidate := filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe")
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

func validateCLIEndpoint(raw, name string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
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

func createPreparedJob(ctx context.Context, jobs jobCreator, crawler crawlerPreparer, rawURL string) (string, error) {
	if err := crawler.Prepare(ctx); err != nil {
		return "", fmt.Errorf("prepare crawler before creating job: %w", err)
	}
	return createJob(ctx, jobs, rawURL)
}

func createJob(ctx context.Context, jobs jobCreator, rawURL string) (string, error) {
	created, err := jobs.CreateJob(ctx, rawURL)
	if err != nil {
		return "", err
	}
	return created.ID, nil
}

func prepareAndAuthorize(ctx context.Context, crawler crawlerPreparer, authorize authorizeDeviceFunc, cfg deviceauth.Config, presenter deviceauth.Presenter) (string, error) {
	if err := crawler.Prepare(ctx); err != nil {
		return "", fmt.Errorf("prepare crawler before authorization: %w", err)
	}
	return authorize(ctx, cfg, presenter)
}

func validateAndPrepareAndAuthorize(ctx context.Context, crawler crawlerPreparer, authorize authorizeDeviceFunc, cfg deviceauth.Config, presenter deviceauth.Presenter) (string, error) {
	if err := deviceauth.ValidateConfig(cfg); err != nil {
		return "", err
	}
	return prepareAndAuthorize(ctx, crawler, authorize, cfg, presenter)
}

func preparePendingAndAuthorize(ctx context.Context, crawler crawlerPreparer, authorize authorizeDeviceFunc, cfg deviceauth.Config, presenter deviceauth.Presenter) (string, error, error) {
	if err := deviceauth.ValidateConfig(cfg); err != nil {
		return "", nil, err
	}
	preparationErr := crawler.Prepare(ctx)
	token, err := authorize(ctx, cfg, presenter)
	if err != nil {
		return "", preparationErr, err
	}
	return token, preparationErr, nil
}

func validateCLIOfferURL(raw string) (string, error) {
	clean := strings.TrimSpace(raw)
	parsed, err := url.Parse(clean)
	if err != nil || parsed.Scheme != "https" || !strings.EqualFold(parsed.Host, "detail.1688.com") || parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" || parsed.RawPath != "" || !strings.HasPrefix(parsed.Path, "/offer/") || !strings.HasSuffix(parsed.Path, ".html") || strings.TrimSpace(localagentURLID(parsed.Path)) == "" {
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
		if err := newVerificationURLCommand(verificationURI).Start(); err != nil {
			return errors.New("could not open device verification page")
		}
	}
	return nil
}

func newVerificationURLCommand(verificationURI string) *exec.Cmd {
	return exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", verificationURI)
}
