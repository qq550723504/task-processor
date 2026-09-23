package batchcapture

import "strings"

// Challenge classifiers implement design section 4 D1.3. The rule exists because
// a risk-control page and a genuinely delisted product can look identical from
// the extension's reply: internal/extension/background.ts collapses every
// capture failure to ACTION_UNAVAILABLE, so the reason can never be read from
// the extension. The only trustworthy source is the executor's own observation of
// the page after navigation and before it triggers a capture.
//
// The decision is deliberately two independent judgments:
//
//	A. challenge / risk-control judgment — the page is a human-verification or
//	   login gate. This stops the whole batch, because continuing would hammer a
//	   gate that only a person can clear (design section 1.6: captcha is the one
//	   place a human is required).
//	B. product-data judgment — product fields were obtained, so the item is a
//	   normal capture, or they were not and the page is nonetheless definitive
//	   about the product being gone, which is a single-item failure.
//
// When neither judgment fires the residual case is deliberately resolved as
// "pause": a false pause costs a glance, while a false continue silently skips
// an item that may be sitting behind risk control.

// Verdict is what the executor should do with the current item.
type Verdict string

const (
	// VerdictProceed means product data was observed; continue with capture.
	VerdictProceed Verdict = "proceed"
	// VerdictPauseBatch means the whole batch must stop and be handed to a
	// person. It is never treated as a single-item failure.
	VerdictPauseBatch Verdict = "pause_batch"
	// VerdictSingleItemFailure means the item is definitively unusable (delisted
	// or invalid). Per design section 7 the batch continues with the next item.
	VerdictSingleItemFailure Verdict = "single_item_failure"
	// VerdictIndeterminate means the executor made no observation at all (for
	// example navigation failed before any page was rendered). It is kept
	// distinct from pause so callers cannot silently treat "I did not look" as a
	// decision, but it fails closed exactly like pause.
	VerdictIndeterminate Verdict = "indeterminate"
)

// riskControlMarkers are the response/body markers this repository has already
// measured on 1688 risk-control pages. See
// docs/superpowers/specs/2026-09-20-issue399-1688-context-locator-design.md:168,
// which records real pages being marked as `punish?x5secdata=...` after repeated
// automated navigation, and the design doc's own note that anonymous requests
// hit risk control returning HTTP 200 with Bxpunish/x5secdata and no product
// data. They are NOT invented selectors.
var riskControlMarkers = []string{"bxpunish", "x5secdata"}

// verificationURLMarkers are path/fragment fragments that identify a human
// verification or login gate rather than a product page.
var verificationURLMarkers = []string{
	"/punish",
	"x5secdata",
	"login.1688.com",
	"/member/login",
	"passport.1688.com",
}

// delistedMarkers are body phrases that make "this product is gone" definitive.
// They are matched case-insensitively and only ever used to decide a SINGLE item
// failure, never to continue a batch past a challenge.
var delistedMarkers = []string{
	"商品不存在",
	"商品已下架",
	"该商品已删除",
	"宝贝不存在或已删除",
	"item not found",
	"product not found",
}

// Observation is what the executor saw on the product page after navigation and
// before triggering a capture. Only fields the executor can actually read are
// present; there is intentionally no field for "the extension said X", because
// the extension cannot report a reason.
type Observation struct {
	// FinalURL is the URL after navigation settled.
	FinalURL string
	// BodyText is the visible page text.
	BodyText string
	// RawHTML is the page source. Risk-control markers are not visible text: the
	// measured 1688 responses carry them in element attributes and inline scripts,
	// so matching only visible text misses every real challenge. RawHTML is
	// therefore the authoritative field for marker matching and BodyText is kept
	// only for diagnosis.
	RawHTML string
	// ProductDataPresent reports whether the product fields the capture contract
	// requires were actually read from the DOM.
	ProductDataPresent bool
	// Performed is false when navigation or evaluation failed outright, so no
	// judgment was made at all.
	Performed bool
}

// pageEvidence is the text markers are matched against: the page source when it
// was readable, and the visible text otherwise.
func (o Observation) pageEvidence() string {
	if o.RawHTML != "" {
		return o.RawHTML
	}
	return o.BodyText
}

// Classify applies design section 4 D1.3 to one page observation.
func Classify(o Observation) Verdict {
	if !o.Performed {
		return VerdictIndeterminate
	}
	// A. The challenge judgment is evaluated first and is never overridable.
	// A page that is simultaneously a verification gate and shows product data is
	// contradictory; stopping is the only safe reading.
	evidence := o.pageEvidence()
	if IsChallengePage(o.FinalURL, evidence) {
		return VerdictPauseBatch
	}
	// B. The product-data judgment.
	if o.ProductDataPresent {
		return VerdictProceed
	}
	if IsDefinitivelyDelisted(evidence) {
		return VerdictSingleItemFailure
	}
	// Residual ambiguity: no product data, no challenge marker, and no
	// definitive "gone" statement. Default to pausing rather than skipping.
	return VerdictPauseBatch
}

// IsChallengePage reports whether the page is a risk-control or login gate.
func IsChallengePage(finalURL, bodyText string) bool {
	loweredURL := strings.ToLower(finalURL)
	for _, marker := range verificationURLMarkers {
		if strings.Contains(loweredURL, marker) {
			return true
		}
	}
	return HasRiskControlMarker(bodyText)
}

// HasRiskControlMarker reports whether the page body carries a measured
// risk-control marker. It is exported because the same evidence is worth
// asserting on directly in tests and diagnostics.
func HasRiskControlMarker(bodyText string) bool {
	lowered := strings.ToLower(bodyText)
	for _, marker := range riskControlMarkers {
		if strings.Contains(lowered, marker) {
			return true
		}
	}
	return false
}

// IsDefinitivelyDelisted reports whether the page states unambiguously that the
// product is gone. Anything weaker than an explicit statement is NOT treated as
// definitive, because a wrong "delisted" reading skips an item that may have
// been blocked by risk control instead.
func IsDefinitivelyDelisted(bodyText string) bool {
	lowered := strings.ToLower(bodyText)
	for _, marker := range delistedMarkers {
		if strings.Contains(lowered, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}

// VerdictAction describes how the batch loop must respond, so a caller cannot
// accidentally treat a pause as a skip.
type VerdictAction struct {
	Verdict Verdict
	// StopBatch is true when the whole batch must stop and be handed to a person.
	StopBatch bool
	// ContinueToNextItem is true only for a definitive single-item failure.
	ContinueToNextItem bool
	// RedoCurrentItemAfterHuman is true when the same item must be re-done after
	// a person intervenes (design section 4 D1.3): the item is neither skipped nor
	// finalised.
	RedoCurrentItemAfterHuman bool
}

// Action maps a verdict onto the batch loop's behaviour.
func (v Verdict) Action() VerdictAction {
	switch v {
	case VerdictProceed:
		return VerdictAction{Verdict: v}
	case VerdictSingleItemFailure:
		return VerdictAction{Verdict: v, ContinueToNextItem: true}
	default:
		// Pause and indeterminate both stop the batch. Pause additionally means
		// the current item is re-done once a person has cleared the gate; an
		// indeterminate observation is re-attempted the same way.
		return VerdictAction{Verdict: v, StopBatch: true, RedoCurrentItemAfterHuman: true}
	}
}
