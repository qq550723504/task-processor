package batchcapture

// The browser-side tests here are fixture-only: they substitute the public 1688
// page response and a transport-only receiver, exactly as
// extensions/1688-capture/scripts/browser-smoke.mjs does. They cannot prove 1688
// or backend acceptance, and they deliberately never touch a real 1688 account
// (design section 17 item 8).
//
// They are skipped unless both a fingerprint browser binary and a fixture
// extension build are supplied, because neither belongs in CI: the browser is a
// large local artifact and the fixture build is a generated directory.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"
)

const (
	fixturePort      = "4399"
	fixtureOfferID   = "981645030344"
	fixtureSource    = "https://detail.1688.com/offer/" + fixtureOfferID + ".html"
	fixtureAppURL    = "http://127.0.0.1:" + fixturePort + "/capture/1688"
	fixtureAppPrefix = "http://127.0.0.1:" + fixturePort + "/capture/1688"
)

// The fixture application reports operation ids in the UUID shape the real result
// contract requires (isAcquisitionUUID in product-acquisition.ts). A stub value such
// as "fixture-operation-1" is not merely unrealistic here: accepting one is exactly
// the behaviour the driver must refuse, so a fixture that used one would make the
// real contract untestable.
const (
	fixtureOperationID = "3f1c8a52-6f5e-4a1d-9c47-8f2a5b0d6e31"
	fixtureOtherID     = "7b2d4c19-8e3a-4f60-b5d2-1a9c7e4f8036"
)

// fixtureReceiver is the transport-only application stand-in. It performs the
// same `capture.read` handshake the real capture page performs, presents the same
// machine-readable scope/submit contract the executor drives, and records what it
// received, so the test can tell whether the extension actually produced a payload
// that reached an application context.
//
// It is not the real receiver: it does not submit, does not authenticate, and does
// not prove any backend behaviour. Its scope values and terminal outcome are
// settable so the guard paths can be exercised deliberately.
const fixtureReceiver = `<!doctype html><html lang="zh-CN"><head><meta charset="UTF-8"><meta name="referrer" content="no-referrer"><title>Batch Fixture Receiver</title></head><body>
<h1>Transport Fixture — 不是真实后端</h1><p id="status">等待受限capture消息</p>
<dl><dt>Current verified account</dt><dd data-batch-scope-actor></dd>
<dt>Effective enterprise</dt><dd data-batch-scope-organization></dd></dl>
<button id="confirm" data-batch-confirm-submit disabled>Confirm and submit</button>
<div id="refusal" data-batch-submit-refusal hidden></div>
<div id="result" data-batch-submit-result=""></div>
<pre id="summary"></pre>
<script>
const params = new URLSearchParams(location.hash.slice(1));
const extensionId=params.get('extensionId'),handoffId=params.get('handoffId'),idempotencyKey=params.get('idempotencyKey');
window.fixtureReceived=null;
window.fixtureStatus='INIT';
window.__fixtureClicks=0;
window.__fixtureScope=FIXTURE_SCOPE_PLACEHOLDER;
window.__fixtureOutcome=FIXTURE_OUTCOME_PLACEHOLDER;
window.__fixtureRefusal=FIXTURE_REFUSAL_PLACEHOLDER;
window.__fixtureHydrateDelay=FIXTURE_HYDRATE_DELAY_PLACEHOLDER;
if(window.__fixtureRefusal){const banner=document.querySelector('#refusal');banner.textContent=window.__fixtureRefusal;banner.hidden=false;}
window.renderFixtureScope=()=>{
  document.querySelector('[data-batch-scope-actor]').textContent=window.__fixtureScope.actorId;
  document.querySelector('[data-batch-scope-organization]').textContent=window.__fixtureScope.organizationId;
};
window.hideFixtureScope=()=>{
  document.querySelector('[data-batch-scope-actor]').removeAttribute('data-batch-scope-actor');
  document.querySelector('[data-batch-scope-organization]').removeAttribute('data-batch-scope-organization');
};
window.renderFixtureScope();
// The real page renders its scope and enables its control asynchronously, after its own
// context reads resolve. A delay reproduces the window in which the tab's URL already
// carries the handoff key while the contract is still absent.
if(window.__fixtureHydrateDelay>0){setTimeout(()=>{window.renderFixtureScope();},window.__fixtureHydrateDelay);}
document.querySelector('#confirm').addEventListener('click',()=>{
  window.__fixtureClicks+=1;
  const refusal=document.querySelector('#refusal');
  if(window.__fixtureRefusal){refusal.textContent=window.__fixtureRefusal;refusal.hidden=false;document.querySelector('#confirm').disabled=true;return;}
  const node=document.querySelector('#result');
  node.setAttribute('data-batch-submit-result',window.__fixtureOutcome.status);
  if(window.__fixtureOutcome.operationId){node.setAttribute('data-batch-operation-id',window.__fixtureOutcome.operationId);}
  node.textContent=window.__fixtureOutcome.status;
});
async function readCapture(){
  if(window.__fixtureHydrateDelay>0){await new Promise(resolve=>setTimeout(resolve,window.__fixtureHydrateDelay));}
  if(!extensionId){document.querySelector('#status').textContent='恢复模式：只有原key，无自动POST';window.fixtureStatus='RECOVERY_ONLY';return;}
  let response;
  for(let attempt=0;attempt<10;attempt++){
    response=await chrome.runtime.sendMessage(extensionId,{version:1,type:'capture.read',handoffId,idempotencyKey});
    if(response?.type==='capture.payload')break;
    await new Promise(resolve=>setTimeout(resolve,250));
  }
  if(response?.type!=='capture.payload'){document.querySelector('#status').textContent='CAPTURE_UNAVAILABLE';window.fixtureStatus='CAPTURE_UNAVAILABLE';return;}
  window.fixtureReceived=response;
  history.replaceState(null,'','#operationKey='+idempotencyKey);
  document.querySelector('#summary').textContent=JSON.stringify(response.payload,null,2);
  document.querySelector('#status').textContent='FIXTURE_CAPTURE_RECEIVED';window.fixtureStatus='FIXTURE_CAPTURE_RECEIVED';
  document.querySelector('#confirm').disabled=false;
}
readCapture().catch(e=>{document.querySelector('#status').textContent='CAPTURE_UNAVAILABLE';window.fixtureStatus='THREW:'+String(e);});
</script></body></html>`

// requireFixtureEnv returns the browser binary and fixture extension directory, or
// skips the test. The manifest name is checked so a production build can never be
// used by accident, because a production build's application URL is a real origin.
func requireFixtureEnv(t *testing.T) (string, string) {
	t.Helper()
	browser := os.Getenv("BATCHCAPTURE_BROWSER")
	extDir := os.Getenv("BATCHCAPTURE_EXTENSION_DIST")
	if browser == "" || extDir == "" {
		t.Skip("set BATCHCAPTURE_BROWSER and BATCHCAPTURE_EXTENSION_DIST to run the fixture browser tests")
	}
	manifest, err := os.ReadFile(filepath.Join(extDir, "manifest.json"))
	if err != nil {
		t.Fatalf("read fixture manifest: %v", err)
	}
	if !strings.Contains(string(manifest), "Fixture") {
		t.Fatalf("refusing to drive a non-fixture extension build at %s", extDir)
	}
	// The application URL is compiled into the service worker, so that is where
	// the fixture target has to be confirmed before anything is driven.
	worker, err := os.ReadFile(filepath.Join(extDir, "background.js"))
	if err != nil {
		t.Fatalf("read fixture service worker: %v", err)
	}
	if !strings.Contains(string(worker), fixtureAppURL) {
		t.Fatalf("fixture build does not target %s; rebuild with CAPTURE_APP_URL=%s", fixtureAppURL, fixtureAppURL)
	}
	// Measured constraint: Chrome grants activeTab when a person invokes the action,
	// and Chromium 144 has no Extensions.triggerAction, so a programmatic
	// chrome.action.openPopup() never grants it and chrome.scripting.executeScript
	// fails with "manifest must request permission to access the respective host".
	// The extension therefore declares https://detail.1688.com/* as a host grant, and
	// this suite runs against the ordinary --fixture build; no patched copy is needed.
	// The check stays fatal rather than skipped: with the environment set, a build that
	// cannot exercise capture would report success while testing nothing.
	var parsed struct {
		HostPermissions []string `json:"host_permissions"`
	}
	if err := json.Unmarshal(manifest, &parsed); err != nil {
		t.Fatalf("parse fixture manifest: %v", err)
	}
	if !slices.Contains(parsed.HostPermissions, "https://detail.1688.com/*") {
		// Fatal, not Skip: the operator has already opted in by setting the
		// environment, so a build that cannot exercise capture means the suite would
		// report success while testing nothing.
		t.Fatalf("fixture build must declare host_permissions https://detail.1688.com/* "+
			"(activeTab cannot be granted without a real action click, which Chromium 144 cannot synthesize); "+
			"current host_permissions=%v; rebuild from extensions/1688-capture or do not set BATCHCAPTURE_EXTENSION_DIST",
			parsed.HostPermissions)
	}
	return browser, extDir
}

