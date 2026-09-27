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
	listen          string
	browserPath     string
	headless        bool
	timeout         time.Duration
	credential      string
	allowedOrigins  string
	shutdownTimeout time.Duration
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("1688-public-browser-collector", flag.ContinueOnError)
	var opts options
	fs.StringVar(&opts.listen, "listen", "127.0.0.1:19545", "listen address; keep on loopback unless the deployment provides network isolation")
	fs.StringVar(&opts.browserPath, "browser", "", "path to the Chromium binary (required)")
	fs.BoolVar(&opts.headless, "headless", true, "run the browser headless")
	fs.DurationVar(&opts.timeout, "timeout", 90*time.Second, "per-acquisition budget")
	fs.StringVar(&opts.credential, "credential", "", "service credential; defaults to $"+credentialEnvKey)
	fs.StringVar(&opts.allowedOrigins, "allowed-origins", strings.Join(browser.DefaultAllowedOrigins, ","), "comma-separated egress allowlist")
	fs.DurationVar(&opts.shutdownTimeout, "shutdown-timeout", 15*time.Second, "graceful shutdown budget")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if opts.browserPath == "" {
		return errors.New("-browser is required")
	}
	credential := strings.TrimSpace(opts.credential)
	if credential == "" {
		credential = strings.TrimSpace(os.Getenv(credentialEnvKey))
	}
	if credential == "" {
		// Fail closed: never start a collector whose callers cannot be verified.
		return fmt.Errorf("caller admission credential is required (set -credential or $%s)", credentialEnvKey)
	}
	origins := splitOrigins(opts.allowedOrigins)
	if len(origins) == 0 {
		return errors.New("-allowed-origins must not be empty: the collector must never have an unbounded egress allowlist")
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})

	provider := browser.New(browser.Options{
		ExecutablePath: opts.browserPath,
		Headless:       opts.headless,
		Budget:         opts.timeout,
		AllowedOrigins: origins,
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
