// Command 1688-batch-import runs one item of a local 1688 batch queue through the
// browser extension and the application capture page.
//
// It is the runnable entry point for design section 14 slice S1 ("one item driven
// end to end, with the terminal state read back"). It talks to a real 1688 page
// through the operator's own browser profile, so the actor and organization it is
// started for are treated as the operator's DECLARED expectation, never as consent:
// the batch's authoritative scope is read from the application and confirmed by a
// person at the moment the application first shows it (design section 4 D1.4).
//
// It does not create a server route, does not change the extension, and does not
// add a producer. See docs/superpowers/specs/2026-09-21-1688-batch-local-agent-design.md
// sections 4 D1.3, 4 D1.4 and 17.1.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
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

// offerURL reduces an accepted page to the one form this executor navigates: the
// canonical source the extension itself records.
//
// The scheme matters, and not cosmetically. The extension reads through its declared
// host grant (https://detail.1688.com/*), because activeTab can only be granted by a
// real action click and an unattended batch run has none. Queuing an http:// URL that
// does not redirect would navigate the browser to a host that grant does not cover,
// and the read would fail at executeScript with "manifest must request permission to
// access the respective host" after the item was already recorded. Query and fragment
// are dropped the same way pageSource drops them: they are not part of an offer's
// identity, and keeping them would let one offer be queued under two spellings and
// so submitted twice.
func offerURL(raw string) (string, bool) {
	match := offerPattern.FindStringSubmatch(raw)
	if match == nil {
		return "", false
	}
	return "https://detail.1688.com/offer/" + match[1] + ".html", true
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "1688-batch-import:", err)
		var stop *stopAndVerifyError
		if errors.As(err, &stop) {
			os.Exit(exitStopAndVerify)
		}
		os.Exit(exitUsage)
	}
}

// stopAndVerifyError marks every failure whose only safe next step is a person
// checking the application. They differ in how they are explained, not in what they
// ask for: after any of them the item may already have reached the application, so
// no automatic retry and no "just re-create the queue" is allowed.
type stopAndVerifyError struct{ err error }

func (e *stopAndVerifyError) Error() string { return e.err.Error() }
func (e *stopAndVerifyError) Unwrap() error { return e.err }