// startFixtureApp serves the transport-only receiver on the port the fixture
// extension build targets.
// startFixtureApp serves the transport-only receiver with the given scope, so a
// test can make the page present a scope the user did not approve.
func startFixtureApp(t *testing.T, scope AppScope) {
	t.Helper()
	serveFixtureApp(t, scope, "")
}

// startFixtureAppRefusing serves a receiver that shows a refusal banner while its
// confirmation control remains enabled, which is the case guard 3 exists for: the
// control's own state is not evidence that the page will accept the submission.
func startFixtureAppRefusing(t *testing.T, scope AppScope, refusal string) {
	t.Helper()
	serveFixtureApp(t, scope, refusal)
}

func serveFixtureApp(t *testing.T, scope AppScope, refusal string) {
	t.Helper()
	serveFixtureAppWithOutcome(t, scope, refusal, SubmitOutcome{Status: "published", OperationID: fixtureOperationID})
}

// startFixtureAppWithOutcome serves a receiver whose terminal result is chosen by
// the test, so the driver's handling of a status it does not recognise can be
// exercised rather than assumed.
func startFixtureAppWithOutcome(t *testing.T, scope AppScope, outcome SubmitOutcome) {
	t.Helper()
	serveFixtureAppWithOutcome(t, scope, "", outcome)
}

func serveFixtureAppWithOutcome(t *testing.T, scope AppScope, refusal string, outcome SubmitOutcome) {
	t.Helper()
	serveFixtureAppHydrating(t, scope, refusal, outcome, 0)
}

// startFixtureAppHydratingLate serves a receiver whose scope and confirmation control only
// render after delay. The handoff URL carries the key from the first paint, so this is the
// real single-page-application race the driver has to wait out.
func startFixtureAppHydratingLate(t *testing.T, scope AppScope, delay time.Duration) {
	t.Helper()
	serveFixtureAppHydrating(t, scope, "", SubmitOutcome{Status: "published", OperationID: fixtureOperationID}, int(delay.Milliseconds()))
}

func serveFixtureAppHydrating(t *testing.T, scope AppScope, refusal string, outcome SubmitOutcome, hydrateDelayMs int) {
	t.Helper()
	scopeJSON, err := json.Marshal(map[string]string{
		"actorId":        scope.ActorID,
		"organizationId": scope.OrganizationID,
	})
	if err != nil {
		t.Fatalf("encode fixture scope: %v", err)
	}
	refusalJSON, err := json.Marshal(refusal)
	if err != nil {
		t.Fatalf("encode fixture refusal: %v", err)
	}
	outcomeJSON, err := json.Marshal(map[string]string{
		"status":      outcome.Status,
		"operationId": outcome.OperationID,
	})
	if err != nil {
		t.Fatalf("encode fixture outcome: %v", err)
	}
	body := strings.Replace(fixtureReceiver, "FIXTURE_SCOPE_PLACEHOLDER", string(scopeJSON), 1)
	body = strings.Replace(body, "FIXTURE_REFUSAL_PLACEHOLDER", string(refusalJSON), 1)
	body = strings.Replace(body, "FIXTURE_OUTCOME_PLACEHOLDER", string(outcomeJSON), 1)
	body = strings.Replace(body, "FIXTURE_HYDRATE_DELAY_PLACEHOLDER", strconv.Itoa(hydrateDelayMs), 1)
	listener, err := net.Listen("tcp", "127.0.0.1:"+fixturePort)
	if err != nil {
		t.Fatalf("fixture port %s is unavailable: %v", fixturePort, err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/capture/1688" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Referrer-Policy", "no-referrer")
		_, _ = w.Write([]byte(body))
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	})
}

// routeFixtureProduct substitutes the public product response for the fixture
// offer so the extension extracts a known document from a real 1688 URL.
func routeFixtureProduct(t *testing.T, driver *Driver) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "batch-product.html"))
	if err != nil {
		t.Fatalf("read product fixture: %v", err)
	}
	err = driver.Context().Route("**/*", func(route playwright.Route) {
		request := route.Request()
		if strings.HasPrefix(request.URL(), "https://detail.1688.com/offer/") {
			_ = route.Fulfill(playwright.RouteFulfillOptions{
				ContentType: playwright.String("text/html; charset=utf-8"),
				Body:        playwright.String(string(body)),
			})
			return
		}
		_ = route.Continue()
	})
	if err != nil {
		t.Fatalf("route product page: %v", err)
	}
}

func launchFixtureDriver(t *testing.T, browser, extDir string) *Driver {
	t.Helper()
	return launchFixtureDriverWithTimeout(t, browser, extDir, 30*time.Second)
}

// launchFixtureDriverWithTimeout exists for the tests that assert the driver keeps
// waiting: they pay the control timeout on purpose, so they get to bound it.
func launchFixtureDriverWithTimeout(t *testing.T, browser, extDir string, timeout time.Duration) *Driver {
	t.Helper()
	driver, err := LaunchDriver(DriverOptions{
		ExecutablePath: browser,
		ProfileDir:     t.TempDir(),
		ExtensionDist:  extDir,
		Headless:       true,
		ControlTimeout: timeout,
	})
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	t.Cleanup(func() { _ = driver.Close() })
	return driver
}

// fixtureAppState reads the receiver's recorded state from the application tab.
func fixtureAppState(t *testing.T, driver *Driver) (status string, offerID string, missing int) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		for _, page := range driver.Context().Pages() {
			if !strings.HasPrefix(page.URL(), fixtureAppPrefix) {
				continue
			}
			raw, err := page.Evaluate(`(() => {
  const r = window.fixtureReceived;
  return {
    status: window.fixtureStatus || '',
    offerID: r && r.payload && r.payload.evidence ? r.payload.evidence.offerID : '',
    missing: r && r.payload && r.payload.evidence ? (r.payload.evidence.missingFacts || []).length : -1,
  };
})()`)
			if err != nil {
				continue
			}
			m, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			status, _ = m["status"].(string)
			offerID, _ = m["offerID"].(string)
			switch v := m["missing"].(type) {
			case float64:
				missing = int(v)
			case int:
				missing = v
			}
			if status != "" && status != "INIT" {
				return status, offerID, missing
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", "", -1
}

// TestFixtureDriverCapturesAndHandsOffItem proves the S1 browser mechanism end to
// end on a substituted page: the real action popup opens, its real capture
// control produces a capture the extension accepts, and its real handoff control
// opens an application tab whose fragment carries the generated idempotency key.
func TestFixtureDriverCapturesAndHandsOffItem(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)

	driver.OpenSourcePage(fixtureSource)
	if got := driver.Observe(true).FinalURL; got != fixtureSource {
		t.Fatalf("source page not observed: %q", got)
	}
	if verdict := driver.ClassifyCurrentPage(true); verdict != VerdictProceed {
		t.Fatalf("product page classified as %q, want proceed", verdict)
	}

	popup, err := driver.OpenPopup()
	if err != nil {
		t.Fatalf("open action popup: %v", err)
	}
	defer func() { _ = popup.Close() }()
	captured, err := popup.Capture()
	if err != nil {
		t.Fatalf("capture: %v", err)
	}
	if !captured.Captured() {
		t.Fatalf("capture did not produce a handoffable payload: %+v", captured)
	}

	handoffURL, err := popup.Handoff()
	if err != nil {
		t.Fatalf("handoff: %v", err)
	}
	ref, err := captureRefFromURL(handoffURL)
	if err != nil {
		t.Fatalf("handoff url %q: %v", handoffURL, err)
	}
	if ref.IdempotencyKey == "" {
		t.Fatalf("handoff url %q carried no idempotency key", handoffURL)
	}
	// The application normalises the fragment, so the extension id is present only
	// when the tab was observed before the normalisation.
	if ref.ExtensionID != "" && ref.ExtensionID != driver.ExtensionID() {
		t.Fatalf("handoff extension id %q, want %q", ref.ExtensionID, driver.ExtensionID())
	}

	status, offerID, missing := fixtureAppState(t, driver)
	if status != "FIXTURE_CAPTURE_RECEIVED" {
		t.Fatalf("application context did not receive the capture: status=%q", status)
	}
	if offerID != fixtureOfferID {
		t.Fatalf("application received offer %q, want %q", offerID, fixtureOfferID)
	}
	if missing < 0 {
		t.Fatalf("application received no missing-facts list")
	}
}

