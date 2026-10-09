package imageagent_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/authidentity"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

type setConfigurationFixture struct {
	template   agentconfig.SetTemplate
	snapshot   agentconfig.ImageConfigurationSnapshot
	receipt    *agentconfig.ImageRunAdmissionReceipt
	enabled    bool
	admitACK   error
	admissions int
}

func (c *setConfigurationFixture) PrepareImageConfiguration(_ context.Context, input agentconfig.ImageStartCommand) (agentconfig.ImageConfigurationSnapshot, error) {
	if !c.enabled {
		return agentconfig.ImageConfigurationSnapshot{}, agentconfig.ErrNotEnabled
	}
	c.snapshot = agentconfig.ImageConfigurationSnapshot{ID: "91d39d8d-3819-4a7c-ae7f-04ce91951f9a", Digest: strings.Repeat("a", 64), Scope: input.Scope, MemberID: input.MemberID, RequestKey: input.RequestKey, ContextID: input.ContextID, RunID: input.RunID, TargetPlatform: input.TargetPlatform, SourceDigest: input.SourceDigest, InputDigest: input.InputDigest, Parameters: c.template, ParametersDigest: strings.Repeat("b", 64), Epoch: "1", AgentVersion: agentconfig.ImageAgentVersion, HardLimits: input.HardLimits}
	return c.snapshot, nil
}
func (c *setConfigurationFixture) LoadImageConfiguration(context.Context, agent.Scope, agent.ConfigurationSnapshotRef) (agentconfig.ImageConfigurationSnapshot, error) {
	return c.snapshot, nil
}
func (c *setConfigurationFixture) ReadImageRunAdmission(context.Context, agent.Scope, agent.ConfigurationSnapshotRef) (agentconfig.ImageRunAdmissionReceipt, error) {
	if c.receipt == nil {
		return agentconfig.ImageRunAdmissionReceipt{}, agentconfig.ErrNotFound
	}
	return *c.receipt, nil
}
func (c *setConfigurationFixture) AdmitImageRun(_ context.Context, command agentconfig.ImageRunAdmissionCommand, _ agentconfig.ImageRunLimits) (agentconfig.ImageRunAdmissionReceipt, error) {
	if !c.enabled {
		return agentconfig.ImageRunAdmissionReceipt{}, agentconfig.ErrNotEnabled
	}
	c.admissions++
	now := time.Now().UTC().Truncate(time.Microsecond)
	receipt := agentconfig.ImageRunAdmissionReceipt{ID: "6be5e768-31c1-434e-a6c0-22a508fb3a35", Command: command, AdmittedAt: now, Deadline: now.Add(time.Duration(command.Limits.ElapsedSeconds) * time.Second)}
	raw, _ := json.Marshal(receipt)
	sum := sha256.Sum256(raw)
	receipt.Digest = hex.EncodeToString(sum[:])
	c.receipt = &receipt
	return receipt, c.admitACK
}

type setContextFixture struct {
	preparation     imageagent.ImageSetPreparation
	revalidationErr error
}

func (c *setContextFixture) ResolveImageSet(context.Context, imageagent.ExecutionIdentity, imageagent.PrepareImageSetInput) (imageagent.ImageSetPreparation, error) {
	return c.preparation, nil
}
func (c *setContextFixture) RevalidateImageSet(context.Context, imageagent.ExecutionIdentity, imageagent.RunProjection) error {
	return c.revalidationErr
}

type setQuoteFixture struct {
	quote imageagent.ImageGenerationQuote
}

func (q *setQuoteFixture) ReadImageGenerationQuote(context.Context, imageagent.ExecutionIdentity) (imageagent.ImageGenerationQuote, error) {
	return q.quote, nil
}