func run(args []string) error {
	flags := flag.NewFlagSet("1688-batch-import", flag.ContinueOnError)
	flags.Usage = func() {
		fmt.Fprint(flags.Output(), `Usage: 1688-batch-import --queue <file> --url <product-url> \
  --actor <id> --organization <id> --browser <chrome.exe> --extension <dist-dir>   --profile <dir> [--headless]

Drives one item of a local batch queue. --actor and --organization are the scope this
run EXPECTS. They are not consent: the batch is attributed only to the identity the
application reports as verified, after a person confirms it on the terminal. A scope
that disagrees with the application stops the batch. The queue file is created when
it does not exist, without a confirmed scope.

This command needs a person, so --headless cannot satisfy design section 4 D1.3/D1.4:
there is no visible window to clear a captcha or login wall in, and no one to confirm
the scope, so --headless is rejected instead of pretending those steps can happen.

Exit codes: 0 terminal result recorded, 3 stop and verify (outcome unknown, an
unreadable queue that may already hold a submitted item, or a blocked batch),
2 usage or other failure.
`)
	}
	var cfg config
	flags.StringVar(&cfg.QueuePath, "queue", "", "local batch queue file (created when missing)")
	flags.StringVar(&cfg.SourceURL, "url", "", "1688 product detail URL")
	flags.StringVar(&cfg.BatchID, "batch-id", "local-batch", "batch identifier recorded in a new queue")
	flags.StringVar(&cfg.ActorID, "actor", "", "actor id this run expects the application to report (required)")
	flags.StringVar(&cfg.Organization, "organization", "", "organization id this run expects the application to report (required)")
	flags.StringVar(&cfg.BrowserPath, "browser", "", "fingerprint browser executable (required)")
	flags.StringVar(&cfg.ExtensionDist, "extension", "", "unpacked extension directory (required)")
	flags.StringVar(&cfg.ProfileDir, "profile", "", "browser profile directory (required; the 1688 login lives here)")
	flags.BoolVar(&cfg.Headless, "headless", false, "run headless (only valid when nothing can require a person)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if err := cfg.validate(); err != nil {
		return err
	}
	// Design section 4 D1.3 keeps the browser visible for the person who has to clear
	// a login wall or a captcha, and section 4 D1.4 requires that same person to
	// confirm the batch scope. Neither step can happen in a headless run, so the mode
	// is refused up front rather than producing a prompt that points at no window.
	if cfg.Headless {
		return fmt.Errorf("--headless is not supported: a person must be able to clear a login wall or captcha in a visible window and confirm the batch scope")
	}

	// The queue is read before the browser is touched, so this failure must already
	// carry the stop-and-verify classification: a queue that cannot be read may hide
	// an item that was already handed over, and "ordinary failure" invites recreating
	// it.
	queuePath, err := ensureQueue(cfg)
	if err != nil {
		return describeStop(err, cfg.QueuePath)
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

	opts := batchcapture.ImportOptions{
		QueuePath: queuePath,
		Approved:  batchcapture.AppScope{ActorID: cfg.ActorID, OrganizationID: cfg.Organization},
		// The confirmation is read from the terminal the operator is looking at, not
		// from a flag, so a scripted run cannot answer it on a person's behalf.
		ConfirmScope: promptScopeConfirmation(os.Stdin, os.Stdout),
	}
	result, err := repeatWhileGateNeedsAHuman(
		func() (batchcapture.ImportResult, error) { return batchcapture.ImportOne(driver, opts) },
		func(r batchcapture.ImportResult, cause error) bool {
			return waitForGateCleared(os.Stdin, os.Stdout, r, cause)
		},
	)
	fmt.Printf("item %d %s\n  state: %s\n  idempotency key: %s\n  operation id: %s\n",
		result.Seq, result.URL, result.State, result.IdempotencyKey, result.OperationID)
	return describeStop(err, queuePath)
}

// repeatWhileGateNeedsAHuman is the design section 4 D1.3 pause: a challenge or a
// login wall stops the batch, a person clears it inside the browser the executor
// opened, and then the SAME item is re-done from navigation.
//
// The two callbacks are separate so the loop can be tested without a browser: the
// redo is only safe because a gate stop happens before any handoff, leaving the item
// re-capturable.
func repeatWhileGateNeedsAHuman(
	attempt func() (batchcapture.ImportResult, error),
	askToContinue func(batchcapture.ImportResult, error) bool,
) (batchcapture.ImportResult, error) {
	for {
		result, err := attempt()
		if !batchcapture.NeedsVisibleSession(result, err) || !askToContinue(result, err) {
			return result, err
		}
	}
}

// waitForGateCleared prints what the person has to do and blocks until they say it is
// done. The browser stays open for exactly this wait: it holds the locked-out session
// that only a person can restore.
func waitForGateCleared(in io.Reader, out io.Writer, result batchcapture.ImportResult, cause error) bool {
	fmt.Fprintf(out, "\nThe batch stopped before anything was submitted:\n  %v\n\n"+
		"Deal with the gate in the browser window that is still open (sign in, solve the\n"+
		"verification, or clear the risk-control page), then type y to redo item %d.\n"+
		"Anything else, including a bare Enter, stops.\nContinue? [y/N] ", cause, result.Seq)
	return readYes(in)
}

// promptScopeConfirmation is the CLI half of design section 4 D1.4: the executor
// shows the identity the APPLICATION verified and asks a person to confirm it before
// the batch's scope is bound.
func promptScopeConfirmation(in io.Reader, out io.Writer) func(batchcapture.AppScope) (bool, error) {
	return func(scope batchcapture.AppScope) (bool, error) {
		fmt.Fprintf(out, "\nThe application reports this verified identity for the batch:\n"+
			"  actor:        %s\n  organization: %s\n\n"+
			"Attribute this batch to that organization? [y/N] ", scope.ActorID, scope.OrganizationID)
		return readYes(in), nil
	}
}

// readYes reads one line and treats anything that is not an explicit yes as a
// refusal, including EOF. A missing answer is never a default: the whole point of
// the prompt is that a person looked at the value.
func readYes(in io.Reader) bool {
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

// describeStop turns any stop-for-a-person failure into the message the operator reads.
//
// It is a named function so the exit-code contract can be tested without a browser:
// the mapping is part of the interface, and getting it wrong is what would turn an
// ambiguous submit into a re-run.
//
// The unreadable queue is routed here for the same reason. Design section 4 D2.2
// treats damaged, truncated or unverifiable queue content as outcome_unknown, so
// reporting it as an ordinary failure would invite recreating or re-running the
// queue — exactly the duplicate publication the local queue exists to prevent. The
// distinction is *whether* the queue could be read, not what it said when it could:
// a mismatched approved scope stays an ordinary failure, because nothing is
// ambiguous about it.
func describeStop(err error, queuePath string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, batchcapture.ErrBatchBlocked) {
		return &stopAndVerifyError{err: fmt.Errorf(
			"%w\nVerify in the application before doing anything else. The queue file %s records which item blocked the batch; do not re-run it",
			err, queuePath)}
	}
	if errors.Is(err, batchcapture.ErrItemStateUnconfirmed) {
		// The durable file could not be updated, so it still holds the previous
		// record. Claiming the item is recorded as outcome_unknown would point the
		// operator at a label that is not in the file.
		return &stopAndVerifyError{err: fmt.Errorf(
			"%w\nThe queue file %s could not be updated, so it may still record this item as submitting rather than the outcome this run reached. Verify in the application before doing anything else; do not re-run it",
			err, queuePath)}
	}
	if errors.Is(err, batchcapture.ErrOutcomeUnknown) {
		return &stopAndVerifyError{err: fmt.Errorf(
			"%w\nVerify in the application before doing anything else. The queue file %s records the item as outcome_unknown; do not re-run it",
			err, queuePath)}
	}
	if errors.Is(err, batchcapture.ErrQueueCorrupt) {
		return &stopAndVerifyError{err: fmt.Errorf(
			"%w\nVerify in the application before doing anything else. The queue file %s cannot be read confidently, so an item in it may already have reached the application; do not recreate the queue and do not re-run the item",
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
	if _, ok := offerURL(c.SourceURL); !ok {
		return fmt.Errorf("--url is not a 1688 product detail URL: %s", c.SourceURL)
	}
	return nil
}

// ensureQueue creates the queue when it is absent and otherwise adds the item if it
// is not already recorded.
//
// A new queue is deliberately created WITHOUT a confirmed scope. Approval belongs to
// the identity the application reports and a person confirms at the first item's
// delivery (design section 4 D1.4); writing the flags' values here would mark an
// unverified expectation as consent, which is what a scripted run would then walk
// straight past. An existing approval is never rewritten.
func ensureQueue(cfg config) (string, error) {
	// The queue is the record the run navigates from, so the URL it stores is the
	// normalized one no matter how the flag was spelled.
	sourceURL, ok := offerURL(cfg.SourceURL)
	if !ok {
		return "", fmt.Errorf("--url is not a 1688 product detail URL: %s", cfg.SourceURL)
	}
	queue, err := batchcapture.Load(cfg.QueuePath)
	switch {
	case errors.Is(err, batchcapture.ErrQueueMissing):
		queue = batchcapture.NewQueue(cfg.BatchID)
		queue.Items = append(queue.Items, batchcapture.Item{Seq: 1, URL: sourceURL, State: batchcapture.ItemQueued})
		if err := queue.Save(cfg.QueuePath); err != nil {
			return "", err
		}
		fmt.Printf("created queue %s (no confirmed scope yet)\n", cfg.QueuePath)
		return cfg.QueuePath, nil
	case err != nil:
		return "", err
	}
	// Only a queue that already carries a confirmed scope can disagree with this run.
	// An unapproved queue has no scope to contradict; the confirmation happens later.
	if queue.ScopeApproved && !queue.ScopeMatches(cfg.ActorID, cfg.Organization) {
		return "", fmt.Errorf("queue %s is approved for %s/%s, not for %s/%s",
			cfg.QueuePath, queue.ApprovedActorID, queue.ApprovedOrganizationID, cfg.ActorID, cfg.Organization)
	}
	for _, item := range queue.Items {
		if item.URL == sourceURL {
			return cfg.QueuePath, nil
		}
	}
	seq := 1
	for _, item := range queue.Items {
		if item.Seq >= seq {
			seq = item.Seq + 1
		}
	}
	queue.Items = append(queue.Items, batchcapture.Item{Seq: seq, URL: sourceURL, State: batchcapture.ItemQueued})
	if err := queue.Save(cfg.QueuePath); err != nil {
		return "", err
	}
	return cfg.QueuePath, nil
}