// TestFixtureDriverResetIsolatesItems proves design section 4 D1.1 against the
// measured popup lifetime: the handoff opens the application tab, which closes the
// action popup, while the background controller keeps holding the payload it
// already captured. The next item must therefore reopen the popup and clear that
// capture, and it must carry a different idempotency key. Reusing the key would
// make item two read back as item one's operation.
func TestFixtureDriverResetIsolatesItems(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)

	// Item one.
	firstPopup, err := driver.PrepareItem(fixtureSource)
	if err != nil {
		t.Fatalf("prepare item one: %v", err)
	}
	defer func() { _ = firstPopup.Close() }()
	first, err := firstPopup.Capture()
	if err != nil || !first.Captured() {
		t.Fatalf("first capture: state=%+v err=%v", first, err)
	}
	firstURL, err := firstPopup.Handoff()
	if err != nil {
		t.Fatalf("first handoff: %v", err)
	}
	firstRef, err := captureRefFromURL(firstURL)
	if err != nil {
		t.Fatalf("first handoff url: %v", err)
	}

	// The handoff opened a tab, so the popup that was attached for item one is
	// gone and the next item has to open its own.
	secondPopup, err := driver.PrepareItem(fixtureSource)
	if err != nil {
		t.Fatalf("prepare item two: %v", err)
	}
	defer func() { _ = secondPopup.Close() }()
	afterReset, err := secondPopup.State()
	if err != nil {
		t.Fatalf("item two popup state: %v", err)
	}
	if afterReset.Captured() || afterReset.FreshShown {
		t.Fatalf("item two popup still holds item one's capture: %+v", afterReset)
	}

	second, err := secondPopup.Capture()
	if err != nil || !second.Captured() {
		t.Fatalf("second capture: state=%+v err=%v", second, err)
	}
	secondURL, err := secondPopup.Handoff()
	if err != nil {
		t.Fatalf("second handoff: %v", err)
	}
	secondRef, err := captureRefFromURL(secondURL)
	if err != nil {
		t.Fatalf("second handoff url: %v", err)
	}
	if firstRef.IdempotencyKey == secondRef.IdempotencyKey {
		t.Fatalf("second item reused the first item's idempotency key %q", firstRef.IdempotencyKey)
	}
}

// TestFixtureDriverPrepareItemClearsCarriedCapture is the failure the batch loop
// would otherwise produce: a capture left in the background controller by a
// previous item being captured and handed off again as the next item. The
// controller deliberately keeps its payload across popup openings
// (background.ts:25, controller.ts:18), so only an explicit reset clears it.
func TestFixtureDriverPrepareItemClearsCarriedCapture(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)

	firstPopup, err := driver.PrepareItem(fixtureSource)
	if err != nil {
		t.Fatalf("prepare first item: %v", err)
	}
	if _, err := firstPopup.Capture(); err != nil {
		t.Fatalf("first capture: %v", err)
	}
	if _, err := firstPopup.Handoff(); err != nil {
		t.Fatalf("first handoff: %v", err)
	}
	_ = firstPopup.Close()

	// Reopening the popup without PrepareItem must still show the carried capture,
	// which is exactly why PrepareItem performs the reset.
	carried, err := driver.OpenPopup()
	if err != nil {
		t.Fatalf("reopen popup: %v", err)
	}
	defer func() { _ = carried.Close() }()
	state, err := carried.State()
	if err != nil {
		t.Fatalf("state: %v", err)
	}
	if !state.FreshShown {
		t.Fatalf("controller did not carry the previous capture, so this test no longer proves the reset is needed: %+v", state)
	}
	if err := carried.Reset(); err != nil {
		t.Fatalf("reset: %v", err)
	}
	cleared, err := carried.State()
	if err != nil {
		t.Fatalf("state after reset: %v", err)
	}
	if cleared.Captured() || cleared.FreshShown {
		t.Fatalf("reset did not clear the carried capture: %+v", cleared)
	}
}

// TestFixtureDriverStopsOnChallenge proves design section 4 D1.3 at the driver
// level: a substituted risk-control response is classified as a batch pause
// rather than as a product failure, and it is never overridden by the fact that
// the executor asked for a capture.
func TestFixtureDriverStopsOnChallenge(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	driver := launchFixtureDriver(t, browser, extDir)

	challengeBody, err := os.ReadFile(filepath.Join("testdata", "challenge-risk-control.html"))
	if err != nil {
		t.Fatalf("read challenge fixture: %v", err)
	}
	err = driver.Context().Route("**/*", func(route playwright.Route) {
		if strings.HasPrefix(route.Request().URL(), "https://detail.1688.com/offer/") {
			_ = route.Fulfill(playwright.RouteFulfillOptions{
				ContentType: playwright.String("text/html; charset=utf-8"),
				Body:        playwright.String(string(challengeBody)),
			})
			return
		}
		_ = route.Continue()
	})
	if err != nil {
		t.Fatalf("route challenge page: %v", err)
	}

	driver.OpenSourcePage(fixtureSource)
	verdict := driver.ClassifyCurrentPage(true)
	if verdict != VerdictPauseBatch {
		t.Fatalf("challenge page classified as %q, want pause_batch", verdict)
	}
	action := verdict.Action()
	if !action.StopBatch || !action.RedoCurrentItemAfterHuman || action.ContinueToNextItem {
		t.Fatalf("pause_batch action lets the batch continue: %+v", action)
	}
}

// TestFixtureDriverReportsUnavailableMechanisms keeps the failure mode honest: a
// missing extension directory must be reported as an unavailable driver rather
// than silently driving nothing.
func TestFixtureDriverReportsUnavailableMechanisms(t *testing.T) {
	if _, err := LaunchDriver(DriverOptions{}); !errors.Is(err, ErrDriverUnavailable) {
		t.Fatalf("empty options returned %v, want ErrDriverUnavailable", err)
	}
	dir := t.TempDir()
	_, err := LaunchDriver(DriverOptions{
		ExecutablePath: filepath.Join(dir, "missing-browser.exe"),
		ProfileDir:     dir,
		ExtensionDist:  dir,
	})
	if !errors.Is(err, ErrDriverUnavailable) {
		t.Fatalf("missing browser returned %v, want ErrDriverUnavailable", err)
	}
	if err := fmt.Errorf("%w: probe", ErrDriverUnavailable); !errors.Is(err, ErrDriverUnavailable) {
		t.Fatal("wrapped driver error lost its identity")
	}
}

// fixtureAppPageByKey finds the application tab for one handoff key. The driver
// resolves tabs this way in production, so the tests inspect the same tab the
// executor acted on rather than whichever tab happens to share the URL prefix.
func fixtureAppPageByKey(t *testing.T, driver *Driver, key string) playwright.Page {
	t.Helper()
	if key == "" {
		t.Fatal("no handoff key to look the application tab up by")
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, page := range driver.Context().Pages() {
			ref, err := captureRefFromURL(page.URL())
			if err == nil && ref.IdempotencyKey == key {
				return page
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("no application tab carries handoff key %s", key)
	return nil
}

// fixtureAppPage drives one item far enough that the application page is showing a
// captured payload and an enabled confirmation control.
func fixtureAppPage(t *testing.T) (*Driver, playwright.Page, string) {
	t.Helper()
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	popup, err := driver.PrepareItem(fixtureSource)
	if err != nil {
		t.Fatalf("prepare item: %v", err)
	}
	t.Cleanup(func() { _ = popup.Close() })
	if _, err := popup.Capture(); err != nil {
		t.Fatalf("capture: %v", err)
	}
	appURL, err := popup.Handoff()
	if err != nil {
		t.Fatalf("handoff: %v", err)
	}
	ref, err := captureRefFromURL(appURL)
	if err != nil {
		t.Fatalf("read the handoff key: %v", err)
	}
	page := fixtureAppPageByKey(t, driver, ref.IdempotencyKey)
	if _, err := page.WaitForSelector(selConfirmAndSubmit, playwright.PageWaitForSelectorOptions{
		State: playwright.WaitForSelectorStateAttached,
	}); err != nil {
		t.Fatalf("confirmation control never appeared: %v", err)
	}
	// The control exists from first paint and is only enabled once the payload has
	// arrived, so waiting for it to be enabled is what makes "the click was allowed"
	// true rather than a matter of timing.
	if _, err := page.WaitForFunction(`() => {
		const node = document.querySelector('`+selConfirmAndSubmit+`');
		return node !== null && !node.disabled;
	}`, nil); err != nil {
		t.Fatalf("confirmation control never became enabled: %v", err)
	}
	return driver, page, appURL
}

// fixtureClicks reports how many times the confirmation control was really
// clicked, which is how a guard is proven to have prevented the submission rather
// than merely reported an error after it.
func fixtureClicks(t *testing.T, page playwright.Page) int {
	t.Helper()
	raw, err := page.Evaluate("window.__fixtureClicks")
	if err != nil {
		t.Fatalf("read click counter: %v", err)
	}
	switch value := raw.(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		t.Fatalf("click counter is %T, want a number", raw)
		return 0
	}
}

// TestFixtureDriverConfirmsSubmitWithApprovedScope is design section 17.1's
// "drive Confirm and submit and read the terminal state" step.
func TestFixtureDriverConfirmsSubmitWithApprovedScope(t *testing.T) {
	driver, page, _ := fixtureAppPage(t)
	scope, err := driver.ReadAppScope(page)
	if err != nil {
		t.Fatalf("read scope: %v", err)
	}
	if !scope.IsComplete() {
		t.Fatalf("scope incomplete: %+v", scope)
	}
	outcome, err := driver.ConfirmAndSubmit(page, scope)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if outcome.Status != "published" {
		t.Fatalf("outcome status %q, want published", outcome.Status)
	}
	if outcome.OperationID != fixtureOperationID {
		t.Fatalf("outcome operation id %q, want %q", outcome.OperationID, fixtureOperationID)
	}
	if clicks := fixtureClicks(t, page); clicks != 1 {
		t.Fatalf("confirmation control clicked %d times, want exactly 1", clicks)
	}
}

// TestFixtureDriverRefusesScopeThatIsNotTheApprovedScope proves guard 1 does
// prevent the click: the page must not be submitted under an organization the
// executor was not approved for, and the refusal must happen before the click.
func TestFixtureDriverRefusesScopeThatIsNotTheApprovedScope(t *testing.T) {
	driver, page, _ := fixtureAppPage(t)
	approved := AppScope{ActorID: "fixture-actor-a", OrganizationID: "fixture-org-approved"}
	if _, err := driver.ConfirmAndSubmit(page, approved); !errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("err=%v, want ErrScopeMismatch", err)
	}
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("a mismatched scope still clicked the control %d times", clicks)
	}
}

// TestFixtureDriverTreatsPageRefusalAsFinal proves guard 3: once the page refuses,
// nothing is clicked and no retry is attempted.
func TestFixtureDriverTreatsPageRefusalAsFinal(t *testing.T) {
	driver, page, _ := fixtureAppPage(t)
	scope, err := driver.ReadAppScope(page)
	if err != nil {
		t.Fatalf("read scope: %v", err)
	}
	if _, err := page.Evaluate(`(() => {
		window.__fixtureRefusal = '当前用户与企业未确认，已拒绝本次操作。';
		const node = document.querySelector('#refusal');
		node.textContent = window.__fixtureRefusal;
		node.hidden = false;
	})()`); err != nil {
		t.Fatalf("stage refusal: %v", err)
	}
	if _, err := driver.ConfirmAndSubmit(page, scope); !errors.Is(err, ErrSubmitRefused) {
		t.Fatalf("err=%v, want ErrSubmitRefused", err)
	}
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("a refused page was still clicked %d times", clicks)
	}
	// A second attempt must not sneak a click in either.
	if _, err := driver.ConfirmAndSubmit(page, scope); !errors.Is(err, ErrSubmitRefused) {
		t.Fatalf("second attempt err=%v, want ErrSubmitRefused", err)
	}
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("a retry clicked a refused page %d times", clicks)
	}
}

