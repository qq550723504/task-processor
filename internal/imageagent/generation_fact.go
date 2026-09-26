package imageagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/url"
	"strings"
	"time"
)

// GenerationIntent belongs to the existing V3 attempt, not to a second job.
// No mutable URL, secret, balance or browser-selected month is stored here.
type GenerationIntent struct {
	Identity             SlotExternalEffectIdentity
	MemberID             string
	CatalogHash          string
	SourceDigest         string
	PromptVersion        string
	RouteReference       string
	CredentialReference  string
	ConfigurationVersion string
	Provider             string
	Model                string
	Protocol             string
	Resolution           string
	Quality              string
	PriceVersion         string
	Points               int64
	LimitVersion         int64
	MonthStart           time.Time
}

type GenerationReservationReceipt struct {
	IntentID       string
	Fingerprint    string
	OrganizationID string
	MemberID       string
	OperationID    string
	ReservationID  string
	ResourceType   string
	Points         int64
	PriceVersion   string
	LimitVersion   int64
	MonthStart     time.Time
}

type GenerationSuccess struct {
	ResponseID   string
	RequestID    string
	ResultDigest string
	// Worker-only locator. Never expose GenerationSuccess in a public DTO.
	ResultURL         string
	ResultUnavailable string
	UsageKnown        bool
	InputTokens       int64
	OutputTokens      int64
	TotalTokens       int64
}

type GenerationState string

const (
	GenerationPrepared        GenerationState = "prepared"
	GenerationDispatchStarted GenerationState = "dispatch_started"
	GenerationUnknown         GenerationState = "unknown"
	GenerationNoEffect        GenerationState = "no_generation"
	GenerationSucceeded       GenerationState = "provider_succeeded"
)

type GenerationFact struct {
	Version     int
	Intent      GenerationIntent
	IntentID    string
	Fingerprint string
	State       GenerationState
	Reservation GenerationReservationReceipt
	Success     GenerationSuccess
	Settlement  GenerationSettlementReceipt
}

type GenerationSettlementReceipt struct {
	IntentID, Fingerprint, OperationID, ReservationID, State, ProofDigest string
	Points                                                                int64
}

type GenerationFactRepository interface {
	PrepareGenerationIntent(context.Context, GenerationIntent) (GenerationFact, error)
	ReadGenerationFact(context.Context, SlotExternalEffectIdentity) (GenerationFact, error)
	BindGenerationReservation(context.Context, GenerationIntent, GenerationReservationReceipt) (GenerationFact, error)
	BeginGenerationDispatch(context.Context, GenerationIntent) (GenerationFact, bool, error)
	RecordGenerationNoEffect(context.Context, GenerationIntent) (GenerationFact, error)
	MarkGenerationUnknown(context.Context, GenerationIntent) (GenerationFact, error)
	RecordGenerationSuccess(context.Context, GenerationIntent, GenerationSuccess) (GenerationFact, error)
	BindGenerationSettlement(context.Context, GenerationIntent, GenerationSettlementReceipt) (GenerationFact, error)
}

type GenerationResources interface {
	ReserveImageGeneration(context.Context, SlotExternalEffectIdentity) (GenerationReservationReceipt, error)
	FinalizeImageGeneration(context.Context, SlotExternalEffectIdentity) (GenerationSettlementReceipt, error)
}

