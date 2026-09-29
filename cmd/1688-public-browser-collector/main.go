// Command 1688-public-browser-collector runs the anonymous public 1688 browser
// acquisition collector as a standalone process (architecture design D13).
//
// It is deliberately a credential-free evidence producer:
//
//   - It holds NO database credentials and has NO database network
//     reachability. That is what makes the D12 egress boundary hold
//     structurally rather than by convention.
//   - It performs NO authorization and holds NO tenant identity. The RPC
//     request carries only the canonical public source URL.
//   - It writes NO operation rows. Idempotency, recovery, and publication all
//     stay in the current-application.
//
// The process refuses to start unless a caller-admission credential is
// configured (design A1): without it, any workload on the same network could
// drive Chromium and spend the shared browser/IP budget.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	browser "task-processor/internal/integration/acquisition/a1688/browser"
	"task-processor/internal/integration/acquisition/browsercollector"
)

const credentialEnvKey = "A1688_BROWSER_COLLECTOR_CREDENTIAL"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "1688-public-browser-collector:", err)
		os.Exit(1)
	}
}

type options struct {
	listen            string
	browserPath       string
	headless          bool
	timeout           time.Duration
	credential        string
	allowedOrigins    string
	shutdownTimeout   time.Duration
	minInterval       time.Duration
	startupQuarantine time.Duration
	jitter            float64
	challengeCooldown time.Duration
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("1688-public-browser-collector", flag.ContinueOnError)
	var opts options
	fs.StringVar(&opts.listen, "listen", "127.0.0.1:19545", "listen address; keep on loopback unless the deployment provides network isolation")
	fs.StringVar(&opts.browserPath, "browser", "", "path to the Chromium binary (required)")
	fs.BoolVar(&opts.headless, "headless", true, "run the browser headless")
	fs.DurationVar(&opts.timeout, "timeout", browser.DefaultTimeout, "per-acquisition budget; must stay below the application acquisition route budget (sourcing.AcquisitionTimeout)")
	fs.StringVar(&opts.allowedOrigins, "allowed-origins", strings.Join(browser.DefaultAllowedOrigins, ","), "comma-separated egress allowlist")
	fs.DurationVar(&opts.shutdownTimeout, "shutdown-timeout", 15*time.Second, "graceful shutdown budget")
	fs.DurationVar(&opts.minInterval, "min-interval", browser.DefaultMinInterval, "floor between acquisition starts; the 1688 challenge is frequency-triggered, so this is the primary control")
	fs.Float64Var(&opts.jitter, "jitter", browser.DefaultJitterFraction, "random extra fraction of min-interval, so collectors do not synchronise")
	fs.DurationVar(&opts.challengeCooldown, "challenge-cooldown", browser.DefaultChallengeCooldown, "how long to refuse work after a challenge is observed")
	fs.DurationVar(&opts.startupQuarantine, "startup-quarantine", 0, "how long a freshly started collector refuses its first request; 0 follows -challenge-cooldown; must not be negative")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if opts.browserPath == "" {
		return errors.New("-browser is required")
	}
	// The service credential is read from the environment only. A command-line
	// flag would expose it through the process table, container specs and shell
	// history, letting any co-located process or operator impersonate the
	// application and spend the shared browser/IP budget.
	credential := strings.TrimSpace(os.Getenv(credentialEnvKey))
	// The negative escape hatch on the quarantine exists for tests only. Accepting
	// it here would let an operator start with no restart protection and immediately
	// reuse an exit IP that was challenged moments earlier, which is exactly the
	// invariant the quarantine exists to provide.
	if opts.startupQuarantine < 0 {
		return fmt.Errorf("-startup-quarantine must not be negative")
	}
	if credential == "" {
		// Fail closed: never start a collector whose callers cannot be verified.
		return fmt.Errorf("caller admission credential is required (set $%s)", credentialEnvKey)
	}
	origins := splitOrigins(opts.allowedOrigins)
	if len(origins) == 0 {
		return errors.New("-allowed-origins must not be empty: the collector must never have an unbounded egress allowlist")
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	provider := browser.New(browser.Options{
		ExecutablePath:    opts.browserPath,
		Headless:          opts.headless,
		Budget:            opts.timeout,
		AllowedOrigins:    origins,
		MinInterval:       opts.minInterval,
		Jitter:            opts.jitter,
		ChallengeCooldown: opts.challengeCooldown,
		StartupQuarantine: opts.startupQuarantine,
	})
	handler, err := browsercollector.Handler(browsercollector.Options{
		Provider: provider,
		Admit:    browsercollector.SharedSecretAdmission(credential),
	})
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:              opts.listen,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		// The write timeout must exceed the acquisition budget so a legitimate
		// slow collection is not cut off mid-flight by the server.
		WriteTimeout: opts.timeout + 15*time.Second,
		IdleTimeout:  60 * time.Second,
	}
	listener, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return fmt.Errorf("listen %s: %w", opts.listen, err)
	}
	logger.WithField("listen", listener.Addr().String()).WithField("allowedOrigins", origins).Info("1688 public browser collector started")

	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
		close(errc)
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), opts.shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	logger.Info("1688 public browser collector stopped")
	return nil
}

func splitOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}
	return origins
}