// TestFixtureDriverFailsClosedWithoutReadableScope proves an unreadable scope stops
// the item instead of submitting under whatever the session happens to be.
func TestFixtureDriverFailsClosedWithoutReadableScope(t *testing.T) {
	driver, page, _ := fixtureAppPage(t)
	if _, err := page.Evaluate("window.hideFixtureScope()"); err != nil {
		t.Fatalf("hide scope: %v", err)
	}
	if _, err := driver.ReadAppScope(page); !errors.Is(err, ErrScopeUnavailable) {
		t.Fatalf("err=%v, want ErrScopeUnavailable", err)
	}
	if _, err := driver.ConfirmAndSubmit(page, AppScope{ActorID: "fixture-actor-a", OrganizationID: "fixture-org-a"}); !errors.Is(err, ErrScopeUnavailable) {
		t.Fatalf("confirm err=%v, want ErrScopeUnavailable", err)
	}
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("an unreadable scope still clicked the control %d times", clicks)
	}
}

// TestFixtureDriverRefusesDisabledControl proves guard 2 addresses the real
// control's own state rather than clicking whatever element is present.
func TestFixtureDriverRefusesDisabledControl(t *testing.T) {
	driver, page, _ := fixtureAppPage(t)
	scope, err := driver.ReadAppScope(page)
	if err != nil {
		t.Fatalf("read scope: %v", err)
	}
	if _, err := page.Evaluate(`document.querySelector('` + selConfirmAndSubmit + `').disabled = true`); err != nil {
		t.Fatalf("disable control: %v", err)
	}
	if _, err := driver.ConfirmAndSubmit(page, scope); !errors.Is(err, ErrSubmitUnavailable) {
		t.Fatalf("err=%v, want ErrSubmitUnavailable", err)
	} else if !strings.Contains(err.Error(), "control_disabled") {
		// Clicking a disabled control dispatches no event, so the submission stays
		// safe either way; what this asserts is that the operator is told the real
		// reason instead of being handed an unreadable-result timeout.
		t.Fatalf("a disabled control was not reported as such: %v", err)
	}
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("a disabled control was clicked %d times", clicks)
	}
}

// TestFixtureDriverRecordsNonTerminalResultAsNotTerminal proves the driver never
// invents a terminal state: a page that reports nothing definitive must surface as
// an error so the queue records outcome_unknown (design section 15.10).
func TestFixtureDriverRecordsNonTerminalResultAsNotTerminal(t *testing.T) {
	driver, page, _ := fixtureAppPage(t)
	scope, err := driver.ReadAppScope(page)
	if err != nil {
		t.Fatalf("read scope: %v", err)
	}
	if _, err := page.Evaluate(`window.__fixtureOutcome = {status:'', operationId:''}`); err != nil {
		t.Fatalf("stage non-terminal outcome: %v", err)
	}
	driver.timeout = 2 * time.Second
	if _, err := driver.ConfirmAndSubmit(page, scope); !errors.Is(err, ErrSubmitUnavailable) {
		t.Fatalf("err=%v, want ErrSubmitUnavailable", err)
	}
	if clicks := fixtureClicks(t, page); clicks != 1 {
		t.Fatalf("click counter %d, want 1 (the click happened, only the result was unreadable)", clicks)
	}
}

// fixtureApprovedScope is the scope the fixture receiver presents, so the approved
// scope and the page agree unless a test deliberately changes one of them.
var fixtureApprovedScope = AppScope{ActorID: "fixture-actor-a", OrganizationID: "fixture-org-a"}

// fixtureQueue writes a one-item queue whose scope a person has already approved,
// which is the state design section 4 D1.4 requires before any item may run.
func fixtureQueue(t *testing.T, scope AppScope) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "queue.json")
	queue := NewQueue("fixture-batch")
	if err := queue.ApproveScope(scope.ActorID, scope.OrganizationID); err != nil {
		t.Fatalf("approve scope: %v", err)
	}
	queue.Items = append(queue.Items, Item{Seq: 1, URL: fixtureSource, State: ItemQueued})
	if err := queue.Save(path); err != nil {
		t.Fatalf("save queue: %v", err)
	}
	return path
}

// fixtureAppTabCount counts open tabs at the fixture application, which is how a
// test proves a handoff never happened without waiting for a timeout.
func fixtureAppTabCount(t *testing.T, driver *Driver) int {
	t.Helper()
	return fixtureAppTabCountWithin(t, driver, 0)
}

