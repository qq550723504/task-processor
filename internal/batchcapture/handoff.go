package batchcapture

// Handoff reference parsing.
//
// The extension generates the idempotency key and the handoff id itself
// (`controller.ts`: `this.flow.prepare(this.payload, crypto.randomUUID(),
// crypto.randomUUID())`) and places them in the fragment of the application URL
// it opens (`handoff.ts` handoffURL). The fragment is therefore the only place the
// executor can learn the key, and design section 4 D2 requires the key to be
// persisted so a restarted executor can read the operation back instead of
// resubmitting it.
//
// Nothing here generates or modifies a key: a key the executor invented would not
// match the operation the extension already bound the capture to.

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
)

// ErrHandoffRef reports a URL that is not an extension handoff URL.
var ErrHandoffRef = errors.New("not an extension handoff url")

// uuidPattern mirrors the extension's UUID pattern in validation.ts, which is the
// pattern the key and handoff id were generated against.
var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[1-8][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// recoveryParameter is the fragment parameter the application's capture page
// leaves behind. capture-receiver.tsx replaces the handoff fragment with
// `#operationKey=<original UUID>` before any POST, and handoff.ts recoveryURL()
// builds the same fragment from the same value, so it names the same capture.
const recoveryParameter = "operationKey"

// HandoffRef is the identity of one capture as the executor can observe it.
type HandoffRef struct {
	// ExtensionID and HandoffID are present only while the handoff fragment is
	// still in place. The application replaces it with the normalised recovery
	// fragment as soon as it reads the payload, so a reference read later carries
	// the key alone.
	ExtensionID    string
	HandoffID      string
	IdempotencyKey string
}

// IsZero reports whether no capture reference was parsed.
func (h HandoffRef) IsZero() bool { return h == HandoffRef{} }

// captureRefFromURL reads either fragment form the application capture URL can
// carry. Both forms are the same capture: the application normalises the handoff
// fragment to `#operationKey=<key>` (capture-receiver.tsx:62), and the extension's
// own recovery URL uses the identical fragment (handoff.ts recoveryURL).
func captureRefFromURL(rawURL string) (HandoffRef, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return HandoffRef{}, fmt.Errorf("%w: %v", ErrHandoffRef, err)
	}
	if parsed.Fragment == "" {
		return HandoffRef{}, fmt.Errorf("%w: no fragment", ErrHandoffRef)
	}
	values, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		return HandoffRef{}, fmt.Errorf("%w: %v", ErrHandoffRef, err)
	}
	// The recovery form is unambiguous and is checked first: it carries no
	// extension id, so it cannot be mistaken for a handoff fragment.
	if key := values.Get(recoveryParameter); key != "" {
		if len(values) != 1 {
			return HandoffRef{}, fmt.Errorf("%w: recovery fragment carries extra parameters", ErrHandoffRef)
		}
		if !uuidPattern.MatchString(key) {
			return HandoffRef{}, fmt.Errorf("%w: recovery key is not well-formed", ErrHandoffRef)
		}
		if parsed.RawQuery != "" || parsed.User != nil {
			return HandoffRef{}, fmt.Errorf("%w: capture url must carry no query or credentials", ErrHandoffRef)
		}
		return HandoffRef{IdempotencyKey: key}, nil
	}
	return handoffFragmentKey(rawURL)
}

// handoffFragmentKey parses the fragment the extension itself sets on the
// application URL (handoff.ts handoffURL).
//
// It accepts the fragment only, and only the three parameters the extension sets.
// A URL carrying a recovery fragment is handled by captureRefFromURL instead.
func handoffFragmentKey(rawURL string) (HandoffRef, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return HandoffRef{}, fmt.Errorf("%w: %v", ErrHandoffRef, err)
	}
	if parsed.Fragment == "" {
		return HandoffRef{}, fmt.Errorf("%w: no fragment", ErrHandoffRef)
	}
	values, err := url.ParseQuery(parsed.Fragment)
	if err != nil {
		return HandoffRef{}, fmt.Errorf("%w: %v", ErrHandoffRef, err)
	}
	ref := HandoffRef{
		ExtensionID:    values.Get("extensionId"),
		HandoffID:      values.Get("handoffId"),
		IdempotencyKey: values.Get("idempotencyKey"),
	}
	if ref.ExtensionID == "" || ref.HandoffID == "" || ref.IdempotencyKey == "" {
		return HandoffRef{}, fmt.Errorf("%w: fragment is not a handoff", ErrHandoffRef)
	}
	if !uuidPattern.MatchString(ref.HandoffID) || !uuidPattern.MatchString(ref.IdempotencyKey) {
		return HandoffRef{}, fmt.Errorf("%w: handoff identifiers are not well-formed", ErrHandoffRef)
	}
	// Any other parameter would mean the fragment carries something this executor
	// does not understand, which must not be silently forwarded or persisted.
	for key := range values {
		switch key {
		case "extensionId", "handoffId", "idempotencyKey":
		default:
			return HandoffRef{}, fmt.Errorf("%w: unexpected fragment parameter %q", ErrHandoffRef, key)
		}
	}
	if parsed.RawQuery != "" || parsed.User != nil {
		return HandoffRef{}, fmt.Errorf("%w: handoff url must carry no query or credentials", ErrHandoffRef)
	}
	return ref, nil
}
