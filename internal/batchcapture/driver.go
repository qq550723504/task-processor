package batchcapture

// Driver owns the browser side of one batch run. It launches the fingerprint
// browser the operator already uses, loads the shipped extension into it, and
// exposes the two surfaces the design depends on:
//
//   - the product page, observed by the executor itself (design section 4 D1.3)
//   - the extension's real action popup, whose real controls are clicked rather
//     than bypassed with internal messages (design section 4 D1.2, guard 2)
//
// Two mechanisms here are not obvious and were established by direct measurement
// on ungoogled Chromium 144, the browser this account's profile was created with:
//
//  1. Chromium 144 does not expose the CDP method `Extensions.triggerAction` (its
//     Extensions domain has only loadUnpacked/uninstall/*StorageItems). The real
//     action popup is therefore opened with `chrome.action.openPopup()` evaluated
//     in the extension's own service worker.
//  2. Playwright does not surface the action popup as a Page, so it is driven
//     through a raw CDP session obtained with Target.attachToTarget and
//     Target.sendMessageToTarget.
//
// Both are load-bearing: without them the popup's real controls cannot be
// clicked, and clicking the real controls is what design section 4 D1.2 guard 2
// requires.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// DefaultControlTimeout bounds every wait for a page, popup or target.
const DefaultControlTimeout = 30 * time.Second

var (
	// ErrDriverUnavailable reports that the browser, extension or a CDP surface
	// could not be brought up, so no observation can be made at all.
	ErrDriverUnavailable = errors.New("batch driver unavailable")
	// ErrControlNotFound reports that an expected control or target is absent,
	// which means the shipped extension and this executor have diverged.
	ErrControlNotFound = errors.New("extension control not found")
)

// DriverOptions configures one browser session.
type DriverOptions struct {
	// ExecutablePath is the browser binary. The design requires the fingerprint
	// browser, and the operator profile must have been created by that same
	// binary version, or the browser refuses to open it.
	ExecutablePath string
	// ProfileDir holds browser session state. It is opened read/write because the
	// browser writes to it; the executor never reads credentials from it.
	ProfileDir string
	// ExtensionDist is the unpacked extension directory to load.
	ExtensionDist string
	// Headless selects headless operation. The action popup is opened through the
	// extension API rather than a synthetic window click, so headless works.
	Headless bool
	// ControlTimeout overrides DefaultControlTimeout when non-zero.
	ControlTimeout time.Duration
}

func (o DriverOptions) timeout() time.Duration {
	if o.ControlTimeout > 0 {
		return o.ControlTimeout
	}
	return DefaultControlTimeout
}

func (o DriverOptions) validate() error {
	if o.ExecutablePath == "" {
		return fmt.Errorf("%w: ExecutablePath is required", ErrDriverUnavailable)
	}
	if _, err := os.Stat(o.ExecutablePath); err != nil {
		return fmt.Errorf("%w: browser binary: %v", ErrDriverUnavailable, err)
	}
	if o.ProfileDir == "" {
		return fmt.Errorf("%w: ProfileDir is required", ErrDriverUnavailable)
	}
	if o.ExtensionDist == "" {
		return fmt.Errorf("%w: ExtensionDist is required", ErrDriverUnavailable)
	}
	if info, err := os.Stat(o.ExtensionDist); err != nil || !info.IsDir() {
		return fmt.Errorf("%w: extension directory %q is not readable", ErrDriverUnavailable, o.ExtensionDist)
	}
	return nil
}

// Driver is a live browser session. It is not safe for concurrent use: the design
// runs exactly one executor per profile (section 4 D3).
type Driver struct {
	pw             *playwright.Playwright
	context        playwright.BrowserContext
	browserSession playwright.CDPSession
	extensionID    string
	timeout        time.Duration

	// source is the executor-owned product tab. It is tracked explicitly because
	// the handoff step opens an application tab, so "the last page" is not the
	// product page once a handoff has happened.
	source playwright.Page
}

