package productimage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

var ErrCapabilityUsageQuoteUnavailable = errors.New("product image capability usage quote unavailable")

type CapabilityDispatchState string

const (
	CapabilityNotDispatched        CapabilityDispatchState = "not_dispatched"
	CapabilityRejectedBeforeEffect CapabilityDispatchState = "rejected_before_effect"
	CapabilityDispatchedUnknown    CapabilityDispatchState = "dispatched_unknown"
)

type CapabilityDispatchError struct {
	State              CapabilityDispatchState
	ProviderRequestIDs []string
	Err                error
}

func (e *CapabilityDispatchError) Error() string {
	if e == nil || e.Err == nil {
		return "product image capability dispatch failed"
	}
	return e.Err.Error()
}

func (e *CapabilityDispatchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func CapabilityDispatchStateOf(err error) CapabilityDispatchState {
	var dispatch *CapabilityDispatchError
	if errors.As(err, &dispatch) {
		return dispatch.State
	}
	return CapabilityDispatchedUnknown
}

func capabilityQuoteFingerprint(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func localCapabilityUsageQuote(request CapabilityUsageQuoteRequest, maximumOutputs int64) CapabilityUsageQuote {
	quote := CapabilityUsageQuote{
		Operation: request.Operation, Provider: "local", PricingVersion: "local-v1",
		MaximumOutputs: maximumOutputs, CostUpperBoundKnown: true,
	}
	quote.Fingerprint = capabilityQuoteFingerprint(struct {
		Request CapabilityUsageQuoteRequest
		Quote   CapabilityUsageQuote
	}{request, quote})
	return quote
}