func NewGenerationFact(intent GenerationIntent) (GenerationFact, error) {
	for _, value := range []string{intent.Identity.TenantID, intent.Identity.OwnerUserID, intent.Identity.RunID, intent.Identity.SlotID, intent.MemberID, intent.PromptVersion, intent.RouteReference, intent.CredentialReference, intent.ConfigurationVersion, intent.PriceVersion} {
		if strings.TrimSpace(value) != value || value == "" || len(value) > 192 {
			return GenerationFact{}, ErrValidation
		}
	}
	if intent.Identity.PlanRevision <= 0 || intent.Identity.Attempt <= 0 || intent.Points <= 0 || intent.LimitVersion <= 0 ||
		!validGenerationCatalogHash(intent.CatalogHash) || !generationDigest(intent.SourceDigest) ||
		intent.Provider != "grsai" || intent.Model != "gpt-image-2.5" || intent.Protocol != "grsai-json-sync-v1" || intent.Resolution != "1024x1024" || intent.Quality != "auto" ||
		intent.MonthStart.IsZero() || intent.MonthStart != time.Date(intent.MonthStart.UTC().Year(), intent.MonthStart.UTC().Month(), 1, 0, 0, 0, 0, time.UTC) {
		return GenerationFact{}, ErrValidation
	}
	return GenerationFact{Version: 1, Intent: intent, IntentID: generationHash(intent.Identity), Fingerprint: generationHash(intent), State: GenerationPrepared}, nil
}
func (f GenerationFact) BindReservation(receipt GenerationReservationReceipt) (GenerationFact, error) {
	if err := f.Validate(); err != nil {
		return f, err
	}
	if !f.matchesReceipt(receipt) {
		return f, ErrRevisionConflict
	}
	if f.Reservation.ReservationID != "" {
		if f.Reservation != receipt {
			return f, ErrRevisionConflict
		}
		return f, nil
	}
	if f.State != GenerationPrepared {
		return f, ErrRevisionConflict
	}
	f.Reservation = receipt
	return f, nil
}
func (f GenerationFact) BeginDispatch() (GenerationFact, bool, error) {
	if err := f.Validate(); err != nil {
		return f, false, err
	}
	if f.State != GenerationPrepared {
		return f, false, nil
	}
	if f.Reservation.ReservationID == "" {
		return f, false, ErrRevisionConflict
	}
	f.State = GenerationDispatchStarted
	return f, true, nil
}
func (f GenerationFact) RecordNoGeneration() (GenerationFact, error) {
	if err := f.Validate(); err != nil {
		return f, err
	}
	if f.State != GenerationPrepared && f.State != GenerationNoEffect {
		return f, ErrRevisionConflict
	}
	f.State = GenerationNoEffect
	return f, nil
}
func (f GenerationFact) MarkUnknown() (GenerationFact, error) {
	if err := f.Validate(); err != nil {
		return f, err
	}
	if f.State == GenerationSucceeded {
		return f, nil
	}
	if f.State != GenerationDispatchStarted && f.State != GenerationUnknown {
		return f, ErrRevisionConflict
	}
	f.State = GenerationUnknown
	return f, nil
}
func (f GenerationFact) RecordSuccess(success GenerationSuccess) (GenerationFact, error) {
	if err := f.Validate(); err != nil {
		return f, err
	}
	if !validGenerationSuccess(success) {
		return f, ErrValidation
	}
	if f.State == GenerationSucceeded {
		if f.Success != success {
			return f, ErrRevisionConflict
		}
		return f, nil
	}
	if f.State != GenerationDispatchStarted && f.State != GenerationUnknown {
		return f, ErrRevisionConflict
	}
	f.State = GenerationSucceeded
	f.Success = success
	return f, nil
}

func (f GenerationFact) Validate() error {
	base, err := NewGenerationFact(f.Intent)
	if err != nil || f.Version != 1 || f.IntentID != base.IntentID || f.Fingerprint != base.Fingerprint {
		return ErrValidation
	}
	if f.Reservation != (GenerationReservationReceipt{}) && !f.matchesReceipt(f.Reservation) {
		return ErrValidation
	}
	switch f.State {
	case GenerationPrepared, GenerationNoEffect:
		if f.Success != (GenerationSuccess{}) {
			return ErrValidation
		}
	case GenerationDispatchStarted, GenerationUnknown:
		if f.Reservation.ReservationID == "" || f.Success != (GenerationSuccess{}) {
			return ErrValidation
		}
	case GenerationSucceeded:
		if f.Reservation.ReservationID == "" || !validGenerationSuccess(f.Success) {
			return ErrValidation
		}
	default:
		return ErrValidation
	}
	if f.Settlement != (GenerationSettlementReceipt{}) && !f.matchesSettlement(f.Settlement) {
		return ErrValidation
	}
	return nil
}

