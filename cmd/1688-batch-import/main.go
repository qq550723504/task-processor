// Command 1688-batch-import runs one item of a local 1688 batch queue through the
// browser extension and the application capture page.
//
// It is the runnable entry point for design section 14 slice S1 ("one item driven
// end to end, with the terminal state read back"). It talks to a real 1688 page
// through the operator's own browser profile, so it refuses to run without an
// explicitly approved actor and organization and never invents one: the scope a
// human confirmed is the scope the submission is attributed to.
//
// It does not create a server route, does not change the extension, and does not
// add a producer. See docs/superpowers/specs/2026-09-21-1688-batch-local-agent-design.md
// sections 4 D1.4 and 17.1.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"task-processor/internal/batchcapture"
)

// Exit codes are part of the interface, because the whole point of the local queue
// is that a person can tell "done" from "stop and check".
const (
	exitOK    = 0
	exitUsage = 2
	// exitStopAndVerify is used both for an unknown outcome and for a blocked
	// batch. They are the same instruction to the operator — stop and check the
	// application before touching the queue again — and must never be confused with
	// a usage error, which would invite a re-run.
	exitStopAndVerify = 3
)

type config struct {
	QueuePath     string
	SourceURL     string
	BatchID       string
	ActorID       string
	Organization  string
	BrowserPath   string
	ExtensionDist string
	ProfileDir    string
	Headless      bool
}

// offerPattern mirrors the extension's own pageSource rule, so a URL this command
// accepts is a URL the extension would also accept.
var offerPattern = regexp.MustCompile(`^https?://detail\.1688\.com(?::(?:80|443))?/offer/([1-9][0-9]{0,19})\.html(?:[?#].*)?$`)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "1688-batch-import:", err)
		var unknown *outcomeUnknownError
		if errors.As(err, &unknown) {
			os.Exit(exitStopAndVerify)
		}
		var blocked *batchBlockedError
		if errors.As(err, &blocked) {
			os.Exit(exitStopAndVerify)
		}
		os.Exit(exitUsage)
	}
}

// outcomeUnknownError marks the one failure a caller must treat differently: the
// item may or may not have been published, so the only allowed next step is a
// human check, never an automatic retry.
type outcomeUnknownError struct{ err error }

func (e *outcomeUnknownError) Error() string { return e.err.Error() }
func (e *outcomeUnknownError) Unwrap() error { return e.err }

// batchBlockedError marks the other stop-for-a-person outcome: an earlier item in
// the queue may already have been published, so no further item may be submitted
// until someone checks the application. It is not a usage error.
type batchBlockedError struct{ err error }

func (e *batchBlockedError) Error() string { return e.err.Error() }
func (e *batchBlockedError) Unwrap() error { return e.err }

func run(args []string) error {
	flags := flag.NewFlagSet("1688-batch-import", flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), `Usage: 1688-batch-import --queue <file> --url <product-url> \
  --actor <id> --organization <id> --browser <chrome.exe> --extension <dist-dir>   --profile <dir> [--headless]

Drives one item of a local batch queue. The actor and organization must be the
values a person confirmed in the application; they are never read from the browser
session. The queue file is created when it does not exist.

Exit codes: 0 terminal result recorded, 3 stop and verify (outcome unknown or a
blocked batch), 2 usage or other failure.
`)
	}
	var cfg config
	flags.StringVar(&cfg.QueuePath, "queue", "", "local batch queue file (created when missing)")
	flags.StringVar(&cfg.SourceURL, "url", "", "1688 product detail URL")
	flags.StringVar(&cfg.BatchID, "batch-id", "local-batch", "batch identifier recorded in a new queue")
	flags.StringVar(&cfg.ActorID, "actor", "", "approved actor id (required)")
	flags.StringVar(&cfg.Organization, "organization", "", "approved organization id (required)")
	flags.StringVar(&cfg.BrowserPath, "browser", "", "fingerprint browser executable (required)")
	flags.StringVar(&cfg.ExtensionDist, "extension", "", "unpacked extension directory (required)")
	flags.StringVar(&cfg.ProfileDir, "profile", "", "browser profile directory (required; the 1688 login lives here)")
	flags.BoolVar(&cfg.Headless, "headless", false, "run headless")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := cfg.validate(); err != nil {
		return err
	}

	queuePath, err := ensureQueue(cfg)
	if err != nil {
		return err
	}

	driver, err := batchcapture.LaunchDriver(batchcapture.DriverOptions{
		ExecutablePath: cfg.BrowserPath,
		ProfileDir:     cfg.ProfileDir,
		ExtensionDist:  cfg.ExtensionDist,
		Headless:       cfg.Headless,
	})
	if err != nil {
		return fmt.Errorf("launch browser: %w", err)
	}
	defer func() { _ = driver.Close() }()

	result, err := batchcapture.ImportOne(driver, batchcapture.ImportOptions{
		QueuePath: queuePath,
		Approved:  batchcapture.AppScope{ActorID: cfg.ActorID, OrganizationID: cfg.Organization},
	})
	fmt.Printf("item %d %s\n  state: %s\n  idempotency key: %s\n  operation id: %s\n",
		result.Seq, result.URL, result.State, result.IdempotencyKey, result.OperationID)
	return describeStop(err, queuePath)
}

