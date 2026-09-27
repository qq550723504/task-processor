// Package browser provides the server-side anonymous public 1688 acquisition
// provider backed by a controlled fingerprint browser.
//
// It is the src2b-public-browser-v1 provider. It produces only untrusted
// sourcing.AcquisitionEvidence; the current sourcing owner maps and validates it.
// It never uses 1688 login state, cookies, or a source account: the context is
// fresh and non-persistent (D3). The ported behavior (launch args, stealth
// init script, window.context field paths) comes from the operator's existing
// fingerprint browser implementation; see architecture design section 10.1.
package browser

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	sigjson "sigs.k8s.io/json"
	"task-processor/internal/product/sourcing"
)

var (
	// ErrUnavailable reports that the browser or driver could not be brought up.
	ErrUnavailable = errors.New("browser acquisition unavailable")
	// ErrUnsupported reports that the page did not carry a supported product
	// shape (e.g. a challenge/interstitial page with no window.context).
	ErrUnsupported = errors.New("public page structure unsupported")
	// ErrChallenge reports a detected anti-automation challenge page.
	ErrChallenge = errors.New("public page challenged")
	// ErrCapacity reports that this collector is already running its maximum
	// number of concurrent browser acquisitions (design D8).
	ErrCapacity = errors.New("browser collector at concurrency limit")
)

// ParserVersion identifies the extraction contract. It is part of the evidence
// fingerprint (content-independent per D1) and appears in envelope metadata.
const ParserVersion = "1688-browser-context-v1"

// Options configures one browser-backed provider. The collector process holds
// no database credentials and no tenant identity (D13); this carries only the
// browser binary and the anonymous-public allowlist.
type Options struct {
	// ExecutablePath is the browser binary. Production uses the fingerprint
	// Chromium; tests may point at any Chromium.
	ExecutablePath string
	// Headless selects headless operation.
	Headless bool
	// NavigationTimeout bounds one page navigation.
	NavigationTimeout time.Duration
	// Budget bounds one whole acquisition (browser start, navigation, extract).
	// The caller (application layer) already bounds the provider with a child
	// context; this is the inner defense.
	Budget time.Duration
	// AllowedOrigins is the exact allowlist of origins the browser may reach
	// (D12 defense-in-depth; the connection-layer allowlist is authoritative and
	// is a deployment concern, Slice 3). Empty disables the origin check, which
	// is only acceptable in isolated tests.
	AllowedOrigins []string
	// MaxConcurrent bounds how many browser acquisitions this process runs at
	// once (design D8 / 12-B2, initial value 2). Organization-scoped acquisition
	// limits do not bound a collector-wide burst, and every concurrent request
	// launches its own Chromium, so the cap has to live on the collector. Zero
	// means DefaultMaxConcurrent.
	MaxConcurrent int
	// MaxResponseBytes aborts an allowed subresource whose declared or observed
	// body exceeds this size, before Chromium materializes it (design D8).
	MaxResponseBytes int64
	// navigateURLOverride replaces the navigation target. It exists only so the
	// browser fixture test can drive a real Chromium against a loopback fixture
	// page; production never sets it (the target is always source.URL).
	navigateURLOverride string
}

// DefaultTimeout bounds one provider acquisition.
// It must stay well inside the current-application acquisition route budget
// (sourcing.AcquisitionTimeout = 20s), because D1 persists the operation only
// after acquisition returns: a provider that outlives the route leaves the user
// with a deadline and no replayable operation. The route timer also starts
// before the request body is read, so the budget must leave room for body read
// plus evidence mapping and publication, not merely fit under 20s on its own.
// 10s keeps the provider well inside that envelope; a request body for this
// endpoint is a few hundred bytes, so body read is negligible in practice.
//
// Raising this requires raising the route budget in the same change, and then
// the BFF (22s) and browser client (25s) deadlines as well.
const DefaultTimeout = 10 * time.Second

// DefaultMaxConcurrent is the collector-wide concurrency cap (design D8, 12-B2).
// It is deliberately small: each acquisition launches its own Chromium, and the
// shared egress IP is the scarce resource.
const DefaultMaxConcurrent = 2