// fixtureAppTabCountWithin allows the application tab to appear slightly later than
// the instant the handoff click was dispatched. It exists only for the test that
// abandons the app tab's URL, so production code still has exactly one observation
// point (waitForAppTab) and is not given a second, competing wait.
func fixtureAppTabCountWithin(t *testing.T, driver *Driver, wait time.Duration) int {
	t.Helper()
	deadline := time.Now().Add(wait)
	for {
		count := 0
		for _, page := range driver.Context().Pages() {
			if strings.HasPrefix(page.URL(), fixtureAppPrefix) {
				count++
			}
		}
		if count > 0 || !time.Now().Before(deadline) {
			return count
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestFixtureDriverImportsOneItemEndToEnd is design section 14 S1's acceptance:
// one item is driven from the queue to a read-back terminal result.
func TestFixtureDriverImportsOneItemEndToEnd(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.State != ItemSubmitted {
		t.Fatalf("result state %q, want submitted", result.State)
	}
	if result.IdempotencyKey == "" {
		t.Fatal("no idempotency key was recorded")
	}
	if result.OperationID != fixtureOperationID {
		t.Fatalf("operation id %q, want %q", result.OperationID, fixtureOperationID)
	}

	// The durable record must agree with the returned result, not just the variable
	// the driver happened to keep in memory.
	stored, err := Load(queuePath)
	if err != nil {
		t.Fatalf("reload queue: %v", err)
	}
	if stored.Items[0].State != ItemSubmitted {
		t.Fatalf("stored state %q, want submitted", stored.Items[0].State)
	}
	if stored.Items[0].IdempotencyKey != result.IdempotencyKey {
		t.Fatalf("stored key %q, want %q", stored.Items[0].IdempotencyKey, result.IdempotencyKey)
	}
	if stored.Items[0].OperationID != result.OperationID {
		t.Fatalf("stored operation id %q", stored.Items[0].OperationID)
	}

	status, offerID, _ := fixtureAppState(t, driver)
	if status != "FIXTURE_CAPTURE_RECEIVED" {
		t.Fatalf("application did not receive the payload: status=%q", status)
	}
	if offerID != fixtureOfferID {
		t.Fatalf("application received offer %q, want %q", offerID, fixtureOfferID)
	}
	page := fixtureAppPageByKey(t, driver, result.IdempotencyKey)
	if clicks := fixtureClicks(t, page); clicks != 1 {
		t.Fatalf("confirmation clicked %d times, want exactly 1", clicks)
	}
}

// TestFixtureDriverDoesNotHandoffWhenIntentWriteFails is the acceptance item for
// design section 4 D2.2: a write that is not confirmed durable must stop the item
// before anything can be submitted.
func TestFixtureDriverDoesNotHandoffWhenIntentWriteFails(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	calls := 0
	failing := func(queue *Queue, path string) error {
		calls++
		return errors.New("injected write failure")
	}
	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope, Persist: failing})
	if !errors.Is(err, ErrQueueWriteFailed) {
		t.Fatalf("err=%v, want ErrQueueWriteFailed", err)
	}
	if calls != 1 {
		t.Fatalf("persist called %d times, want 1 (the item must stop at the first unconfirmed write)", calls)
	}
	// The failure happened before the dispatch, so nothing reached the application and
	// the file still holds a re-capturable state. Reporting it as an unknown outcome (or
	// as a record the file may not hold) would send the operator to verify an application
	// that never received anything and would burn the one signal that means "do not
	// re-run", so the crash-safety classification must not leak in here.
	if errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("a pre-handoff write failure was reported as an unknown outcome: %v", err)
	}
	if errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("a pre-handoff write failure was reported as a possibly-submitting record: %v", err)
	}
	if !CanRecapture(result.State) {
		t.Fatalf("result state %q is not re-capturable although nothing was handed off", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if !CanRecapture(stored.Items[0].State) {
		t.Fatalf("the failed intent write left the item un-recapturable: %+v", stored.Items[0])
	}
	if _, blocked := stored.BlockingItem(); blocked {
		t.Fatalf("the failed intent write blocked the batch although nothing was handed off")
	}
	// Nothing may have reached the application.
	if count := fixtureAppTabCount(t, driver); count != 0 {
		t.Fatalf("%d application tabs were opened despite an unconfirmed write", count)
	}
	if status := fixtureStatus(t, driver); status != "INIT" {
		t.Fatalf("the extension state advanced to %q without a confirmed write", status)
	}
}

// TestFixtureDriverStopsBeforeTheHandoffWhenTheIntentWriteCouldNotBeRestored covers the
// one phase-one failure that is not an ordinary re-runnable one. The intent write is the
// record that makes the item submittable, and a save can fail after its atomic replace
// has already become visible (the directory flush that makes the replacement durable
// happens last). queue.Save writes the previous content back in that case, but when it
// cannot, the file may hold submitting - the one state that forbids an automatic redo.
//
// So the item must still not be handed off, and the run must NOT tell the operator the
// item can simply be re-done: BlockingItem would stop the next run on that same record
// and the operator would have no way to reconcile the advice with the file.
func TestFixtureDriverStopsBeforeTheHandoffWhenTheIntentWriteCouldNotBeRestored(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	calls := 0
	// A real save whose every directory flush fails: the replacement lands, nothing can
	// be confirmed durable, and the previous content cannot be written back either.
	failingFlush := func(queue *Queue, path string) error {
		calls++
		return queue.save(path, func(string) error { return os.ErrPermission })
	}
	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope, Persist: failingFlush})
	if !errors.Is(err, ErrQueueWriteFailed) {
		t.Fatalf("err=%v, want ErrQueueWriteFailed", err)
	}
	if calls != 1 {
		t.Fatalf("persist called %d times, want 1 (the item must stop at the first unconfirmed write)", calls)
	}
	// The file may say submitting, so the state must be reported as unconfirmed rather
	// than as a re-capturable one, and it must not claim an unknown outcome: the
	// application provably never saw a payload.
	if !errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("err=%v, want ErrItemStateUnconfirmed", err)
	}
	if errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("a pre-handoff unrestored write was reported as an unknown outcome: %v", err)
	}
	if errors.Is(err, ErrVerdictStop) {
		t.Fatalf("the redo affordance leaked into a write the file cannot confirm: %v", err)
	}
	if NeedsVisibleSession(result, err) {
		t.Fatalf("the item was offered for a redo although the file may record it as submitting")
	}
	if result.State != ItemSubmitting {
		t.Fatalf("state=%s, want %s", result.State, ItemSubmitting)
	}
	if strings.Contains(err.Error(), "the item can be re-done") {
		t.Fatalf("the message promises a redo the file may not allow: %v", err)
	}
	// The whole point: nothing reached the application, and the extension was never asked
	// to capture or hand off.
	if count := fixtureAppTabCount(t, driver); count != 0 {
		t.Fatalf("%d application tabs were opened despite an unrestored write", count)
	}
	if status := fixtureStatus(t, driver); status != "INIT" {
		t.Fatalf("the extension state advanced to %q without a confirmed write", status)
	}
}

// TestFixtureDriverRecordsOutcomeUnknownWhenKeyWriteFails covers the window that
// design section 4 D2.1 cannot remove: the payload is already visible to the
// application when the key write is attempted, so a failure there must leave an
// outcome_unknown record rather than a re-doable one.
func TestFixtureDriverRecordsOutcomeUnknownWhenKeyWriteFails(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	calls := 0
	failingAfterFirst := func(queue *Queue, path string) error {
		calls++
		if calls == 1 {
			return queue.Save(path)
		}
		return errors.New("injected write failure")
	}
	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope, Persist: failingAfterFirst})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown", err)
	}
	// The message must not claim the file records the outcome this run reached: the
	// write is exactly what failed, so the durable record is still the submitting one.
	if !errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("the unconfirmed write is not distinguished from a recorded outcome: %v", err)
	}
	if !strings.Contains(err.Error(), `"submitting"`) {
		t.Fatalf("the operator is not told which state the file holds: %v", err)
	}
	// The returned result has to name the state the run stopped in. Reporting the zero
	// value here would print an empty state for an item that is in fact submitting, so
	// an operator reading the CLI output could not tell what to look for.
	if result.State != ItemSubmitting {
		t.Fatalf("result state %q, want submitting (the state whose durability was not confirmed)", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	// The key could not be written, so the purely local record still says "submitting".
	// That is the safe outcome, and the property to assert is that it is not
	// re-capturable: a second capture is how a duplicate publication happens.
	if CanRecapture(stored.Items[0].State) {
		t.Fatalf("a failed key write left the item re-capturable: %+v", stored.Items[0])
	}
	if _, blocked := stored.BlockingItem(); !blocked {
		t.Fatal("the queue does not report the item as blocking the batch")
	}
}

// fixtureStatus reads the fixture receiver's own status marker.
func fixtureStatus(t *testing.T, driver *Driver) string {
	t.Helper()
	for _, page := range driver.Context().Pages() {
		if !strings.HasPrefix(page.URL(), fixtureAppPrefix) {
			continue
		}
		raw, err := page.Evaluate("window.fixtureStatus")
		if err != nil {
			t.Fatalf("read fixture status: %v", err)
		}
		status, _ := raw.(string)
		return status
	}
	return "INIT"
}

// routeFixtureChallenge serves a measured-shape risk-control page instead of a
// product page, so the pre-handoff gate can be exercised without real 1688 traffic.
func routeFixtureChallenge(t *testing.T, driver *Driver) {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "challenge-risk-control.html"))
	if err != nil {
		t.Fatalf("read challenge fixture: %v", err)
	}
	err = driver.Context().Route("**/*", func(route playwright.Route) {
		if strings.HasPrefix(route.Request().URL(), "https://detail.1688.com/offer/") {
			_ = route.Fulfill(playwright.RouteFulfillOptions{
				ContentType: playwright.String("text/html; charset=utf-8"),
				Body:        playwright.String(string(body)),
			})
			return
		}
		_ = route.Continue()
	})
	if err != nil {
		t.Fatalf("route challenge page: %v", err)
	}
}

