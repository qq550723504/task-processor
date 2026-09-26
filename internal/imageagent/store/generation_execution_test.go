package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/imageagent"
	resourceadapter "task-processor/internal/integration/orgresource"
)

type generationExecutionAuth struct{ denied bool }

func (a *generationExecutionAuth) AuthorizeExecution(context.Context, imageagent.ExecutionIdentity) error {
	if a.denied {
		return imageagent.ErrIdentityRequired
	}
	return nil
}

type generationExecutionProvider struct {
	observer   func(context.Context, imageagent.GenerationSuccess) error
	dispatches int
	unknown    bool
}

type lostBeginACK struct {
	imageagent.GenerationFactRepository
}

func (r lostBeginACK) BeginGenerationDispatch(ctx context.Context, intent imageagent.GenerationIntent) (imageagent.GenerationFact, bool, error) {
	_, won, err := r.GenerationFactRepository.BeginGenerationDispatch(ctx, intent)
	if err != nil || !won {
		return imageagent.GenerationFact{}, won, err
	}
	return imageagent.GenerationFact{}, false, errors.New("commit ACK lost")
}

type afterGenerationReserve struct {
	imageagent.GenerationResources
	after   func()
	loseACK bool
}

func (r afterGenerationReserve) ReserveImageGeneration(ctx context.Context, id imageagent.SlotExternalEffectIdentity) (imageagent.GenerationReservationReceipt, error) {
	receipt, err := r.GenerationResources.ReserveImageGeneration(ctx, id)
	if err == nil && r.after != nil {
		r.after()
	}
	if err == nil && r.loseACK {
		return imageagent.GenerationReservationReceipt{}, errors.New("reserve ACK lost")
	}
	return receipt, err
}

func (p *generationExecutionProvider) QuoteSlot(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	maximum := imageagent.UsageVector{Images: 1, ModelCalls: 1, AgentSteps: 1}
	return imageagent.SlotUsageQuote{Maximum: maximum, Fingerprint: "provider-quote", Operations: []imageagent.SlotUsageOperation{{Name: "render_source_white_background", Provider: "grsai", Model: "gpt-image-2.5", Fingerprint: "provider-op", Maximum: maximum, MaximumOutputs: 1}}}, nil
}
func (p *generationExecutionProvider) GenerateQuotedSlot(ctx context.Context, input imageagent.SlotExecutionInput, quote imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	p.dispatches++
	if quote.Fingerprint != "provider-quote" || string(input.SourceBytes) != "exact-source-bytes" {
		return imageagent.SlotGeneratedOutput{}, imageagent.ErrValidation
	}
	if p.unknown {
		return imageagent.SlotGeneratedOutput{}, errors.New("response lost")
	}
	if err := p.observer(ctx, imageagent.GenerationSuccess{ResponseID: "response-1", ResultDigest: strings.Repeat("c", 64), ResultUnavailable: "invalid_result"}); err != nil {
		return imageagent.SlotGeneratedOutput{}, err
	}
	return imageagent.SlotGeneratedOutput{}, errors.New("generated output download failed")
}
func (*generationExecutionProvider) GenerateSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	panic("uncapped generation")
}
func (*generationExecutionProvider) BuildSlotResult(context.Context, imageagent.SlotExecutionInput, imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	panic("not used")
}

func TestGenerationExecutionPreservesProviderOutcomeAndNeverRedispatches(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		name := "success_then_download_failure"
		if unknown {
			name = "response_lost"
		}
		t.Run(name, func(t *testing.T) {
			owner, commercial, limits, intent := generationPointDatabases(t, true)
			ctx := context.Background()
			resources, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, owner, generationPointTestAuth{})
			require.NoError(t, err)
			catalog, err := owner.GetAssetCatalog(ctx, intent.Identity.RunScope)
			require.NoError(t, err)
			input := imageagent.SlotExecutionInput{RunID: intent.Identity.RunID, TenantID: intent.Identity.TenantID, UserID: intent.Identity.OwnerUserID, PlanRevision: 1, Attempt: 1, TargetPlatform: "product", ImagePolicyContext: &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}}, AssetCatalog: catalog,
				OrganizationIdentity: imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, RunID: intent.Identity.RunID, TenantID: intent.Identity.TenantID, UserID: intent.Identity.OwnerUserID, MemberID: intent.MemberID, BusinessTaskID: "task-" + intent.Identity.RunID}}
			provider := &generationExecutionProvider{unknown: unknown}
			auth := &generationExecutionAuth{}
			executor, err := imageagent.NewGenerationExecution(imageagent.GenerationExecutionDependencies{Facts: owner, Resources: resources, Authorizer: auth, MaxSourceBytes: 1024,
				ReadMemberLimit: func(context.Context, string, string) (imageagent.GenerationMemberLimit, error) {
					return imageagent.GenerationMemberLimit{Version: 1, MonthStart: intent.MonthStart}, nil
				},
				ReadSourceBytes: func(context.Context, imageagent.SlotExecutionInput) ([]byte, error) {
					return []byte("exact-source-bytes"), nil
				},
				PrepareProvider: func(_ context.Context, observer func(context.Context, imageagent.GenerationSuccess) error) (imageagent.PreparedGenerationProvider, error) {
					provider.observer = observer
					return imageagent.PreparedGenerationProvider{Executor: provider, Metadata: imageagent.GenerationProviderMetadata{RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "config-1", PromptVersion: "prompt-1", PriceVersion: "price-1", Points: 12}}, nil
				},
				RevalidateProvider: func(context.Context, imageagent.GenerationProviderMetadata) error { return nil },
			})
			require.NoError(t, err)
			quote, err := executor.QuoteSlot(ctx, input, imageagent.BudgetPolicy{})
			require.NoError(t, err)
			_, err = executor.GenerateQuotedSlot(ctx, input, quote)
			require.Error(t, err)
			require.Equal(t, 1, provider.dispatches)
			fact, err := owner.ReadGenerationFact(ctx, intent.Identity)
			require.NoError(t, err)
			sum := sha256.Sum256([]byte("exact-source-bytes"))
			require.Equal(t, hex.EncodeToString(sum[:]), fact.Intent.SourceDigest)
			read, err := limits.ReadMonthlyLimit(ctx, intent.Identity.TenantID, intent.MemberID)
			require.NoError(t, err)
			if unknown {
				require.Equal(t, imageagent.GenerationUnknown, fact.State)
				require.EqualValues(t, 12, read.Reserved)
				require.Zero(t, read.Consumed)
			} else {
				require.Equal(t, imageagent.GenerationSucceeded, fact.State)
				require.Equal(t, "committed", fact.Settlement.State)
				require.Zero(t, read.Reserved)
				require.EqualValues(t, 12, read.Consumed)
			}
			_, err = executor.GenerateQuotedSlot(ctx, input, quote)
			require.Error(t, err)
			require.Equal(t, 1, provider.dispatches)
		})
	}
}