func imageSetServiceFixture(t *testing.T) (*imageagent.Service, imageagent.Repository, *recordingWorkflowClient, *setConfigurationFixture, *setContextFixture, *setQuoteFixture, context.Context, imageagent.PrepareImageSetInput) {
	t.Helper()
	config := &setConfigurationFixture{enabled: true, template: agentconfig.SetTemplate{Schema: agentconfig.ImageConfigurationSchema, Mode: "standard", ShareOriginals: true, Background: "white", Language: "zh", Carousel: []agentconfig.ContentTask{{ID: "identity", Purpose: "product_identity"}}, Detail: []agentconfig.ContentTask{{ID: "overview", Purpose: "product_overview"}}}}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{ProductContext: imageagent.ProductContextRef{ProductID: "product", Title: "Test product", SourceSnapshotVersion: 1}, Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://images.example.org/source.png", Width: 1024, Height: 1024}}})
	require.NoError(t, err)
	contexts := &setContextFixture{preparation: imageagent.ImageSetPreparation{Source: imageagent.ImageSourceBinding{ProductID: "product", OperationID: "operation", OriginalPublicationID: "publication", OriginalVersion: 1, EffectiveVersion: 1, CatalogHash: catalog.Manifest.Hash}, Target: imageagent.ImageTarget{Platform: "product"}, Catalog: catalog, Observations: []imageagent.ImageSourceObservation{{AssetID: "source-1", SHA256: strings.Repeat("c", 64), Bytes: 10, Width: 1024, Height: 1024, MediaType: "image/png"}}}}
	quotes := &setQuoteFixture{quote: imageagent.ImageGenerationQuote{Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price", Points: 12, RouteReference: "route", CredentialReference: "credential", ConfigurationVersion: "config"}}
	repository := store.NewMemoryRepository()
	workflows := &recordingWorkflowClient{}
	service, err := imageagent.NewService(repository, workflows, staticCatalogResolver{catalog: catalog}, imageagent.WithOrganizationScope(), imageagent.WithImageSetDependencies(imageagent.ImageSetDependencies{Configuration: config, Contexts: contexts, Quotes: quotes, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 10000, ElapsedSeconds: 3600}}))
	require.NoError(t, err)
	ctx := authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "member"})
	input := imageagent.PrepareImageSetInput{RequestID: "c5fd7c43-138c-43dc-8dfb-6f9be74a5e9e", ContextID: "operation", Target: imageagent.ImageTargetSelection{Platform: "product"}, SharedOriginalIDs: []string{"source-1"}}
	return service, repository, workflows, config, contexts, quotes, ctx, input
}

func confirmSetInput(prepared imageagent.PreparedImageSet) imageagent.ConfirmImagePlanInput {
	return imageagent.ConfirmImagePlanInput{RunID: prepared.Projection.Run.ID, ActionID: "b912e7d4-df80-44a5-8510-3cdf91d5b8dd", ExpectedRevision: 1, PlanDigest: prepared.PlanDigest, QuoteDigest: prepared.QuoteDigest}
}

func TestPrepareFullImageSetSharesOnlyOriginalsAndRequiresConfirmation(t *testing.T) {
	s, _, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 2, prepared.Images)
	require.EqualValues(t, 24, prepared.Points)
	require.Equal(t, imageagent.RunStatusAwaitingPlanApproval, prepared.Projection.Run.Status)
	require.Empty(t, workflows.starts)
	require.Zero(t, config.admissions)
	require.Equal(t, prepared.Projection.Plan.Slots[0].Recipe.References, prepared.Projection.Plan.Slots[1].Recipe.References)
	require.NotEqual(t, prepared.Projection.Plan.Slots[0].ID, prepared.Projection.Plan.Slots[1].ID)
	replay, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.Equal(t, prepared, replay)
	input.SharedOriginalIDs = []string{"source-2"}
	_, err = s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
}

func TestConfirmImageSetUsesOriginalReceiptAfterAdmissionOrStartACKLoss(t *testing.T) {
	s, repo, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	config.admitACK = errors.New("admission ACK lost")
	workflows.startErr = errors.New("Temporal start ACK lost")
	confirmed, err := s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
	require.ErrorContains(t, err, "start ACK lost")
	require.Equal(t, imageagent.RunStatusExecuting, confirmed.Run.Status)
	require.Equal(t, 1, config.admissions)
	original := *confirmed.Run.ImageAdmission
	config.enabled = false
	workflows.startErr = nil
	replayed, err := s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
	require.NoError(t, err)
	require.Equal(t, original, *replayed.Run.ImageAdmission)
	require.Equal(t, 1, config.admissions)
	require.Len(t, workflows.starts, 2)
	require.Equal(t, workflows.starts[0], workflows.starts[1])
	stored, err := repo.GetProjection(ctx, imageagent.ScopeForRun(confirmed.Run))
	require.NoError(t, err)
	require.Equal(t, &original, stored.Run.ImageAdmission)
}