// DefaultMaxSubresourceBytes bounds a single allowed subresource body so an
// oversized document or script is aborted before Chromium materializes it
// (design D8). It is generous for a 1688 detail page and its assets.
const DefaultMaxSubresourceBytes int64 = 8 << 20

// DefaultAllowedOrigins is the resolved egress allowlist (design A2, user
// decision 2026-09-26): the 1688 product host, its CDN, and 1688 site assets,
// over https only. The browser never fetches image bytes (evidence carries URLs
// only), so no object-storage host is admitted.
var DefaultAllowedOrigins = []string{
	"https://detail.1688.com",
	"https://*.alicdn.com",
	"https://*.1688.com",
}

// navigationTimeout bounds one page navigation. It is capped by the acquisition
// budget so a single navigation can never consume the whole budget on its own.
func (o Options) navigationTimeout() time.Duration {
	budget := o.budget()
	timeout := o.NavigationTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > budget {
		return budget
	}
	return timeout
}

func (o Options) maxConcurrent() int {
	if o.MaxConcurrent > 0 {
		return o.MaxConcurrent
	}
	return DefaultMaxConcurrent
}

func (o Options) maxResponseBytes() int64 {
	if o.MaxResponseBytes > 0 {
		return o.MaxResponseBytes
	}
	return DefaultMaxSubresourceBytes
}

func (o Options) budget() time.Duration {
	if o.Budget > 0 {
		return o.Budget
	}
	return DefaultTimeout
}

// Client is a PublicAcquirer backed by a controlled browser.
type Client struct {
	opts Options
	// slots is the collector-wide concurrency cap. Every acquisition launches a
	// separate Chromium, so a burst across organizations would otherwise exhaust
	// CPU, memory and the shared egress IP.
	slots chan struct{}
}

// The browser provider must satisfy the current owner's acquisition contract.
var _ sourcing.PublicAcquirer = (*Client)(nil)

// New builds a browser-backed public acquirer. It performs no IO at
// construction; the browser starts per acquisition.
// maxConcurrentInternal exposes the resolved cap for tests.
func (c *Client) maxConcurrentInternal() int { return c.opts.maxConcurrent() }

func New(opts Options) *Client {
	return &Client{opts: opts, slots: make(chan struct{}, opts.maxConcurrent())}
}