func TestGenerationExecutionPreDispatchFencesAndLostACK(t *testing.T) {
	for _, mode := range []string{"begin_ack_lost", "reserve_ack_lost", "revoked_after_reserve", "price_drift", "route_drift"} {
		t.Run(mode, func(t *testing.T) {
			owner, commercial, limits, intent := generationPointDatabases(t, true)
			ctx := context.Background()
			resources, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, owner, generationPointTestAuth{})
			require.NoError(t, err)
			catalog, err := owner.GetAssetCatalog(ctx, intent.Identity.RunScope)
			require.NoError(t, err)
			input := imageagent.SlotExecutionInput{RunID: intent.Identity.RunID, TenantID: intent.Identity.TenantID, UserID: intent.Identity.OwnerUserID, PlanRevision: 1, Attempt: 1, TargetPlatform: "product", ImagePolicyContext: &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}}, AssetCatalog: catalog,
				OrganizationIdentity: imageagent.ExecutionIdentity{ScopeProtocol: imageagent.OrganizationScopeProtocol, RunID: intent.Identity.RunID, TenantID: intent.Identity.TenantID, UserID: intent.Identity.OwnerUserID, MemberID: intent.MemberID, BusinessTaskID: "task-" + intent.Identity.RunID}}
			provider := &generationExecutionProvider{}
			auth := &generationExecutionAuth{}
			priceVersion := "price-1"
			var facts imageagent.GenerationFactRepository = owner
			if mode == "begin_ack_lost" {
				facts = lostBeginACK{owner}
			}
			var resourcePort imageagent.GenerationResources = resources
			if mode == "revoked_after_reserve" {
				resourcePort = afterGenerationReserve{GenerationResources: resources, after: func() { auth.denied = true }}
			}
			if mode == "reserve_ack_lost" {
				resourcePort = afterGenerationReserve{GenerationResources: resources, loseACK: true}
			}
			executor, err := imageagent.NewGenerationExecution(imageagent.GenerationExecutionDependencies{Facts: facts, Resources: resourcePort, Authorizer: auth, MaxSourceBytes: 1024,
				ReadMemberLimit: func(context.Context, string, string) (imageagent.GenerationMemberLimit, error) {
					return imageagent.GenerationMemberLimit{Version: 1, MonthStart: intent.MonthStart}, nil
				},
				ReadSourceBytes: func(context.Context, imageagent.SlotExecutionInput) ([]byte, error) {
					return []byte("exact-source-bytes"), nil
				},
				PrepareProvider: func(_ context.Context, observer func(context.Context, imageagent.GenerationSuccess) error) (imageagent.PreparedGenerationProvider, error) {
					provider.observer = observer
					return imageagent.PreparedGenerationProvider{Executor: provider, Metadata: imageagent.GenerationProviderMetadata{RouteReference: "route-1", CredentialReference: "credential-1", ConfigurationVersion: "config-1", PromptVersion: "prompt-1", PriceVersion: priceVersion, Points: 12}}, nil
				},
				RevalidateProvider: func(context.Context, imageagent.GenerationProviderMetadata) error {
					if mode == "route_drift" {
						return imageagent.ErrRevisionConflict
					}
					return nil
				},
			})
			require.NoError(t, err)
			quote, err := executor.QuoteSlot(ctx, input, imageagent.BudgetPolicy{})
			require.NoError(t, err)
			if mode == "price_drift" {
				priceVersion = "price-2"
			}
			_, err = executor.GenerateQuotedSlot(ctx, input, quote)
			require.Error(t, err)
			require.Zero(t, provider.dispatches)
			read, err := limits.ReadMonthlyLimit(ctx, intent.Identity.TenantID, intent.MemberID)
			require.NoError(t, err)
			require.Zero(t, read.Consumed)
			fact, factErr := owner.ReadGenerationFact(ctx, intent.Identity)
			switch mode {
			case "price_drift":
				require.ErrorIs(t, factErr, imageagent.ErrRunNotFound)
				require.Zero(t, read.Reserved)
			case "begin_ack_lost":
				require.NoError(t, factErr)
				require.Equal(t, imageagent.GenerationDispatchStarted, fact.State)
				require.EqualValues(t, 12, read.Reserved)
				_, err = executor.GenerateQuotedSlot(ctx, input, quote)
				require.Error(t, err)
				require.Zero(t, provider.dispatches)
			default:
				require.NoError(t, factErr)
				require.Equal(t, imageagent.GenerationNoEffect, fact.State)
				require.Equal(t, "released", fact.Settlement.State)
				require.Zero(t, read.Reserved)
			}
		})
	}
}
