package imageagent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"time"
)

// These ports belong only to the current single-source generation owner.
// Runtime callbacks resolve credentials, fetch public source bytes and build
// the pinned provider; they do not choose reservation or terminal outcomes.
type GenerationMemberLimit struct {
	Version    int64
	MonthStart time.Time
}
type GenerationProviderMetadata struct {
	RouteReference, CredentialReference, ConfigurationVersion, PromptVersion, PriceVersion string
	Points                                                                                 int64
}
type PreparedGenerationProvider struct {
	Metadata GenerationProviderMetadata
	Executor BudgetedStagedSlotExecutor
}
type GenerationExecutionDependencies struct {
	Facts              GenerationFactRepository
	Resources          GenerationResources
	Authorizer         ExecutionAuthorizer
	ReadMemberLimit    func(context.Context, string, string) (GenerationMemberLimit, error)
	ReadSourceBytes    func(context.Context, SlotExecutionInput) ([]byte, error)
	PrepareProvider    func(context.Context, func(context.Context, GenerationSuccess) error) (PreparedGenerationProvider, error)
	RevalidateProvider func(context.Context, GenerationProviderMetadata) error
	MaxSourceBytes     int
}
type GenerationExecution struct {
	dependencies GenerationExecutionDependencies
}

func NewGenerationExecution(d GenerationExecutionDependencies) (*GenerationExecution, error) {
	if d.Facts == nil || d.Resources == nil || d.Authorizer == nil || d.ReadMemberLimit == nil || d.ReadSourceBytes == nil || d.PrepareProvider == nil || d.RevalidateProvider == nil || d.MaxSourceBytes <= 0 {
		return nil, ErrValidation
	}
	return &GenerationExecution{dependencies: d}, nil
}
func (e *GenerationExecution) QuoteSlot(ctx context.Context, input SlotExecutionInput, policy BudgetPolicy) (SlotUsageQuote, error) {
	if err := e.authorize(ctx, input); err != nil {
		return SlotUsageQuote{}, err
	}
	provider, err := e.dependencies.PrepareProvider(ctx, func(context.Context, GenerationSuccess) error { return ErrProviderContractViolation })
	if err != nil {
		return SlotUsageQuote{}, err
	}
	_, quote, err := quoteGenerationProvider(ctx, provider, input, policy)
	return quote, err
}
func (e *GenerationExecution) GenerateQuotedSlot(ctx context.Context, input SlotExecutionInput, expected SlotUsageQuote) (SlotGeneratedOutput, error) {
	reject := func(err error) (SlotGeneratedOutput, error) {
		return SlotGeneratedOutput{}, &ProviderDispatchError{State: ProviderRejectedBeforeEffect, Err: err}
	}
	if err := e.authorize(ctx, input); err != nil {
		return reject(err)
	}
	id := SlotExternalEffectIdentity{RunScope: RunScope{TenantID: input.TenantID, OwnerUserID: input.UserID, RunID: input.RunID}, PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt}
	// Existing attempts are exclusively reconciled by the effect recovery owner.
	// A readback (including a prepared row) can never grant a new POST permit.
	if _, err := e.dependencies.Facts.ReadGenerationFact(ctx, id); err == nil {
		return generationUnknown(ErrRevisionConflict)
	} else if !errors.Is(err, ErrRunNotFound) {
		return generationUnknown(err)
	}
	var intent GenerationIntent
	provider, err := e.dependencies.PrepareProvider(ctx, func(observeCtx context.Context, success GenerationSuccess) error {
		finalCtx, cancel := generationFinalizationContext(observeCtx)
		defer cancel()
		if _, err := e.dependencies.Facts.RecordGenerationSuccess(finalCtx, intent, success); err != nil {
			return err
		}
		return e.finalize(finalCtx, intent)
	})
	if err != nil {
		return reject(err)
	}
	base, quote, err := quoteGenerationProvider(ctx, provider, input, BudgetPolicy{})
	if err != nil {
		return reject(err)
	}
	if !reflect.DeepEqual(quote, expected) {
		return reject(ErrRevisionConflict)
	}
	data, err := e.dependencies.ReadSourceBytes(ctx, input)
	if err != nil {
		return reject(err)
	}
	if len(data) == 0 || len(data) > e.dependencies.MaxSourceBytes {
		return reject(ErrValidation)
	}
	data = append([]byte(nil), data...)
	sum := sha256.Sum256(data)
	limit, err := e.dependencies.ReadMemberLimit(ctx, input.TenantID, input.OrganizationIdentity.MemberID)
	if err != nil {
		return reject(err)
	}
	m := provider.Metadata
	intent = GenerationIntent{Identity: id, MemberID: input.OrganizationIdentity.MemberID, CatalogHash: input.AssetCatalog.Manifest.Hash, SourceDigest: hex.EncodeToString(sum[:]), PromptVersion: m.PromptVersion, RouteReference: m.RouteReference, CredentialReference: m.CredentialReference, ConfigurationVersion: m.ConfigurationVersion, Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: m.PriceVersion, Points: m.Points, LimitVersion: limit.Version, MonthStart: limit.MonthStart}
	if _, err = NewGenerationFact(intent); err != nil {
		return reject(err)
	}
	if _, err = e.dependencies.Facts.PrepareGenerationIntent(ctx, intent); err != nil {
		return generationUnknown(err)
	}
	// Once the intent exists, even reserve ACK loss must be closed by its
	// durable no-generation proof and the commercial operation fence.
	closeBeforeDispatch := func(cause error) (SlotGeneratedOutput, error) {
		finalCtx, cancel := generationFinalizationContext(ctx)
		defer cancel()
		if _, err := e.dependencies.Facts.RecordGenerationNoEffect(finalCtx, intent); err != nil {
			return generationUnknown(err)
		}
		if err := e.finalize(finalCtx, intent); err != nil {
			return generationUnknown(err)
		}
		return reject(cause)
	}
	receipt, err := e.dependencies.Resources.ReserveImageGeneration(ctx, id)
	if err != nil {
		return closeBeforeDispatch(err)
	}
	if _, err = e.dependencies.Facts.BindGenerationReservation(ctx, intent, receipt); err != nil {
		return closeBeforeDispatch(err)
	}
	if err = e.authorize(ctx, input); err != nil {
		return closeBeforeDispatch(err)
	}
	if err = e.dependencies.RevalidateProvider(ctx, m); err != nil {
		return closeBeforeDispatch(err)
	}
	if err = ctx.Err(); err != nil {
		return closeBeforeDispatch(err)
	}
	_, won, err := e.dependencies.Facts.BeginGenerationDispatch(ctx, intent)
	if err != nil {
		return generationUnknown(err)
	}
	if !won {
		return generationUnknown(ErrRevisionConflict)
	}
	input.SourceBytes = data
	input.SourceDigest = intent.SourceDigest
	output, dispatchErr := provider.Executor.GenerateQuotedSlot(ctx, input, base)
	finalCtx, cancel := generationFinalizationContext(ctx)
	defer cancel()
	fact, readErr := e.dependencies.Facts.ReadGenerationFact(finalCtx, id)
	if readErr != nil {
		return generationUnknown(readErr)
	}
	if fact.State != GenerationSucceeded {
		if _, err := e.dependencies.Facts.MarkGenerationUnknown(finalCtx, intent); err != nil {
			return generationUnknown(err)
		}
		if dispatchErr == nil {
			dispatchErr = ErrProviderContractViolation
		}
		return generationUnknown(dispatchErr)
	}
	// Success is charged before output validation/download. Losing the output
	// never turns that irreversible effect into a refundable no-generation.
	if err := e.finalize(finalCtx, intent); err != nil {
		return generationUnknown(err)
	}
	if dispatchErr != nil {
		return generationUnknown(dispatchErr)
	}
	return output, nil
}