// TestFixtureDriverStopsBeforeHandoffOnChallenge is design section 4 D1.3: a
// challenge stops the batch, and because nothing was handed off the item stays
// re-doable so a person can clear the gate and the same item is then re-done.
func TestFixtureDriverStopsBeforeHandoffOnChallenge(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureChallenge(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if !errors.Is(err, ErrVerdictStop) {
		t.Fatalf("err=%v, want ErrVerdictStop", err)
	}
	if result.State != ItemQueued {
		t.Fatalf("result state %q, want queued to match the durable queue", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemQueued {
		t.Fatalf("state %q, want queued (nothing was submitted, so the item is re-doable)", stored.Items[0].State)
	}
	if !CanRecapture(stored.Items[0].State) {
		t.Fatalf("a challenge left the item un-recapturable: %+v", stored.Items[0])
	}
	if count := fixtureAppTabCount(t, driver); count != 0 {
		t.Fatalf("a challenge page still opened %d application tabs", count)
	}
}

// TestFixtureDriverDoesNotRecordSubmittingBeforeTheHandoff is the crash-recovery
// property behind design section 4 D2.1's placement of phase one. Navigation, page
// classification and extension capture cannot reach the application, so a run that
// stops there must never have written submitting: a process killed in that window would
// otherwise leave an item that provably never left the executor permanently blocked on
// human verification, with no CLI path to resume it.
//
// The kill itself cannot be produced by a test, but the durable writes can be recorded,
// and that is enough to show the record never advanced past a re-capturable state before
// the handoff.
func TestFixtureDriverDoesNotRecordSubmittingBeforeTheHandoff(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureChallenge(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	var written []ItemState
	recording := func(queue *Queue, path string) error {
		for _, item := range queue.Items {
			written = append(written, item.State)
		}
		return queue.Save(path)
	}
	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope, Persist: recording})
	if !errors.Is(err, ErrVerdictStop) {
		t.Fatalf("err=%v, want ErrVerdictStop", err)
	}
	for _, state := range written {
		if state == ItemSubmitting {
			t.Fatalf("a pre-handoff stop recorded submitting, so a crash there would block the item: %v", written)
		}
	}
	if result.State != ItemQueued {
		t.Fatalf("result state %q, want queued", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if !CanRecapture(stored.Items[0].State) {
		t.Fatalf("a pre-handoff stop left the item un-recapturable: %+v", stored.Items[0])
	}
	if _, blocked := stored.BlockingItem(); blocked {
		t.Fatalf("a pre-handoff stop blocked the batch: %+v", stored.Items[0])
	}
}

// TestFixtureDriverKeepsAPreHandoffItemRedoableWhenTheStopWriteFails pins the sibling
// of the move above. The write that records a pre-handoff stop is not the safety
// barrier - phase one is, and it has not run - so a failure there must not be reported
// as "the file may say submitting" (it does not) or as stop-and-verify. Nothing was
// handed off, the file still holds a state ImportOne will re-do from, and a gate stop
// must still be recoverable in the browser that is still open.
func TestFixtureDriverKeepsAPreHandoffItemRedoableWhenTheStopWriteFails(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureChallenge(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	failing := func(*Queue, string) error { return errors.New("injected write failure") }
	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope, Persist: failing})
	if !errors.Is(err, ErrQueueWriteFailed) {
		t.Fatalf("err=%v, want ErrQueueWriteFailed", err)
	}
	if errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("a failed pre-handoff revert was reported as a possibly-submitting record: %v", err)
	}
	if !errors.Is(err, ErrVerdictStop) {
		t.Fatalf("the gate cause was lost, so the operator cannot be offered the redo: %v", err)
	}
	if !NeedsVisibleSession(result, err) {
		t.Fatalf("a challenge whose stop write failed is no longer recoverable in the open browser: %v", err)
	}
	if result.State != ItemQueued {
		t.Fatalf("result state %q, want queued (the file never advanced past it)", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if !CanRecapture(stored.Items[0].State) {
		t.Fatalf("the item became un-recapturable after a failed pre-handoff revert: %+v", stored.Items[0])
	}
}

// TestFixtureDriverDoesNotSubmitUnderADifferentPageScope is the acceptance item for
// design section 4 D1.2 guard 1 on the post-handoff path: the application is showing
// a scope the user did not approve, so nothing may be submitted and, because the
// payload is already visible to the application, the item must be recorded as
// undecidable rather than re-doable.
func TestFixtureDriverDoesNotSubmitUnderADifferentPageScope(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, AppScope{ActorID: "fixture-actor-a", OrganizationID: "fixture-org-someone-else"})
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	_, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown", err)
	}
	if !strings.Contains(err.Error(), "approved scope") {
		t.Fatalf("the failure does not name the scope as the cause: %v", err)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemOutcomeUnknown {
		t.Fatalf("state %q, want outcome_unknown", stored.Items[0].State)
	}
	if CanRecapture(stored.Items[0].State) {
		t.Fatalf("a scope mismatch left the item re-capturable: %+v", stored.Items[0])
	}
	page := fixtureAppPageByKey(t, driver, stored.Items[0].IdempotencyKey)
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("the confirmation was clicked %d times under the wrong scope", clicks)
	}
}

// TestFixtureDriverRecordsOutcomeUnknownWhenPageRefuses is design section 4 D1.2
// guard 3: a page that refuses is final. Because the payload was already visible to
// the application, the item becomes undecidable instead of being retried.
func TestFixtureDriverRecordsOutcomeUnknownWhenPageRefuses(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureAppRefusing(t, fixtureApprovedScope, "当前用户与企业未确认，已拒绝本次操作。")
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	_, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown", err)
	}
	if !strings.Contains(err.Error(), "refused") {
		t.Fatalf("the failure does not name the refusal as the cause: %v", err)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemOutcomeUnknown {
		t.Fatalf("state %q, want outcome_unknown", stored.Items[0].State)
	}
	page := fixtureAppPageByKey(t, driver, stored.Items[0].IdempotencyKey)
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("a refused page was clicked %d times", clicks)
	}
}

// TestFixtureDriverTreatsUnrecognisedStatusAsUnknown proves the driver does not
// invent a terminal state: a status it does not understand must stop the item as
// outcome_unknown rather than be recorded as a successful publication.
func TestFixtureDriverTreatsUnrecognisedStatusAsUnknown(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureAppWithOutcome(t, fixtureApprovedScope, SubmitOutcome{Status: "something_new", OperationID: fixtureOtherID})
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown", err)
	}
	if !strings.Contains(err.Error(), "something_new") {
		t.Fatalf("the failure does not name the unrecognised status: %v", err)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemOutcomeUnknown {
		t.Fatalf("state %q, want outcome_unknown", stored.Items[0].State)
	}
	if result.State != ItemOutcomeUnknown {
		t.Fatalf("result state %q, want outcome_unknown", result.State)
	}
}

// TestFixtureDriverRefusesATerminalStatusWithoutAnOperationID is the seventh review
// round's second finding: the application's result contract requires `operationId` for
// every outcome (web/listingkit-ui/src/lib/contracts/product-acquisition.ts:7-9), so a
// render that names a status without one is a partial or drifted page. Accepting it
// would record a publication nobody can look up, or a terminal failure with an empty
// operation id for an item that did reach the application. The driver waits instead,
// and the item ends as outcome_unknown rather than as a success.
func TestFixtureDriverRefusesATerminalStatusWithoutAnOperationID(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	// `published` with no operation id is the exact render the fixture produces when the
	// outcome carries none.
	startFixtureAppWithOutcome(t, fixtureApprovedScope, SubmitOutcome{Status: "published", OperationID: ""})
	driver := launchFixtureDriverWithTimeout(t, browser, extDir, 5*time.Second)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown", err)
	}
	if result.State != ItemOutcomeUnknown {
		t.Fatalf("result state %q, want outcome_unknown", result.State)
	}
	if result.OperationID != "" {
		t.Fatalf("result operation id %q, want empty", result.OperationID)
	}
	// The click did happen; what must not happen is recording it as a publication.
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State == ItemSubmitted {
		t.Fatal("a status without an operation id was recorded as a successful submission")
	}
	if stored.Items[0].State != ItemOutcomeUnknown {
		t.Fatalf("stored state %q, want outcome_unknown", stored.Items[0].State)
	}
}

// TestFixtureDriverRefusesATerminalStatusWithAMalformedOperationID covers the other
// half of the same contract: a value that is present but cannot be an operation id is
// drift too, and UUID shape is the part of the contract that is observable here.
func TestFixtureDriverRefusesATerminalStatusWithAMalformedOperationID(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureAppWithOutcome(t, fixtureApprovedScope, SubmitOutcome{Status: "published", OperationID: "fixture-operation-1"})
	driver := launchFixtureDriverWithTimeout(t, browser, extDir, 5*time.Second)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown", err)
	}
	if result.State != ItemOutcomeUnknown {
		t.Fatalf("result state %q, want outcome_unknown", result.State)
	}
}

// TestFixtureDriverIgnoresAStaleApplicationTab is the regression test for the
// second defect found in review: the browser profile is reused and the previous
// item's application tab is still open when the next item runs. Resolving the page
// by URL prefix alone would drive the old tab and publish under an older key, so the
// tab must be resolved by this item's own handoff key.
func TestFixtureDriverIgnoresAStaleApplicationTab(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)

	// A leftover tab from an earlier run, same origin and path, different key.
	staleKey := "3f1d2c9e-8a4b-4d5e-9f60-1b2c3d4e5f60"
	if _, err := driver.Context().NewPage(); err != nil {
		t.Fatalf("open a page: %v", err)
	}
	stale, err := driver.Context().Pages()[0].Goto(fixtureAppURL + "#operationKey=" + staleKey)
	if err != nil {
		t.Fatalf("navigate the stale tab: %v", err)
	}
	_ = stale

	queuePath := fixtureQueue(t, fixtureApprovedScope)
	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.IdempotencyKey == "" || result.IdempotencyKey == staleKey {
		t.Fatalf("item ran under key %q, want a fresh key", result.IdempotencyKey)
	}
	if result.IdempotencyKey != result.OperationID && result.OperationID == "" {
		t.Fatalf("no operation was recorded: %+v", result)
	}

	// The stale tab must not have been touched, and the item's own tab must be the
	// one that was confirmed.
	if clicks := fixtureClicks(t, fixtureAppPageByKey(t, driver, staleKey)); clicks != 0 {
		t.Fatalf("the stale application tab was clicked %d times", clicks)
	}
	if clicks := fixtureClicks(t, fixtureAppPageByKey(t, driver, result.IdempotencyKey)); clicks != 1 {
		t.Fatalf("the item's own application tab was clicked %d times, want exactly 1", clicks)
	}
}

