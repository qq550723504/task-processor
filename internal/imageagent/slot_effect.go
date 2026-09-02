package imageagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

var (
	ErrBudgetQuoteUnavailable    = errors.New("image agent budget quote unavailable")
	ErrProviderContractViolation = errors.New("image agent provider contract violation")
)

type ProviderDispatchState string

const (
	ProviderNotDispatched        ProviderDispatchState = "not_dispatched"
	ProviderRejectedBeforeEffect ProviderDispatchState = "rejected_before_effect"
	ProviderDispatchedUnknown    ProviderDispatchState = "dispatched_unknown"
)

type ProviderDispatchError struct {
	State              ProviderDispatchState
	ProviderRequestIDs []string
	Err                error
}

func (e *ProviderDispatchError) Error() string {
	if e == nil || e.Err == nil {
		return "image agent provider dispatch failed"
	}
	return e.Err.Error()
}

func (e *ProviderDispatchError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func ProviderDispatchStateOf(err error) ProviderDispatchState {
	var dispatch *ProviderDispatchError
	if errors.As(err, &dispatch) {
		return dispatch.State
	}
	return ProviderDispatchedUnknown
}

type SlotUsageOperation struct {
	Name           string
	Provider       string
	Model          string
	PricingVersion string
	Fingerprint    string
	Maximum        UsageVector
	MaximumOutputs int64
}

type SlotUsageQuote struct {
	Maximum        UsageVector
	Operations     []SlotUsageOperation
	PricingVersion string
	Fingerprint    string
}

type UsageCostBasis string

const (
	UsageCostActual             UsageCostBasis = "actual"
	UsageCostReservedUpperBound UsageCostBasis = "reserved_upper_bound"
	UsageCostUnavailable        UsageCostBasis = "unavailable"
)

type SlotUsageReceipt struct {
	Actual             UsageVector
	ProviderRequestIDs []string
	CostBasis          UsageCostBasis
}

func ValidateSlotUsageQuote(quote SlotUsageQuote) error {
	if quote.Fingerprint == "" || len(quote.Operations) == 0 {
		return fmt.Errorf("%w: quote identity is incomplete", ErrValidation)
	}
	if err := validateUsageVector(quote.Maximum); err != nil {
		return err
	}
	for _, operation := range quote.Operations {
		if operation.Name == "" || operation.Fingerprint == "" || operation.MaximumOutputs <= 0 {
			return fmt.Errorf("%w: quoted operation is incomplete", ErrValidation)
		}
		if err := validateUsageVector(operation.Maximum); err != nil {
			return err
		}
	}
	return nil
}

type SlotExternalEffectPhase string

const (
	SlotExternalEffectProviderStarted     SlotExternalEffectPhase = "provider_started"
	SlotExternalEffectGeneratedComplete   SlotExternalEffectPhase = "generated_complete"
	SlotExternalEffectPublicationComplete SlotExternalEffectPhase = "publication_complete"
)

type SlotExternalEffectIdentity struct {
	RunScope
	PlanRevision int64
	SlotID       string
	Attempt      int
}

type GeneratedAsset struct {
	URL               string
	Bytes             []byte `json:"-"`
	ContentType       string `json:",omitempty"`
	Type              string
	SourceURL         string
	Operations        []string
	Width             int
	Height            int
	Metadata          map[string]string
	ProviderReceiptID string `json:",omitempty"`
}

type SlotGeneratedOutput struct {
	SlotID        string
	Attempt       int
	SourceAssetID string
	Assets        []GeneratedAsset
	UsageReceipt  SlotUsageReceipt
}

type SlotExternalEffectReservation struct {
	Identity         SlotExternalEffectIdentity
	IdempotencyKey   string
	InputFingerprint string
}

type SlotExternalEffectAttempt struct {
	Identity         SlotExternalEffectIdentity
	IdempotencyKey   string
	InputFingerprint string
	Phase            SlotExternalEffectPhase
	Generated        SlotGeneratedOutput
	Published        SlotExecutionResult
}

type SlotExternalEffectRepository interface {
	ReserveSlotExternalEffect(context.Context, SlotExternalEffectReservation) (SlotExternalEffectAttempt, bool, error)
	StoreSlotGeneratedOutput(context.Context, SlotExternalEffectReservation, SlotGeneratedOutput) (SlotExternalEffectAttempt, error)
	CompleteSlotPublication(context.Context, SlotExternalEffectReservation, SlotExecutionResult) (SlotExternalEffectAttempt, error)
	GetSlotExternalEffect(context.Context, SlotExternalEffectIdentity) (SlotExternalEffectAttempt, error)
}

type RecoverableSlotExecutor interface {
	GenerateSlot(context.Context, SlotExecutionInput) (SlotGeneratedOutput, error)
	PublishSlot(context.Context, SlotExecutionInput, SlotGeneratedOutput) (SlotExecutionResult, error)
}

func SlotExecutionFingerprint(input SlotExecutionInput) string {
	encoded, _ := json.Marshal(input)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