func (e *GenerationExecution) authorize(ctx context.Context, input SlotExecutionInput) error {
	identity := input.OrganizationIdentity
	if ValidateOrganizationExecution(identity, input.RunID) != nil || identity.MemberID == "" || identity.TenantID != input.TenantID || identity.UserID != input.UserID {
		return ErrIdentityRequired
	}
	if input.PlanRevision <= 0 || input.Attempt <= 0 || input.Slot.ID == "" || input.Slot.Role != SlotRoleMain || len(input.Slot.SourceAssetIDs) != 1 || len(input.Slot.StyleReferenceIDs) != 0 || input.TargetPlatform != "product" || input.ImagePolicyContext == nil || *input.ImagePolicyContext != (ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}) {
		return ErrValidation
	}
	return e.dependencies.Authorizer.AuthorizeExecution(ctx, identity)
}
func quoteGenerationProvider(ctx context.Context, p PreparedGenerationProvider, input SlotExecutionInput, policy BudgetPolicy) (SlotUsageQuote, SlotUsageQuote, error) {
	m := p.Metadata
	if p.Executor == nil || m.Points <= 0 || m.PriceVersion == "" || m.RouteReference == "" || m.CredentialReference == "" || m.ConfigurationVersion == "" || m.PromptVersion == "" {
		return SlotUsageQuote{}, SlotUsageQuote{}, ErrBudgetQuoteUnavailable
	}
	base, err := p.Executor.QuoteSlot(ctx, input, policy)
	if err != nil {
		return SlotUsageQuote{}, SlotUsageQuote{}, err
	}
	if ValidateSlotUsageQuote(base) != nil || len(base.Operations) != 1 || base.Maximum.Images != 1 || base.Maximum.ModelCalls != 1 {
		return SlotUsageQuote{}, SlotUsageQuote{}, ErrBudgetQuoteUnavailable
	}
	op := base.Operations[0]
	if op.Name != "render_source_white_background" || op.Provider != "grsai" || op.Model != "gpt-image-2.5" || op.MaximumOutputs != 1 {
		return SlotUsageQuote{}, SlotUsageQuote{}, ErrBudgetQuoteUnavailable
	}
	quote := base
	quote.Fingerprint = generationHash(struct {
		Quote    SlotUsageQuote
		Metadata GenerationProviderMetadata
	}{base, m})
	return base, quote, nil
}
func (e *GenerationExecution) finalize(ctx context.Context, intent GenerationIntent) error {
	receipt, err := e.dependencies.Resources.FinalizeImageGeneration(ctx, intent.Identity)
	if err != nil {
		return err
	}
	_, err = e.dependencies.Facts.BindGenerationSettlement(ctx, intent, receipt)
	return err
}
func generationUnknown(err error) (SlotGeneratedOutput, error) {
	return SlotGeneratedOutput{}, &ProviderDispatchError{State: ProviderDispatchedUnknown, Err: err}
}
func generationFinalizationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 60*time.Second)
}