// Acquire fetches one anonymous public product page and returns untrusted
// evidence. It never retries automatically (D4); a detected challenge or an
// unsupported shape is reported honestly.
func (c *Client) Acquire(ctx context.Context, source sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
	canonical, err := sourcing.Canonical1688Source(source.URL)
	if err != nil || canonical != source {
		return sourcing.AcquisitionEvidence{}, sourcing.ErrInvalidAcquisition
	}
	if c == nil {
		return sourcing.AcquisitionEvidence{}, ErrUnavailable
	}
	if err := c.opts.validate(); err != nil {
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	ctx, cancel := context.WithTimeout(ctx, c.opts.budget())
	defer cancel()

	// Collector-wide concurrency cap (D8). Taken before any browser work so an
	// over-capacity burst is rejected instead of launching more Chromium
	// processes and consuming the shared egress IP.
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return sourcing.AcquisitionEvidence{}, ctx.Err()
	default:
		return sourcing.AcquisitionEvidence{}, ErrCapacity
	}

	pw, err := playwright.Run()
	if err != nil {
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: start playwright: %v", ErrUnavailable, err)
	}
	defer func() { _ = pw.Stop() }()

	// Fresh, non-persistent, anonymous context: no cookies, no login, no
	// cross-run profile (D3). A temp dir is used because Chromium requires a
	// user-data-dir; it is removed on exit.
	profile, err := os.MkdirTemp("", "a1688-anon-*")
	if err != nil {
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: temp profile: %v", ErrUnavailable, err)
	}
	defer func() { _ = os.RemoveAll(profile) }()

	context, err := pw.Chromium.LaunchPersistentContext(profile, c.launchOptions())
	if err != nil {
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: launch: %v", ErrUnavailable, err)
	}
	defer func() { _ = context.Close() }()

	// Defense-in-depth egress guard (D12). Enforced at the request callback; the
	// authoritative control is the connection-layer allowlist (Slice 3).
	if len(c.opts.AllowedOrigins) > 0 {
		if err := context.Route("**/*", c.routeGuard); err != nil {
			return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: install route guard: %v", ErrUnavailable, err)
		}
	}
	if err := context.AddInitScript(playwright.Script{Content: playwright.String(stealthScript())}); err != nil {
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: stealth script: %v", ErrUnavailable, err)
	}

	pages := context.Pages()
	var page playwright.Page
	if len(pages) > 0 {
		page = pages[0]
	} else {
		page, err = context.NewPage()
		if err != nil {
			return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: new page: %v", ErrUnavailable, err)
		}
	}
	navigateURL := source.URL
	if c.opts.navigateURLOverride != "" {
		navigateURL = c.opts.navigateURLOverride
	}
	if _, err := page.Goto(navigateURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(float64(c.opts.navigationTimeout().Milliseconds())),
	}); err != nil {
		if ctx.Err() != nil {
			return sourcing.AcquisitionEvidence{}, ctx.Err()
		}
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: navigate: %v", ErrUnavailable, err)
	}
	if err := ctx.Err(); err != nil {
		return sourcing.AcquisitionEvidence{}, err
	}

	// Detect an anti-automation challenge (punish/captcha) before extracting.
	challenged, err := detectChallenge(page)
	if err != nil {
		return sourcing.AcquisitionEvidence{}, err
	}
	if challenged {
		// Design A4: at most ONE bounded automatic attempt, then an honest
		// failure. No manual path, no retry-to-success, no fabricated product.
		// An authentication wall is not solvable by dragging a slider, so the
		// attempt is skipped rather than burning the captcha budget on it.
		if !isAuthenticationWall(page) {
			if _, solveErr := trySolveCaptcha(ctx, page); solveErr != nil && ctx.Err() != nil {
				return sourcing.AcquisitionEvidence{}, ctx.Err()
			}
		}
		if stillChallenged, checkErr := detectChallenge(page); checkErr != nil {
			return sourcing.AcquisitionEvidence{}, checkErr
		} else if stillChallenged {
			return sourcing.AcquisitionEvidence{}, ErrChallenge
		}
	}

	raw, err := page.Evaluate(extractScript())
	if err != nil {
		if ctx.Err() != nil {
			return sourcing.AcquisitionEvidence{}, ctx.Err()
		}
		return sourcing.AcquisitionEvidence{}, fmt.Errorf("%w: evaluate: %v", ErrUnavailable, err)
	}
	evidence, err := decodeEvidence(source, raw)
	if err != nil {
		return sourcing.AcquisitionEvidence{}, err
	}
	return evidence, nil
}

func (o Options) validate() error {
	if o.ExecutablePath == "" {
		return errors.New("executable path is required")
	}
	if _, err := os.Stat(o.ExecutablePath); err != nil {
		return fmt.Errorf("browser binary: %v", err)
	}
	return nil
}

// launchOptions builds minimal anonymous-public launch settings: hide obvious
// automation flags, ignore the default automation-disabling args (ported from
// the operator's implementation).
func (c *Client) launchOptions() playwright.BrowserTypeLaunchPersistentContextOptions {
	return playwright.BrowserTypeLaunchPersistentContextOptions{
		ExecutablePath: playwright.String(c.opts.ExecutablePath),
		Headless:       playwright.Bool(c.opts.Headless),
		Args: []string{
			"--no-first-run",
			"--no-default-browser-check",
			"--disable-blink-features=AutomationControlled",
		},
		IgnoreDefaultArgs: []string{"--enable-automation"},
		Viewport:          &playwright.Size{Width: 1920, Height: 1080},
	}
}