// LaunchDriver starts the browser, loads the extension and returns a ready
// driver. A non-nil error means nothing was left running.
func LaunchDriver(opts DriverOptions) (*Driver, error) {
	if err := opts.validate(); err != nil {
		return nil, err
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, fmt.Errorf("%w: start playwright: %v", ErrDriverUnavailable, err)
	}
	context, err := pw.Chromium.LaunchPersistentContext(opts.ProfileDir, playwright.BrowserTypeLaunchPersistentContextOptions{
		ExecutablePath: playwright.String(opts.ExecutablePath),
		Headless:       playwright.Bool(opts.Headless),
		// The extension is essential here, so the default argument that disables
		// extensions has to be dropped explicitly.
		IgnoreDefaultArgs: []string{"--disable-extensions"},
		Args: []string{
			"--no-first-run",
			"--no-default-browser-check",
			// Required for Extensions.loadUnpacked against the live browser.
			"--enable-unsafe-extension-debugging",
		},
		Viewport: nil,
	})
	if err != nil {
		_ = pw.Stop()
		return nil, fmt.Errorf("%w: launch browser: %v", ErrDriverUnavailable, err)
	}

	driver := &Driver{pw: pw, context: context, timeout: opts.timeout()}
	session, err := context.Browser().NewBrowserCDPSession()
	if err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("%w: open CDP session: %v", ErrDriverUnavailable, err)
	}
	driver.browserSession = session

	loaded, err := session.Send("Extensions.loadUnpacked", map[string]any{
		"path": filepath.ToSlash(opts.ExtensionDist),
	})
	if err != nil {
		_ = driver.Close()
		return nil, fmt.Errorf("%w: load extension: %v", ErrDriverUnavailable, err)
	}
	payload, _ := loaded.(map[string]any)
	id, _ := payload["id"].(string)
	if id == "" {
		_ = driver.Close()
		return nil, fmt.Errorf("%w: extension load returned no id", ErrDriverUnavailable)
	}
	driver.extensionID = id
	return driver, nil
}

// Close releases the browser. It is safe to call more than once.
func (d *Driver) Close() error {
	if d == nil {
		return nil
	}
	var first error
	if d.context != nil {
		if err := d.context.Close(); err != nil && first == nil {
			first = err
		}
		d.context = nil
	}
	if d.pw != nil {
		if err := d.pw.Stop(); err != nil && first == nil {
			first = err
		}
		d.pw = nil
	}
	return first
}

// ExtensionID is the loaded extension's id. The handoff URL and the application's
// capture page both depend on it.
func (d *Driver) ExtensionID() string { return d.extensionID }

// OpenSourcePage navigates the executor-owned tab to a product URL and brings it
// to the front, which the action popup requires.
//
// A navigation error is deliberately not returned: the executor must still
// observe whatever the browser actually landed on, because a challenge page, a
// delisted product and a network failure are three different verdicts.
func (d *Driver) OpenSourcePage(url string) {
	if d.source == nil {
		page, err := d.context.NewPage()
		if err != nil {
			return
		}
		d.source = page
	}
	if _, err := d.source.Goto(url, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(float64(d.timeout.Milliseconds())),
	}); err != nil {
		// Ignore: Observe reports the resulting page state to the classifier.
		_ = err
	}
	_ = d.source.BringToFront()
}

// SourcePage exposes the executor-owned product tab.
func (d *Driver) SourcePage() playwright.Page { return d.source }

// Context exposes the browser session the driver owns. The batch orchestrator
// needs it to observe browser state it owns (the application tab the extension
// opens during handoff, and the response substitution used by fixture runs); it
// is not a general extension point.
func (d *Driver) Context() playwright.BrowserContext { return d.context }

// Observe reads the page the executor is currently looking at. It reports what
// the executor can see rather than trusting any extension reply: the extension
// collapses every capture failure to ACTION_UNAVAILABLE, so it can never report a
// reason, and design section 4 D1.3 therefore makes the page the authority.
func (d *Driver) Observe(productDataPresent bool) Observation {
	if d.source == nil {
		return Observation{}
	}
	observation := Observation{FinalURL: d.source.URL(), Performed: true}
	if text, err := d.source.Locator("body").InnerText(); err == nil {
		observation.BodyText = text
	}
	// Risk-control markers live in attributes and inline scripts, not in visible
	// text, so the page source is what the classifier has to see. A failure here
	// still counts as performed: the URL alone remains a valid observation.
	if html, err := d.source.Locator("html").InnerHTML(); err == nil {
		observation.RawHTML = html
	}
	observation.ProductDataPresent = productDataPresent
	return observation
}