func TestConfirmImageSetRejectsChangedPriceSourceOrDisabledEnterprise(t *testing.T) {
	for _, kind := range []string{"price", "source", "disabled", "member"} {
		t.Run(kind, func(t *testing.T) {
			s, _, workflows, config, contexts, quotes, ctx, input := imageSetServiceFixture(t)
			prepared, err := s.PrepareImageSet(ctx, input)
			require.NoError(t, err)
			switch kind {
			case "price":
				quotes.quote.Points++
			case "source":
				contexts.revalidationErr = imageagent.ErrRevisionConflict
			case "disabled":
				config.enabled = false
			case "member":
				ctx = authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: "org", EffectiveOrganizationID: "org", UserID: "actor", EffectiveMemberID: "replacement"})
			}
			_, err = s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
			require.Error(t, err)
			require.Empty(t, workflows.starts)
			require.Zero(t, config.admissions)
		})
	}
}

func TestPrepareImageSetMissingSpecificationsBlocksBeforeGeneration(t *testing.T) {
	s, _, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	config.template.Detail = []agentconfig.ContentTask{{ID: "dimensions", Purpose: "specification_dimensions"}}
	_, err := s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrValidation)
	require.ErrorContains(t, err, "specifications evidence")
	require.Empty(t, workflows.starts)
}

func TestPrepareImageSetRejectsAResolvedDifferentStore(t *testing.T) {
	s, _, workflows, config, contexts, _, ctx, input := imageSetServiceFixture(t)
	input.Target = imageagent.ImageTargetSelection{Platform: "shein", StoreID: "chosen-store", Site: "shein-us", CategoryID: 1}
	contexts.preparation.Target = imageagent.ImageTarget{Platform: "shein", StoreID: "different-store", Site: "shein-us", CategoryID: 1, ProductTypeID: 2, ApplicationID: "application", ApplicationMode: "fully_managed", AttributesDigest: strings.Repeat("d", 64), VariantsDigest: strings.Repeat("e", 64), RequirementDigest: strings.Repeat("f", 64), RequirementVersion: "current"}
	contexts.preparation.OfficialPlacements = map[string]imageagent.OfficialImagePlacement{"identity": {Group: "skc", Type: 1, Sort: 1, Site: "shein-us"}, "overview": {Group: "detail", Type: 7, Sort: 1, Site: "shein-us"}}
	_, err := s.PrepareImageSet(ctx, input)
	require.ErrorIs(t, err, imageagent.ErrRevisionConflict)
	require.Empty(t, workflows.starts)
	require.Empty(t, config.snapshot.ID)
}

func TestPreparedSetCannotBypassConfirmationThroughOriginalStart(t *testing.T) {
	s, _, workflows, _, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	err = s.Start(ctx, imageagent.StartRunInput{RunID: "bypass", BusinessTaskID: "operation", TargetPlatform: "product", ImagePolicyContext: imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}, Mode: imageagent.RunModeManual, IdempotencyKey: "bypass", Plan: prepared.Projection.Plan, Budget: prepared.Projection.Run.Budget})
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	require.Empty(t, workflows.starts)
}

func TestCancelPreparedSetDoesNotNeedTemporalOrReviveOnConfirmation(t *testing.T) {
	s, repo, workflows, config, _, _, ctx, input := imageSetServiceFixture(t)
	prepared, err := s.PrepareImageSet(ctx, input)
	require.NoError(t, err)
	require.NoError(t, s.Cancel(ctx, prepared.Projection.Run.ID, 1, "cancel-prepared"))
	stored, err := repo.GetProjection(ctx, imageagent.ScopeForRun(prepared.Projection.Run))
	require.NoError(t, err)
	require.Equal(t, imageagent.RunStatusCancelled, stored.Run.Status)
	require.Empty(t, workflows.cancellations)
	_, err = s.ConfirmImagePlan(ctx, confirmSetInput(prepared))
	require.ErrorIs(t, err, imageagent.ErrCommandBlocked)
	require.Zero(t, config.admissions)
}
