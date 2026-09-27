// Package browsercollector holds the internal RPC contract between the
// current-application and the anonymous public browser collector process
// (design D13, section 12-B5).
//
// The contract is deliberately minimal and carries no tenant identity: the
// request holds only the canonical public source URL, and the response holds
// only untrusted, bounded AcquisitionEvidence. Authorization, idempotency,
// operation rows, and publication all stay in the current-application. The
// collector has no database credentials and no database network reachability
// (D13), which is what makes the D12 egress boundary structurally hold.
package browsercollector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	sigjson "sigs.k8s.io/json"
	"task-processor/internal/product/sourcing"
)

// AcquirePath is the single RPC method. It is versioned so a stale caller is
// rejected explicitly instead of silently misreading a changed shape.
const AcquirePath = "/internal/v1/browser-collector/acquire"

// MaxResponseBytes bounds the evidence body before it is decoded. The
// collector already caps page-side collections (D8); this is the second,
// independent bound on the transport so a hostile or broken peer cannot force
// an unbounded allocation here (design section 12-B3, aligned with
// MaxAcquisitionCommandBytes semantics).
const MaxResponseBytes = 2 << 20

// MaxRequestBytes bounds the request body. The request is tiny by contract.
const MaxRequestBytes = 4 << 10

// AcquireRequest is the wire request. It intentionally has no organization,
// actor, role, or token field: the collector must never hold or act on tenant
// identity (D7, D13).
type AcquireRequest struct {
	SourceURL string `json:"sourceURL"`
}

// Validate rejects any request that is not exactly the canonical public source.
func (r AcquireRequest) Validate() error {
	source, err := sourcing.Canonical1688Source(r.SourceURL)
	if err != nil {
		return fmt.Errorf("%w: non-canonical source", sourcing.ErrInvalidAcquisition)
	}
	if source.URL != r.SourceURL {
		return fmt.Errorf("%w: source is not canonical", sourcing.ErrInvalidAcquisition)
	}
	return nil
}

// ErrorCode is the bounded collector error vocabulary. It never carries detail
// strings, tenant data, or page content, so an error body cannot leak the
// visited page or become a covert channel.
type ErrorCode string

const (
	// CodeInvalidRequest means the request was not a canonical public source.
	CodeInvalidRequest ErrorCode = "invalid_request"
	// CodeSourceUnavailable means the page could not be collected (including a
	// detected anti-automation challenge). The collector never invents a
	// product to hide this (D4).
	CodeSourceUnavailable ErrorCode = "source_unavailable"
	// CodeBudgetExceeded means the acquisition exceeded its own time budget.
	CodeBudgetExceeded ErrorCode = "budget_exceeded"
)

// ErrorBody is the bounded error response.
type ErrorBody struct {
	Code ErrorCode `json:"code"`
}

// CollectorError couples a wire code with the domain sentinel the
// current-application maps back onto.
type CollectorError struct {
	Code     ErrorCode
	Sentinel error
}

func (e *CollectorError) Error() string {
	return fmt.Sprintf("browser collector %s: %v", e.Code, e.Sentinel)
}

func (e *CollectorError) Unwrap() error { return e.Sentinel }

// SentinelFor maps a wire code back to the domain sentinel the current
// application layer already knows how to render.
func SentinelFor(code ErrorCode) error {
	switch code {
	case CodeInvalidRequest:
		return sourcing.ErrInvalidAcquisition
	case CodeBudgetExceeded:
		return context.DeadlineExceeded
	case CodeSourceUnavailable:
		return sourcing.ErrAcquisitionFailed
	default:
		// An unknown code is never treated as success and never as a caller
		// error; it is a source failure.
		return sourcing.ErrAcquisitionFailed
	}
}

// WriteError renders a bounded error body with the given HTTP status.
func WriteError(w http.ResponseWriter, status int, code ErrorCode) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	// The body is a fixed, tiny shape; ignoring the error is acceptable because
	// the connection is being torn down either way.
	_ = writeJSON(w, ErrorBody{Code: code})
}

// ReadRequest decodes a request body with a hard byte cap applied before any
// decoding happens.
func ReadRequest(r *http.Request) (AcquireRequest, error) {
	limited := http.MaxBytesReader(nil, r.Body, MaxRequestBytes)
	raw, err := io.ReadAll(limited)
	if err != nil {
		return AcquireRequest{}, errors.Join(sourcing.ErrInvalidAcquisition, err)
	}
	var req AcquireRequest
	if err := decodeStrict(raw, &req); err != nil {
		return AcquireRequest{}, errors.Join(sourcing.ErrInvalidAcquisition, err)
	}
	if err := req.Validate(); err != nil {
		return AcquireRequest{}, err
	}
	return req, nil
}

// writeJSON encodes a bounded response body.
func writeJSON(w io.Writer, value any) error {
	return json.NewEncoder(w).Encode(value)
}

// decodeStrict decodes with the repository's strict decoder, rejecting unknown
// fields and duplicate keys. Unknown-field rejection is what keeps a caller
// from smuggling tenant identity (organizationId/actorId/roles/token) into a
// request that has no such field (D7, D13); sigjson only enforces it when
// DisallowUnknownFields is passed explicitly.
func decodeStrict(raw []byte, target any) error {
	// sigjson reports strict violations (unknown/duplicate fields) in a separate
	// slice with a nil error, so both must be checked or the check silently
	// does nothing.
	strictErrs, err := sigjson.UnmarshalStrict(raw, target, sigjson.DisallowDuplicateFields, sigjson.DisallowUnknownFields)
	if err != nil {
		return err
	}
	if len(strictErrs) > 0 {
		return errors.New(strictErrs[0].Error())
	}
	return nil
}