// describeStop turns a stop-for-a-person failure into the message the operator reads.
//
// It is a named function so the exit-code contract can be tested without a browser:
// the mapping is part of the interface, and getting it wrong is what would turn an
// ambiguous submit into a re-run.
func describeStop(err error, queuePath string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, batchcapture.ErrBatchBlocked) {
		return &batchBlockedError{err: fmt.Errorf(
			"%w\nVerify in the application before doing anything else. The queue file %s records which item blocked the batch; do not re-run it",
			err, queuePath)}
	}
	if errors.Is(err, batchcapture.ErrOutcomeUnknown) {
		return &outcomeUnknownError{err: fmt.Errorf(
			"%w\nVerify in the application before doing anything else. The queue file %s records the item as outcome_unknown; do not re-run it",
			err, queuePath)}
	}
	return err
}

func (c config) validate() error {
	required := []struct {
		name  string
		value string
	}{
		{"--queue", c.QueuePath},
		{"--url", c.SourceURL},
		{"--actor", c.ActorID},
		{"--organization", c.Organization},
		{"--browser", c.BrowserPath},
		{"--extension", c.ExtensionDist},
		{"--profile", c.ProfileDir},
	}
	missing := make([]string, 0, len(required))
	for _, flagValue := range required {
		if strings.TrimSpace(flagValue.value) == "" {
			missing = append(missing, flagValue.name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required flags: %s", strings.Join(missing, ", "))
	}
	if !offerPattern.MatchString(c.SourceURL) {
		return fmt.Errorf("--url is not a 1688 product detail URL: %s", c.SourceURL)
	}
	return nil
}

// ensureQueue creates the queue when it is absent and otherwise adds the item if it
// is not already recorded. An existing queue's approved scope is never rewritten:
// approval belongs to the run that a person confirmed.
func ensureQueue(cfg config) (string, error) {
	queue, err := batchcapture.Load(cfg.QueuePath)
	switch {
	case errors.Is(err, batchcapture.ErrQueueMissing):
		queue = batchcapture.NewQueue(cfg.BatchID)
		if err := queue.ApproveScope(cfg.ActorID, cfg.Organization); err != nil {
			return "", err
		}
		queue.Items = append(queue.Items, batchcapture.Item{Seq: 1, URL: cfg.SourceURL, State: batchcapture.ItemQueued})
		if err := queue.Save(cfg.QueuePath); err != nil {
			return "", err
		}
		fmt.Printf("created queue %s\n", cfg.QueuePath)
		return cfg.QueuePath, nil
	case err != nil:
		return "", err
	}
	if !queue.ScopeMatches(cfg.ActorID, cfg.Organization) {
		return "", fmt.Errorf("queue %s is approved for %s/%s, not for %s/%s",
			cfg.QueuePath, queue.ApprovedActorID, queue.ApprovedOrganizationID, cfg.ActorID, cfg.Organization)
	}
	for _, item := range queue.Items {
		if item.URL == cfg.SourceURL {
			return cfg.QueuePath, nil
		}
	}
	seq := 1
	for _, item := range queue.Items {
		if item.Seq >= seq {
			seq = item.Seq + 1
		}
	}
	queue.Items = append(queue.Items, batchcapture.Item{Seq: seq, URL: cfg.SourceURL, State: batchcapture.ItemQueued})
	if err := queue.Save(cfg.QueuePath); err != nil {
		return "", err
	}
	return cfg.QueuePath, nil
}
