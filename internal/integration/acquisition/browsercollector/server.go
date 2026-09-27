package browsercollector

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	browser "task-processor/internal/integration/acquisition/a1688/browser"
	"task-processor/internal/product/sourcing"
)

// AdmitFunc decides whether an inbound RPC call may drive the browser. It is
// the executable caller-admission seam required by design D13 / finding #14:
// "not reachable from outside and holds no tenant credential" is not by itself
// proof that only the current-application can call, because any workload on the
// same network could otherwise drive Chromium and spend the shared browser/IP
// budget.
//
// The concrete mechanism (network-namespace-only binding vs. mTLS / a dedicated
// service credential) is an open product/security decision (design 12-A1). This
// repository therefore ships no default: Handler refuses to start without one,
// so a deployment cannot silently come up with admission unenforced.
type AdmitFunc func(*http.Request) error

// Options configures the collector HTTP surface.
type Options struct {
	// Provider is the browser-backed acquirer. Required.
	Provider sourcing.PublicAcquirer
	// Admit is the caller-admission check. Required; see AdmitFunc.
	Admit AdmitFunc
}

// Handler builds the collector RPC handler. It returns an error rather than a
// working handler when admission or the provider is missing, so a deployment
// cannot run the collector with the admission control silently disabled.
func Handler(opts Options) (http.Handler, error) {
	if opts.Provider == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	if opts.Admit == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Admission runs before any parsing or provider work, so an unauthorized
		// caller never consumes a browser slot or an IP budget.
		if err := opts.Admit(r); err != nil {
			WriteError(w, http.StatusForbidden, CodeInvalidRequest)
			return
		}
		if r.Method != http.MethodPost {
			WriteError(w, http.StatusMethodNotAllowed, CodeInvalidRequest)
			return
		}
		// The handler is installed as the whole server handler, so every path
		// reaches it. Reject anything other than the single versioned RPC path
		// before any provider work, so a stale or misrouted admitted client
		// cannot spend the shared browser/IP budget outside the contract.
		if r.URL.Path != AcquirePath {
			WriteError(w, http.StatusNotFound, CodeInvalidRequest)
			return
		}
		req, err := ReadRequest(r)
		if err != nil {
			WriteError(w, http.StatusBadRequest, CodeInvalidRequest)
			return
		}
		source, err := sourcing.Canonical1688Source(req.SourceURL)
		if err != nil {
			WriteError(w, http.StatusBadRequest, CodeInvalidRequest)
			return
		}
		evidence, err := opts.Provider.Acquire(r.Context(), source)
		if err != nil {
			writeAcquireError(w, err)
			return
		}
		writeEvidence(w, evidence)
	}), nil
}

// writeAcquireError maps a provider failure onto the bounded wire vocabulary. A
// challenge or an unsupported page is a source failure, not a caller error, and
// is never dressed up as success (D4).
func writeAcquireError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, sourcing.ErrInvalidAcquisition):
		WriteError(w, http.StatusBadRequest, CodeInvalidRequest)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		WriteError(w, http.StatusGatewayTimeout, CodeBudgetExceeded)
	case errors.Is(err, browser.ErrCapacity):
		// A concurrency-limit rejection is retryable, not a source failure.
		WriteError(w, http.StatusServiceUnavailable, CodeCapacity)
	default:
		WriteError(w, http.StatusBadGateway, CodeSourceUnavailable)
	}
}

func writeEvidence(w http.ResponseWriter, evidence sourcing.AcquisitionEvidence) {
	// Bound the response before writing it. The page-side caps still allow a
	// large combination of variants and long strings, so an untrusted page could
	// otherwise make the collector marshal and stream a huge body that no
	// conforming client would ever accept.
	encoded, err := json.Marshal(evidence)
	if err != nil || len(encoded) > MaxResponseBytes {
		// Refuse rather than emit a body the peer must reject anyway.
		WriteError(w, http.StatusBadGateway, CodeSourceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}