// ClassifyCurrentPage observes the current page and returns the verdict the batch
// loop must act on.
func (d *Driver) ClassifyCurrentPage(productDataPresent bool) Verdict {
	return Classify(d.Observe(productDataPresent))
}

// ObserveCurrentPage reads the page a judgment has to be made from, before any
// capture has been attempted. The orchestrator uses it so the challenge judgment can
// be applied first, exactly as Classify orders it, instead of declaring that product
// data is present.
func (d *Driver) ObserveCurrentPage() Observation {
	return d.Observe(false)
}

// PrepareItem navigates to one product URL and returns a popup that is ready to
// capture it.
//
// Two measured behaviours force this to be a single operation rather than two
// calls the caller sequences itself:
//
//   - The controller is a background module singleton (background.ts:25) whose
//     capture() returns early while it already holds a payload, so a capture the
//     previous item left behind would be handed off as the next item.
//   - The action popup closes when the handoff opens the application tab, so the
//     popup attached for the previous item is gone by the next item.
//
// Reopening the popup and then driving its real two-step reset is therefore the
// per-item boundary design section 4 D1.1 requires.
func (d *Driver) PrepareItem(sourceURL string) (*Popup, error) {
	d.OpenSourcePage(sourceURL)
	popup, err := d.OpenPopup()
	if err != nil {
		return nil, err
	}
	if err := popup.Reset(); err != nil {
		_ = popup.Close()
		return nil, err
	}
	return popup, nil
}

// OpenPopup opens the extension's real action popup and attaches to it.
//
// The popup is opened from the extension's own service worker because Chromium
// 144 has no Extensions.triggerAction. `chrome.action.openPopup()` produces the
// genuine action popup, which matters: the extension accepts popup messages only
// when the sender is popup.html with no associated tab, so a popup page opened as
// a normal tab is rejected by design.
func (d *Driver) OpenPopup() (*Popup, error) {
	popupURL := "chrome-extension://" + d.extensionID + "/popup.html"
	// Chrome transiently refuses to open an action popup while the previous one is
	// still closing, which is the state left behind whenever the previous item's
	// handoff has just opened the application tab. Wait for the old target to be
	// gone and then retry, rather than reporting a driver failure for a timing
	// artifact.
	if err := d.waitForNoTarget(popupURL, d.timeout); err != nil {
		return nil, err
	}
	worker, err := d.extensionWorker()
	if err != nil {
		return nil, err
	}
	if worker == nil {
		return nil, fmt.Errorf("%w: extension service worker not visible", ErrDriverUnavailable)
	}
	// The popup only opens for a focused window, so the product tab must be in
	// front. This is also what makes the extension's activeTab lookup address the
	// product tab rather than the application tab.
	if d.source != nil {
		_ = d.source.BringToFront()
	}
	if err := d.openActionPopup(worker); err != nil {
		return nil, err
	}
	targetID, err := d.waitForTarget(popupURL)
	if err != nil {
		return nil, err
	}
	attached, err := d.browserSession.Send("Target.attachToTarget", map[string]any{
		"targetId": targetID, "flatten": false,
	})
	if err != nil {
		return nil, fmt.Errorf("%w: attach to popup: %v", ErrDriverUnavailable, err)
	}
	m, _ := attached.(map[string]any)
	sessionID, _ := m["sessionId"].(string)
	if sessionID == "" {
		return nil, fmt.Errorf("%w: popup attach returned no session", ErrDriverUnavailable)
	}
	popup := &Popup{
		raw:      newRawSession(d.browserSession, sessionID),
		driver:   d,
		timeout:  d.timeout,
		attached: true,
	}
	if err := popup.WaitReady(); err != nil {
		_ = popup.Close()
		return nil, err
	}
	return popup, nil
}