// routeGuard admits only allowlisted origins, and bounds the main document
// response before Chromium materializes it (design D8).
//
// The document is fetched through the route API and re-fulfilled from a bounded
// body, so an oversized page is aborted rather than downloaded in full. Only the
// navigation document is handled this way: subresources keep the plain continue
// path, because rewriting every subresource response is a much larger change to
// a path that has real-network evidence behind it and could not be re-verified
// against the live site while the egress IP is challenged. The bounded extractor
// and the RPC transport cap remain the backstop for subresources; the residual
// subresource download limit is recorded rather than silently assumed.
func (c *Client) routeGuard(route playwright.Route) {
	req := route.Request()
	if !originAllowed(c.opts.AllowedOrigins, req.URL()) {
		_ = route.Abort()
		return
	}
	if !req.IsNavigationRequest() || req.ResourceType() != "document" {
		_ = route.Continue()
		return
	}
	resp, err := route.Fetch()
	if err != nil {
		_ = route.Abort()
		return
	}
	limit := c.opts.maxResponseBytes()
	// Reject on the declared size first, so a huge document is not transferred.
	if declared, perr := strconv.ParseInt(resp.Headers()["content-length"], 10, 64); perr == nil && declared > limit {
		_ = route.Abort()
		return
	}
	body, berr := resp.Body()
	if berr != nil {
		_ = route.Abort()
		return
	}
	if int64(len(body)) > limit {
		_ = route.Abort()
		return
	}
	status := resp.Status()
	_ = route.Fulfill(playwright.RouteFulfillOptions{
		Status:  &status,
		Headers: resp.Headers(),
		Body:    string(body),
	})
}

// originAllowed reports whether target is under one of the allowed origins.
// It compares the full origin (scheme://host[:port]) so a host that merely
// starts with an allowed prefix (e.g. detail.1688.com.evil.test) is rejected.
//
// An allowlist entry may use a single leading "*." label (design A2), e.g.
// "https://*.alicdn.com". The wildcard matches exactly one label, so
// "https://*.alicdn.com" admits "https://img.alicdn.com" but not
// "https://a.b.alicdn.com" and never "https://alicdn.com.evil.test".
func originAllowed(allowed []string, target string) bool {
	origin := originOf(target)
	scheme, host, ok := strings.Cut(origin, "://")
	if !ok {
		host = origin
	}
	for _, entry := range allowed {
		entryScheme, entryHost, entryOK := strings.Cut(originOf(entry), "://")
		if !entryOK {
			// No scheme in the entry: treat the whole value as a host.
			entryScheme, entryHost = "", originOf(entry)
		}
		if entryScheme != scheme {
			continue
		}
		if entryHost == host {
			return true
		}
		if suffix, isWildcard := strings.CutPrefix(entryHost, "*."); isWildcard {
			// Exactly one extra label, and the remainder must match the suffix.
			if label, rest, found := strings.Cut(host, "."); found && label != "" && !strings.Contains(label, ".") && rest == suffix {
				return true
			}
		}
	}
	return false
}

func originOf(raw string) string {
	idx := strings.Index(raw, "://")
	if idx < 0 {
		return strings.ToLower(raw)
	}
	rest := raw[idx+3:]
	if slash := strings.IndexAny(rest, "/?#"); slash >= 0 {
		rest = rest[:slash]
	}
	return strings.ToLower(raw[:idx+3] + rest)
}