// TestFixtureDriverTreatsDelistedPageAsSingleItemFailure is the regression test for
// the third defect found in review: the pre-capture classification claimed product
// data was present, so a definitively delisted page could be recorded as capturable.
// The product-data judgment has to come from whether the capture actually produced
// the required fields.
func TestFixtureDriverTreatsDelistedPageAsSingleItemFailure(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	delisted, err := os.ReadFile(filepath.Join("testdata", "challenge-delisted.html"))
	if err != nil {
		t.Fatalf("read the delisted fixture: %v", err)
	}
	if err := driver.Context().Route("https://detail.1688.com/offer/**", func(route playwright.Route) {
		if err := route.Fulfill(playwright.RouteFulfillOptions{
			Status:      playwright.Int(200),
			ContentType: playwright.String("text/html; charset=utf-8"),
			Body:        string(delisted),
		}); err != nil {
			t.Errorf("fulfil delisted page: %v", err)
		}
	}); err != nil {
		t.Fatalf("route the delisted page: %v", err)
	}
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if !errors.Is(err, ErrVerdictStop) {
		t.Fatalf("err=%v, want ErrVerdictStop", err)
	}
	if errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("a delisted page has no ambiguous outcome: %v", err)
	}
	if result.Verdict != VerdictSingleItemFailure {
		t.Fatalf("verdict=%q, want %q", result.Verdict, VerdictSingleItemFailure)
	}
	// The returned result must agree with what was made durable. Before this was
	// fixed, the pre-handoff stops returned the zero value (an empty state) while the
	// queue held a real state, so the CLI printed a blank state during recovery.
	if result.State != ItemFailed {
		t.Fatalf("result state %q, want failed to match the durable queue", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemFailed {
		t.Fatalf("state %q, want failed", stored.Items[0].State)
	}
	if CanRecapture(stored.Items[0].State) || RequiresHumanReview(stored.Items[0].State) {
		t.Fatalf("a delisted item was left for retry or review: %+v", stored.Items[0])
	}
	if count := fixtureAppTabCount(t, driver); count != 0 {
		t.Fatalf("a delisted page still opened %d application tabs", count)
	}
}

// TestFixtureDriverKeepsUnknownOutcomeWhenTheFinalWriteFails is the regression test
// for the sixth defect found in review: when the crash-safe write of the terminal
// state fails as well, the run must still report an unknown outcome, because that is
// what forbids an automatic retry. Reporting only the write failure would read as an
// ordinary failure and invite a second submission.
//
// The seventh review round added the second half: the message must not claim the file
// records that outcome, because the write that would have recorded it is the write
// that failed. The file still holds the phase-one record, so the reported state is
// submitting and ErrItemStateUnconfirmed says why it is not the intended one.
func TestFixtureDriverKeepsUnknownOutcomeWhenTheFinalWriteFails(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	// A scope the user did not approve makes ConfirmAndSubmit refuse, which is the
	// path that records outcome_unknown.
	startFixtureApp(t, AppScope{ActorID: "fixture-actor-a", OrganizationID: "fixture-org-someone-else"})
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	// Phase one is the submit intent, phase two binds the handoff key; the third
	// write is the terminal state, and that is the one made to fail. The first two
	// must really land, or the file-level assertions below would be checking the
	// queue's initial state instead of the record the failed write failed to replace.
	calls := 0
	failFinal := func(queue *Queue, path string) error {
		calls++
		if calls <= 2 {
			return queue.Save(path)
		}
		return errors.New("disk is gone")
	}
	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope, Persist: failFinal})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown to survive the failed write", err)
	}
	if !errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("the unconfirmed write is not distinguished from a recorded outcome: %v", err)
	}
	if !errors.Is(err, ErrQueueWriteFailed) {
		t.Fatalf("the failed write is not reported: %v", err)
	}
	if result.State != ItemSubmitting {
		t.Fatalf("result state %q, want submitting: the file still holds the phase-one record", result.State)
	}
	if !strings.Contains(err.Error(), "approved scope") {
		t.Fatalf("the original cause was lost: %v", err)
	}
	// The durable file is the one that decides what may happen next, so assert on it
	// rather than on the in-memory queue the failing write never replaced.
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemSubmitting {
		t.Fatalf("stored state %q, want submitting: the phase-one record is what survived", stored.Items[0].State)
	}
	if CanRecapture(stored.Items[0].State) {
		t.Fatalf("a failed terminal write left the item re-capturable: %+v", stored.Items[0])
	}
	if _, blocked := stored.BlockingItem(); !blocked {
		t.Fatal("the queue does not report the item as blocking the batch")
	}
}

// TestFixtureDriverKeepsSubmittingWhenTheTerminalWriteFails covers the third raw
// write-failure site the seventh review round named: the write that would record the
// terminal result itself. The application has already accepted the payload, so the run
// must still end as outcome_unknown and forbid a retry — but the file, which is what
// decides what happens next, still holds the phase-one record. The message and the
// returned state have to say that instead of claiming a result that was never written.
func TestFixtureDriverKeepsSubmittingWhenTheTerminalWriteFails(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	// An approved queue writes the submit intent, then the handoff key, and only then
	// the terminal result, so the third write is the terminal one.
	calls := 0
	failTerminal := func(queue *Queue, path string) error {
		calls++
		if calls <= 2 {
			return queue.Save(path)
		}
		return errors.New("disk is gone")
	}
	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope, Persist: failTerminal})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown: the application already holds the payload", err)
	}
	if !errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("the unconfirmed write is not distinguished from a recorded outcome: %v", err)
	}
	if result.State != ItemSubmitting {
		t.Fatalf("result state %q, want submitting: the terminal record never landed", result.State)
	}
	// The result that was read back in memory is still reported; only its durability
	// is unknown. Dropping it would hide which operation the operator must look for.
	if result.OperationID != fixtureOperationID {
		t.Fatalf("result operation id %q, want %q", result.OperationID, fixtureOperationID)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemSubmitting {
		t.Fatalf("stored state %q, want submitting: the terminal record never landed", stored.Items[0].State)
	}
	if stored.Items[0].OperationID != "" {
		t.Fatalf("the file records an operation id that was never written: %+v", stored.Items[0])
	}
	if CanRecapture(stored.Items[0].State) {
		t.Fatalf("a failed terminal write left the item re-capturable: %+v", stored.Items[0])
	}
}

// TestFixtureDriverKeepsSubmittingWhenTheScopeWriteFails covers the same class for the
// confirmed scope. An unapproved queue obtains the person's confirmation only after the
// payload is visible to the application, so a failure writing that approval must not be
// reported as a recorded approval — the file still holds the phase-one record.
func TestFixtureDriverKeepsSubmittingWhenTheScopeWriteFails(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureUnapprovedQueue(t)

	calls := 0
	failApprove := func(queue *Queue, path string) error {
		calls++
		if calls <= 2 {
			return queue.Save(path)
		}
		return errors.New("disk is gone")
	}
	asked := false
	result, err := ImportOne(driver, ImportOptions{
		QueuePath: queuePath,
		Approved:  fixtureApprovedScope,
		Persist:   failApprove,
		ConfirmScope: func(AppScope) (bool, error) {
			asked = true
			return true, nil
		},
	})
	if !asked {
		t.Fatal("the run never asked for the scope confirmation")
	}
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown: the payload is already visible", err)
	}
	if !errors.Is(err, ErrItemStateUnconfirmed) {
		t.Fatalf("the unconfirmed write is not distinguished from a recorded outcome: %v", err)
	}
	if result.State != ItemSubmitting {
		t.Fatalf("result state %q, want submitting", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.ScopeApproved {
		t.Fatal("the queue records a scope approval that was never written")
	}
	if stored.Items[0].State != ItemSubmitting {
		t.Fatalf("stored state %q, want submitting", stored.Items[0].State)
	}
}

// The handoff click is the point of no return: it is the call that creates the
// application tab carrying the payload, so failing to *observe* that tab must not
// return the item to a capturable state. Re-capturing it would produce a second
// payload and a possible second publication (design section 4 D2.1).
//
// The observation failure is injected for the same reason as the write failure:
// the fixture extension always opens its tab, so "the handoff result could not be
// enumerated" cannot be produced through it.
func TestFixtureDriverKeepsOutcomeUnknownWhenTheHandoffResultCannotBeObserved(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	result, err := ImportOne(driver, ImportOptions{
		QueuePath: queuePath,
		Approved:  fixtureApprovedScope,
		AwaitHandoff: func(*Popup, map[string]bool) (string, error) {
			return "", errors.New("tab enumeration failed")
		},
	})
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("err=%v, want ErrOutcomeUnknown because the handoff was dispatched", err)
	}
	if errors.Is(err, ErrVerdictStop) {
		t.Fatalf("an item that reached the handoff was reported as re-doable: %v", err)
	}
	if result.State != ItemOutcomeUnknown {
		t.Fatalf("result state %q, want outcome_unknown to match the durable queue", result.State)
	}

	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.Items[0].State != ItemOutcomeUnknown {
		t.Fatalf("state=%s, want outcome_unknown", stored.Items[0].State)
	}
	if CanRecapture(stored.Items[0].State) {
		t.Fatalf("the item is still re-capturable after the handoff was dispatched")
	}
	if !RequiresHumanReview(stored.Items[0].State) {
		t.Fatalf("the item does not require human review")
	}
	if stored.Items[0].IdempotencyKey != "" {
		t.Fatalf("a key was recorded although the handoff result was never observed: %q", stored.Items[0].IdempotencyKey)
	}
	// The extension really did open the application tab, which is exactly what makes
	// the item ambiguous instead of re-doable.
	if got := fixtureAppTabCountWithin(t, driver, 3*time.Second); got != 1 {
		t.Fatalf("application tabs=%d, want 1 because the handoff was dispatched", got)
	}
}