// extensionWorker finds the extension's background service worker. Playwright
// does surface extension service workers, which is what makes openPopup callable.
func (d *Driver) extensionWorker() (playwright.Worker, error) {
	want := "chrome-extension://" + d.extensionID + "/background.js"
	deadline := time.Now().Add(d.timeout)
	for time.Now().Before(deadline) {
		for _, worker := range d.context.ServiceWorkers() {
			if worker.URL() == want {
				return worker, nil
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	return nil, nil
}

// openActionPopup asks the extension to open its action popup. The only failure
// mode this repository has measured is Chrome refusing while a previous popup has
// not finished closing, and the caller removes that state before calling this, so
// a single attempt is deliberate: an unexplained retry would hide the real
// refusal instead of reporting it.
func (d *Driver) openActionPopup(worker playwright.Worker) error {
	if _, err := worker.Evaluate("chrome.action.openPopup()"); err != nil {
		return fmt.Errorf("%w: open action popup: %v", ErrDriverUnavailable, err)
	}
	return nil
}

// waitForNoTarget waits until no target has the given URL. It returns an error
// only if one is still present when the timeout expires, because attaching to a
// half-closed popup target would read a document that is being torn down.
func (d *Driver) waitForNoTarget(wantURL string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		id, err := d.findTarget(wantURL)
		if err != nil {
			return err
		}
		if id == "" {
			return nil
		}
		// Actively close a lingering popup target so the next open can succeed.
		_, _ = d.browserSession.Send("Target.closeTarget", map[string]any{"targetId": id})
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("%w: previous popup target %s did not close", ErrDriverUnavailable, wantURL)
}

// waitForTarget polls Target.getTargets until the wanted URL appears.
func (d *Driver) waitForTarget(wantURL string) (string, error) {
	deadline := time.Now().Add(d.timeout)
	for time.Now().Before(deadline) {
		id, err := d.findTarget(wantURL)
		if err != nil {
			return "", err
		}
		if id != "" {
			return id, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", fmt.Errorf("%w: target %s did not appear", ErrDriverUnavailable, wantURL)
}

// findTarget returns the first target whose URL matches exactly.
func (d *Driver) findTarget(wantURL string) (string, error) {
	res, err := d.browserSession.Send("Target.getTargets", nil)
	if err != nil {
		return "", fmt.Errorf("%w: list targets: %v", ErrDriverUnavailable, err)
	}
	m, _ := res.(map[string]any)
	infos, _ := m["targetInfos"].([]any)
	for _, raw := range infos {
		info, _ := raw.(map[string]any)
		url, _ := info["url"].(string)
		if url != wantURL {
			continue
		}
		if id, _ := info["targetId"].(string); id != "" {
			return id, nil
		}
	}
	return "", nil
}

// waitForAppTab waits for the application tab the extension opens during
// handoff, and returns its URL. That URL is the executor's only source for the
// idempotency key the extension generated, so a missing tab is an error rather
// than a detail.
//
// Only a tab that did not exist before the handoff click is accepted. The
// application tab of the previous item is normally still open, and its URL still
// carries that item's key, so adopting any matching tab would hand the executor
// item one's key while it is capturing item two.
//
// Both fragment forms are accepted because the application normalises the handoff
// fragment to `#operationKey=<key>` as soon as it reads the payload, so which form
// is visible depends only on how quickly the tab is observed.
func (d *Driver) waitForAppTab(existing map[string]bool) (string, error) {
	deadline := time.Now().Add(d.timeout)
	for time.Now().Before(deadline) {
		res, err := d.browserSession.Send("Target.getTargets", nil)
		if err != nil {
			return "", fmt.Errorf("%w: list targets: %v", ErrDriverUnavailable, err)
		}
		m, _ := res.(map[string]any)
		infos, _ := m["targetInfos"].([]any)
		for _, raw := range infos {
			info, _ := raw.(map[string]any)
			typ, _ := info["type"].(string)
			url, _ := info["url"].(string)
			targetID, _ := info["targetId"].(string)
			if typ != "page" || url == "" || url == "about:blank" {
				continue
			}
			if existing[targetID] {
				continue
			}
			if url == "chrome-extension://"+d.extensionID+"/popup.html" {
				continue
			}
			ref, err := captureRefFromURL(url)
			if err != nil {
				continue
			}
			// A handoff opened by a different extension build must not be adopted:
			// its key belongs to a capture this executor never made. The normalised
			// form carries no extension id, so the check only applies when present.
			if ref.ExtensionID != "" && ref.ExtensionID != d.extensionID {
				continue
			}
			return url, nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return "", fmt.Errorf("%w: application tab never opened", ErrDriverUnavailable)
}

// pageTargetIDs snapshots the page targets that already exist, so a handoff can be
// required to produce a tab outside that set.
func (d *Driver) pageTargetIDs() map[string]bool {
	seen := map[string]bool{}
	res, err := d.browserSession.Send("Target.getTargets", nil)
	if err != nil {
		return seen
	}
	m, _ := res.(map[string]any)
	infos, _ := m["targetInfos"].([]any)
	for _, raw := range infos {
		info, _ := raw.(map[string]any)
		typ, _ := info["type"].(string)
		if typ != "page" {
			continue
		}
		if id, ok := info["targetId"].(string); ok {
			seen[id] = true
		}
	}
	return seen
}

// rawSession drives a non-flat CDP session. Playwright's CDPSession.Send carries
// no sessionId, so a session obtained with Target.attachToTarget(flatten:false) is
// reachable only through Target.sendMessageToTarget, and its replies arrive as
// Target.receivedMessageFromTarget events.
type rawSession struct {
	session playwright.CDPSession

	// sessionID addresses the attached target; targetID names it. They are not
	// interchangeable, which is why both are retained.
	sessionID string
	targetID  string

	mu      sync.Mutex
	nextID  int
	pending map[int]chan map[string]any
	timeout time.Duration
}

func newRawSession(session playwright.CDPSession, sessionID string) *rawSession {
	r := &rawSession{
		session:   session,
		sessionID: sessionID,
		pending:   map[int]chan map[string]any{},
		timeout:   DefaultControlTimeout,
	}
	session.On("Target.receivedMessageFromTarget", func(params map[string]any) {
		// Only replies for this session may be resolved, or two attached targets
		// could consume each other's responses.
		if sid, _ := params["sessionId"].(string); sid != r.sessionID {
			return
		}
		raw, _ := params["message"].(string)
		var msg map[string]any
		if json.Unmarshal([]byte(raw), &msg) != nil {
			return
		}
		id, ok := msg["id"].(float64)
		if !ok {
			return
		}
		r.mu.Lock()
		ch := r.pending[int(id)]
		delete(r.pending, int(id))
		r.mu.Unlock()
		if ch != nil {
			select {
			case ch <- msg:
			default:
			}
		}
	})
	return r
}

func (r *rawSession) send(method string, params map[string]any) (map[string]any, error) {
	r.mu.Lock()
	r.nextID++
	id := r.nextID
	ch := make(chan map[string]any, 1)
	r.pending[id] = ch
	r.mu.Unlock()

	inner, err := json.Marshal(map[string]any{"id": id, "method": method, "params": params})
	if err != nil {
		r.forget(id)
		return nil, err
	}
	if _, err := r.session.Send("Target.sendMessageToTarget", map[string]any{
		"sessionId": r.sessionID, "message": string(inner),
	}); err != nil {
		r.forget(id)
		return nil, fmt.Errorf("%w: send %s: %v", ErrDriverUnavailable, method, err)
	}
	select {
	case msg := <-ch:
		if e, ok := msg["error"]; ok {
			b, _ := json.Marshal(e)
			return nil, fmt.Errorf("%w: %s: %s", ErrDriverUnavailable, method, b)
		}
		res, _ := msg["result"].(map[string]any)
		return res, nil
	case <-time.After(r.timeout):
		r.forget(id)
		return nil, fmt.Errorf("%w: %s timed out", ErrDriverUnavailable, method)
	}
}

func (r *rawSession) forget(id int) {
	r.mu.Lock()
	delete(r.pending, id)
	r.mu.Unlock()
}

// evaluate runs a synchronous expression in the attached document and returns its
// value. `userGesture` is always set so that a control click carries the same
// privilege a user click has. No API here lets a caller read and then click in two
// separate evaluations on the application page; the popup has no such constraint
// because the extension reads its own state when the click is handled.
func (r *rawSession) evaluate(expression string) (any, error) {
	res, err := r.send("Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
		"awaitPromise":  true,
		"userGesture":   true,
	})
	if err != nil {
		return nil, err
	}
	if details, ok := res["exceptionDetails"]; ok {
		b, _ := json.Marshal(details)
		return nil, fmt.Errorf("%w: evaluate: %s", ErrControlNotFound, b)
	}
	result, _ := res["result"].(map[string]any)
	return result["value"], nil
}

func (r *rawSession) close() error {
	_, err := r.session.Send("Target.detachFromTarget", map[string]any{"sessionId": r.sessionID})
	return err
}