// evidencePayload is the bounded structure returned by the page-side extractor.
type evidencePayload struct {
	OfferID     string    `json:"offerId"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Images      []string  `json:"images"`
	Attributes  []kv      `json:"attributes"`
	Variants    []variant `json:"variants"`
	PriceFacts  []price   `json:"priceFacts"`
	// TruncatedFields names every collection or field the page-side caps
	// actually clipped, plus every numeric value JavaScript could not represent
	// exactly. It is never silently dropped: each entry becomes an explicit
	// warning and missing fact on the evidence.
	TruncatedFields []string `json:"truncatedFields"`
}

type kv struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type variant struct {
	SourceID   string `json:"sourceId"`
	Attributes []kv   `json:"attributes"`
	Price      *price `json:"price"`
}

type price struct {
	Amount      string `json:"amount"`
	Currency    string `json:"currency"`
	MinQuantity string `json:"minQuantity"`
}

// decodeEvidence converts the page payload into untrusted AcquisitionEvidence.
func decodeEvidence(source sourcing.AcquisitionSource, raw any) (sourcing.AcquisitionEvidence, error) {
	encoded, err := json.Marshal(raw)
	if err != nil {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	var payload evidencePayload
	// Unlike the transport decoders, this one does not reject unknown fields:
	// the payload is a projection of third-party page data, which legitimately
	// carries fields we do not model. Duplicate keys cannot occur because the
	// value came back as a JavaScript object, not a JSON document.
	if _, err := sigjson.UnmarshalStrict(encoded, &payload, sigjson.DisallowDuplicateFields); err != nil {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	if payload.OfferID != source.OfferID {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	if strings.TrimSpace(payload.Title) == "" {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	captured := time.Now().UTC()
	evidence := sourcing.AcquisitionEvidence{
		SchemaVersion: 1,
		SourceURL:     source.URL,
		OfferID:       source.OfferID,
		ParserVersion: ParserVersion,
		CapturedAt:    captured,
	}
	title := strings.TrimSpace(payload.Title)
	evidence.Title = &title
	if d := strings.TrimSpace(payload.Description); d != "" {
		evidence.Description = &d
	}
	for _, kv := range payload.Attributes {
		name, value := strings.TrimSpace(kv.Name), strings.TrimSpace(kv.Value)
		if name == "" || value == "" {
			continue
		}
		evidence.Attributes = append(evidence.Attributes, sourcing.AcquisitionAttribute{Name: name, Value: value})
	}
	seen := map[string]bool{}
	for _, img := range payload.Images {
		u := strings.TrimSpace(img)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		evidence.Images = append(evidence.Images, sourcing.AcquisitionImage{URL: u, Role: "source"})
	}
	for _, v := range payload.Variants {
		av := sourcing.AcquisitionVariant{}
		if id := strings.TrimSpace(v.SourceID); id != "" {
			idCopy := id
			av.SourceID = &idCopy
		}
		for _, a := range v.Attributes {
			name, value := strings.TrimSpace(a.Name), strings.TrimSpace(a.Value)
			if name == "" {
				continue
			}
			av.Attributes = append(av.Attributes, sourcing.AcquisitionAttribute{Name: name, Value: value})
		}
		if v.Price != nil {
			av.Price = toAcquisitionPrice(*v.Price)
		}
		evidence.Variants = append(evidence.Variants, av)
	}
	for _, p := range payload.PriceFacts {
		if ap := toAcquisitionPrice(p); ap != nil {
			evidence.PriceFacts = append(evidence.PriceFacts, *ap)
		}
	}
	digest := sha256.Sum256(encoded)
	evidence.ContentSHA256 = hex.EncodeToString(digest[:])
	// Make every page-side clip explicit. A truncated or imprecise field must
	// never reach publication looking like the exact source fact.
	for _, field := range payload.TruncatedFields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		evidence.Warnings = append(evidence.Warnings, sourcing.SourceWarning{
			Code:    "source_evidence_truncated",
			Field:   field,
			Message: "page-side evidence caps clipped or dropped this field",
		})
		evidence.MissingFacts = append(evidence.MissingFacts, sourcing.MissingFact{
			Field:  field,
			Reason: "browser evidence for this field was clipped or not exactly representable and is incomplete",
		})
	}
	return evidence, nil
}

func toAcquisitionPrice(p price) *sourcing.AcquisitionPrice {
	amount := strings.TrimSpace(p.Amount)
	if amount == "" {
		return nil
	}
	out := &sourcing.AcquisitionPrice{Amount: amount}
	if c := strings.TrimSpace(p.Currency); c != "" {
		cCopy := c
		out.Currency = &cCopy
	}
	if q := strings.TrimSpace(p.MinQuantity); q != "" {
		qCopy := q
		out.MinQuantity = &qCopy
	}
	return out
}