// A target snapshot that cannot be taken must be reported as a failure, not as "no
// tabs existed". The previous item's application tab normally survives into the next
// item and still carries the previous item's key, so an empty snapshot makes it look
// like a tab this handoff just created; AppPageByKey would then click the previous
// payload while the current item records its own key (design section 4 D1.2 guard 1).
func TestFixtureDriverReportsAFailedTargetSnapshot(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)

	// The healthy session is the control: the snapshot must work, otherwise an
	// always-failing implementation would satisfy the assertion below.
	healthy, err := driver.pageTargetIDs()
	if err != nil {
		t.Fatalf("snapshot on a healthy session: %v", err)
	}
	if len(healthy) == 0 {
		t.Fatal("the healthy session reported no page targets at all")
	}

	// Detaching the browser session makes every Target command fail, which is the
	// same observable result as a transient enumeration failure.
	if err := driver.browserSession.Detach(); err != nil {
		t.Fatalf("detach browser session: %v", err)
	}
	failed, err := driver.pageTargetIDs()
	if err == nil {
		t.Fatalf("a failed snapshot was reported as the set %v; the previous item's application tab would pass as newly created", failed)
	}
}

// A browser that can no longer be asked for the snapshot must fail before the handoff
// click, so nothing is handed to the application.
//
// This asserts the ordering property rather than the snapshot's error return itself:
// with the session detached the popup cannot read its own state either, so either
// failure may be the one that surfaces. The snapshot's error return is pinned
// separately, at the driver and at the source (`popup_contract_test.go`).
func TestFixtureDriverDoesNotDispatchHandoffWhenTheSessionIsGone(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)

	popup, err := driver.PrepareItem(fixtureSource)
	if err != nil {
		t.Fatalf("prepare item: %v", err)
	}
	t.Cleanup(func() { _ = popup.Close() })
	if _, err := popup.Capture(); err != nil {
		t.Fatalf("capture: %v", err)
	}

	if err := driver.browserSession.Detach(); err != nil {
		t.Fatalf("detach browser session: %v", err)
	}
	if _, err := popup.PrepareHandoff(); err == nil {
		t.Fatal("PrepareHandoff succeeded although the browser reported no targets")
	}
	if _, err := popup.Handoff(); err == nil {
		t.Fatal("the composed handoff proceeded although the browser was unreachable")
	}
	// No click means the item never left the executor, so it stays re-doable instead
	// of becoming ambiguous.
	if got := fixtureAppTabCountWithin(t, driver, 2*time.Second); got != 0 {
		t.Fatalf("application tabs=%d, want 0 because the handoff must not be dispatched", got)
	}
}

// fixtureUnapprovedQueue writes a one-item queue that carries no confirmed scope,
// which is the state design section 4 D1.4 leaves a batch in until its first item
// has been delivered and a person has confirmed the identity the application
// reports. It is deliberately distinct from fixtureQueue, which is post-confirmation.
func fixtureUnapprovedQueue(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "queue.json")
	queue := NewQueue("fixture-batch")
	queue.Items = append(queue.Items, Item{Seq: 1, URL: fixtureSource, State: ItemQueued})
	if err := queue.Save(path); err != nil {
		t.Fatalf("save queue: %v", err)
	}
	return path
}

// TestFixtureDriverConfirmsTheApplicationReportedScope is the F5-1 acceptance test
// for design section 4 D1.4: an unapproved batch becomes attributed to the identity
// the APPLICATION reports, after a person confirms it, and only then is anything
// submitted. The declared scope is only an expectation that has to agree.
func TestFixtureDriverConfirmsTheApplicationReportedScope(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureUnapprovedQueue(t)

	var asked []AppScope
	result, err := ImportOne(driver, ImportOptions{
		QueuePath: queuePath,
		Approved:  fixtureApprovedScope,
		ConfirmScope: func(scope AppScope) (bool, error) {
			asked = append(asked, scope)
			return true, nil
		},
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if len(asked) != 1 {
		t.Fatalf("the confirmation was requested %d times, want exactly 1", len(asked))
	}
	if asked[0] != fixtureApprovedScope {
		t.Fatalf("the person was asked about %+v, want the application-reported %+v", asked[0], fixtureApprovedScope)
	}
	if result.State != ItemSubmitted {
		t.Fatalf("state %q, want submitted", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if !stored.ScopeApproved || !stored.ScopeMatches(fixtureApprovedScope.ActorID, fixtureApprovedScope.OrganizationID) {
		t.Fatalf("the confirmed scope was not persisted: %+v", stored)
	}
	page := fixtureAppPageByKey(t, driver, stored.Items[0].IdempotencyKey)
	if clicks := fixtureClicks(t, page); clicks != 1 {
		t.Fatalf("the confirmation was clicked %d times, want 1", clicks)
	}
}

// TestFixtureDriverStopsWhenThePersonDeclinesTheScope is design section 4 D1.4's
// refusal: no person confirmed the identity, so nothing may be attributed to it. The
// payload is already visible to the application, so the item is left undecidable
// rather than re-doable, exactly like every other post-handoff stop.
func TestFixtureDriverStopsWhenThePersonDeclinesTheScope(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, fixtureApprovedScope)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureUnapprovedQueue(t)

	result, err := ImportOne(driver, ImportOptions{
		QueuePath:    queuePath,
		Approved:     fixtureApprovedScope,
		ConfirmScope: func(AppScope) (bool, error) { return false, nil },
	})
	if !errors.Is(err, ErrScopeUnconfirmed) {
		t.Fatalf("err=%v, want ErrScopeUnconfirmed", err)
	}
	if !errors.Is(err, ErrOutcomeUnknown) {
		t.Fatalf("a declined scope after the handoff is not ambiguous: %v", err)
	}
	if result.State != ItemOutcomeUnknown {
		t.Fatalf("result state %q, want outcome_unknown to match the durable queue", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.ScopeApproved {
		t.Fatalf("a refusal approved the batch: %+v", stored)
	}
	if stored.Items[0].State != ItemOutcomeUnknown {
		t.Fatalf("state %q, want outcome_unknown", stored.Items[0].State)
	}
	if CanRecapture(stored.Items[0].State) {
		t.Fatalf("a declined scope left the item re-capturable: %+v", stored.Items[0])
	}
	page := fixtureAppPageByKey(t, driver, stored.Items[0].IdempotencyKey)
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("the confirmation was clicked %d times although nobody confirmed the scope", clicks)
	}
}

// TestFixtureDriverRefusesAnUnapprovedQueueWhenTheApplicationDisagrees pins the
// declared-vs-reported comparison on the pre-approval path: the flags are an
// expectation, and an application reporting something else stops the batch before
// anyone is even asked to confirm it.
func TestFixtureDriverRefusesAnUnapprovedQueueWhenTheApplicationDisagrees(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureApp(t, AppScope{ActorID: "fixture-actor-a", OrganizationID: "fixture-org-someone-else"})
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureUnapprovedQueue(t)

	asked := false
	result, err := ImportOne(driver, ImportOptions{
		QueuePath: queuePath,
		Approved:  fixtureApprovedScope,
		ConfirmScope: func(AppScope) (bool, error) {
			asked = true
			return true, nil
		},
	})
	if !errors.Is(err, ErrScopeMismatch) {
		t.Fatalf("err=%v, want ErrScopeMismatch", err)
	}
	if asked {
		t.Fatalf("a person was asked to confirm a scope the application did not report")
	}
	if result.State != ItemOutcomeUnknown {
		t.Fatalf("result state %q, want outcome_unknown to match the durable queue", result.State)
	}
	stored, loadErr := Load(queuePath)
	if loadErr != nil {
		t.Fatalf("reload queue: %v", loadErr)
	}
	if stored.ScopeApproved {
		t.Fatalf("a mismatch approved the batch: %+v", stored)
	}
	if stored.Items[0].State != ItemOutcomeUnknown {
		t.Fatalf("state %q, want outcome_unknown", stored.Items[0].State)
	}
	page := fixtureAppPageByKey(t, driver, stored.Items[0].IdempotencyKey)
	if clicks := fixtureClicks(t, page); clicks != 0 {
		t.Fatalf("the confirmation was clicked %d times under a mismatched scope", clicks)
	}
}

// TestFixtureDriverWaitsForTheApplicationPageToHydrate is the end-to-end acceptance for
// the readiness wait: the application's URL carries the handoff key from the first paint,
// while its verified scope and enabled submit control only appear once its own context
// reads resolve. A one-shot read taken in that window is an empty document, and recording
// it as an outcome used to block the item permanently even though the page was only slow.
func TestFixtureDriverWaitsForTheApplicationPageToHydrate(t *testing.T) {
	browser, extDir := requireFixtureEnv(t)
	startFixtureAppHydratingLate(t, fixtureApprovedScope, 1500*time.Millisecond)
	driver := launchFixtureDriver(t, browser, extDir)
	routeFixtureProduct(t, driver)
	queuePath := fixtureQueue(t, fixtureApprovedScope)

	result, err := ImportOne(driver, ImportOptions{QueuePath: queuePath, Approved: fixtureApprovedScope})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.State != ItemSubmitted {
		t.Fatalf("result state %q, want submitted: the driver must wait out the render", result.State)
	}
	if result.OperationID != fixtureOperationID {
		t.Fatalf("operation id %q, want %q", result.OperationID, fixtureOperationID)
	}
	page := fixtureAppPageByKey(t, driver, result.IdempotencyKey)
	if clicks := fixtureClicks(t, page); clicks != 1 {
		t.Fatalf("confirmation clicked %d times, want exactly 1", clicks)
	}
}