func (f GenerationFact) BindSettlement(receipt GenerationSettlementReceipt) (GenerationFact, error) {
	if err := f.Validate(); err != nil {
		return f, err
	}
	if !f.matchesSettlement(receipt) {
		return f, ErrRevisionConflict
	}
	if f.Settlement != (GenerationSettlementReceipt{}) && f.Settlement != receipt {
		return f, ErrRevisionConflict
	}
	f.Settlement = receipt
	return f, nil
}
func (f GenerationFact) TerminalProofDigest() string {
	if f.State != GenerationSucceeded && f.State != GenerationNoEffect {
		return ""
	}
	return generationHash(struct {
		Fingerprint string
		State       GenerationState
		Success     GenerationSuccess
	}{f.Fingerprint, f.State, f.Success})
}
func (f GenerationFact) matchesSettlement(r GenerationSettlementReceipt) bool {
	if r.IntentID != f.IntentID || r.Fingerprint != f.Fingerprint || r.OperationID != "image-finalize:"+f.IntentID || r.Points != f.Intent.Points || r.ProofDigest == "" || r.ProofDigest != f.TerminalProofDigest() {
		return false
	}
	if f.State == GenerationSucceeded {
		return r.State == "committed" && r.ReservationID == f.Reservation.ReservationID
	}
	if f.State != GenerationNoEffect {
		return false
	}
	if f.Reservation.ReservationID != "" {
		return r.State == "released" && r.ReservationID == f.Reservation.ReservationID
	}
	// A reserve may have committed after cancellation but before its binding.
	return (r.State == "no_reservation" && r.ReservationID == "") || (r.State == "released" && r.ReservationID != "" && strings.TrimSpace(r.ReservationID) == r.ReservationID && len(r.ReservationID) <= 128)
}

func (f GenerationFact) matchesReceipt(r GenerationReservationReceipt) bool {
	return r.IntentID == f.IntentID && r.Fingerprint == f.Fingerprint && r.OrganizationID == f.Intent.Identity.TenantID && r.MemberID == f.Intent.MemberID &&
		r.OperationID == "image-reserve:"+f.IntentID && r.ReservationID != "" && strings.TrimSpace(r.ReservationID) == r.ReservationID && len(r.ReservationID) <= 128 &&
		r.ResourceType == "ai_point" && r.Points == f.Intent.Points && r.PriceVersion == f.Intent.PriceVersion && r.LimitVersion == f.Intent.LimitVersion && r.MonthStart == f.Intent.MonthStart
}

func validGenerationSuccess(s GenerationSuccess) bool {
	if s.ResultURL != "" {
		if s.ResultUnavailable != "" || !validGenerationResultURL(s.ResultURL) {
			return false
		}
	} else if s.ResultUnavailable != "invalid_result" {
		return false
	}
	if s.ResponseID == "" || strings.TrimSpace(s.ResponseID) != s.ResponseID || len(s.ResponseID) > 192 || len(s.RequestID) > 192 || strings.TrimSpace(s.RequestID) != s.RequestID || !generationDigest(s.ResultDigest) {
		return false
	}
	if !s.UsageKnown {
		return s.InputTokens == 0 && s.OutputTokens == 0 && s.TotalTokens == 0
	}
	return s.InputTokens >= 0 && s.OutputTokens >= 0 && s.TotalTokens > 0 && s.InputTokens <= s.TotalTokens && s.OutputTokens == s.TotalTokens-s.InputTokens
}

// Bound the original signed URL without rewriting its query. All actual GETs
// additionally use the public-image DNS/dial and download/decoding limits.
func validGenerationResultURL(raw string) bool {
	if len(raw) == 0 || len(raw) > 4096 || strings.ContainsAny(raw, "\r\n\x00") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.Fragment != "" || u.Scheme != "https" {
		return false
	}
	validated, err := ValidateSafeImageURL(raw)
	return err == nil && validated == raw
}

func (s GenerationSuccess) WithResultLocator(raw, unavailable string) GenerationSuccess {
	s.ResultURL, s.ResultUnavailable = "", "invalid_result"
	if unavailable == "" && validGenerationResultURL(raw) {
		s.ResultURL, s.ResultUnavailable = raw, ""
	}
	return s
}

func generationDigest(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && strings.ToLower(value) == value
}
func validGenerationCatalogHash(value string) bool {
	for _, prefix := range []string{"catalog-v1:", "catalog-v2:"} {
		if strings.HasPrefix(value, prefix) {
			return generationDigest(strings.TrimPrefix(value, prefix))
		}
	}
	return false
}
func generationHash(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
