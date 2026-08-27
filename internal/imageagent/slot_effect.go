package imageagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

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
	URL        string
	Type       string
	SourceURL  string
	Operations []string
	Width      int
	Height     int
	Metadata   map[string]string
}

type SlotGeneratedOutput struct {
	SlotID        string
	Attempt       int
	SourceAssetID string
	Assets        []GeneratedAsset
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
