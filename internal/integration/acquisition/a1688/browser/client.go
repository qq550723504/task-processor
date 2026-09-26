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
	// navigateURLOverride replaces the navigation target. It exists only so the
	// browser fixture test can drive a real Chromium against a loopback fixture
	// page; production never sets it (the target is always source.URL).
	navigateURLOverride string
}

// DefaultTimeout mirrors the provider budget used by the design.
const DefaultTimeout = 90 * time.Second

func (o Options) navigationTimeout() time.Duration {
	if o.NavigationTimeout > 0 {
		return o.NavigationTimeout
	}
	return 30 * time.Second
}

func (o Options) budget() time.Duration {
	if o.Budget > 0 {
		return o.Budget
	}
	return DefaultTimeout
}

// Client is a PublicAcquirer backed by a controlled browser.
type Client struct{ opts Options }

// The browser provider must satisfy the current owner's acquisition contract.
var _ sourcing.PublicAcquirer = (*Client)(nil)

// New builds a browser-backed public acquirer. It performs no IO at
// construction; the browser starts per acquisition.
func New(opts Options) *Client { return &Client{opts: opts} }

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
		return sourcing.AcquisitionEvidence{}, ErrChallenge
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

// routeGuard aborts any request whose origin is not explicitly allowed.
func (c *Client) routeGuard(route playwright.Route) {
	req := route.Request()
	if !originAllowed(c.opts.AllowedOrigins, req.URL()) {
		_ = route.Abort()
		return
	}
	_ = route.Continue()
}

// originAllowed reports whether target is under one of the allowed origins.
// It compares the full origin (scheme://host[:port]) so a host that merely
// starts with an allowed prefix (e.g. detail.1688.com.evil.test) is rejected.
func originAllowed(allowed []string, target string) bool {
	origin := originOf(target)
	for _, a := range allowed {
		if originOf(a) == origin {
			return true
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
	Truncated   bool      `json:"truncated"`
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
