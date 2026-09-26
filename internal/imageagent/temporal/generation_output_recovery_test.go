package temporal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/ai"
	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
	"task-processor/internal/integration/grsai"
	resourceadapter "task-processor/internal/integration/orgresource"
	"task-processor/internal/ledger/orgresource"
	productimage "task-processor/internal/product/image"
)

type generationOutputResourceAuth struct{}

type lostGenerationStagingACK struct {
	imageagent.SlotExternalEffectV3Repository
	imageagent.GenerationStagingRecoveryRepository
	lost bool
}

func (r *lostGenerationStagingACK) RecoverGenerationStaging(ctx context.Context, res imageagent.SlotEffectV3Reservation, intent imageagent.GenerationIntent, proof string, manifest imageagent.StagingManifest) (imageagent.SlotEffectV3Attempt, error) {
	effect, err := r.GenerationStagingRecoveryRepository.RecoverGenerationStaging(ctx, res, intent, proof, manifest)
	if err == nil && !r.lost {
		r.lost = true
		return imageagent.SlotEffectV3Attempt{}, errors.New("controlled lost staging ACK")
	}
	return effect, err
}

func (generationOutputResourceAuth) AuthorizeImageGeneration(context.Context, imageagent.GenerationIntent) error {
	return nil
}

func TestGenerationOutputRestartRecoversBothV3Entrypoints(t *testing.T) {
	for _, entry := range []string{"execute_claimed", "recover_unknown", "execute_unknown", "recover_staging", "recover_ack", "recover_already_charged", "recover_bundle", "altered_input", "revoked", "unknown"} {
		t.Run(entry, func(t *testing.T) {
			ctx := context.Background()
			dsn := filepath.Join(t.TempDir(), "image.db") + "?_busy_timeout=5000&_journal_mode=WAL"
			db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			require.NoError(t, store.AutoMigrateOrganizationScope(db))
			repo := store.NewOrganizationRepository(db)
			run := imageagent.Run{ID: "run-recovery", ScopeProtocol: imageagent.OrganizationScopeProtocol, MemberID: "member-1", BusinessTaskID: "acquisition-1", TenantID: "org-1", UserID: "actor-1", Mode: imageagent.RunModeManual, IdempotencyKey: "run-key", Status: imageagent.RunStatusPlanning, CurrentNode: "plan", Version: 1, ActivePlanRevision: 1, MaxConcurrentSlots: 1}
			run.Budget = imageagent.Budget{MaxImages: 1, MaxModelCalls: 1}
			policy, err := run.Budget.Policy()
			require.NoError(t, err)
			plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan-key", SourceAssetIDs: []string{"source-1"}, CreatedBy: run.UserID, Slots: []imageagent.Slot{{ID: "main", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key"}}}
			catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/image.png"}}})
			require.NoError(t, err)
			_, err = repo.InitializeRun(ctx, imageagent.ProjectionInitialization{Scope: imageagent.ScopeForRun(run), Run: run, Plan: plan, Catalog: catalog, Snapshot: imageagent.RunProjection{Run: run, Plan: plan}, CommitID: "start", EventType: "run.initialized", EventPayload: []byte(`{}`)})
			require.NoError(t, err)
			input := ExecuteSlotV3ActivityInput{RunID: run.ID, Identity: imageagent.ExecutionIdentity{RunID: run.ID, ScopeProtocol: run.ScopeProtocol, TenantID: run.TenantID, UserID: run.UserID, MemberID: run.MemberID, BusinessTaskID: run.BusinessTaskID}, PlanRevision: 1, Slot: plan.Slots[0], Attempt: 1, IdempotencyKey: "slot-key:plan:1:attempt:1", AssetCatalog: catalog}
			input.TargetPlatform = "product"
			input.ImagePolicyContext = &imageagent.ImagePolicyContext{Country: "zz", Family: "default", SceneCategory: "general"}
			recoveryInput := effectRecoveryWorkflowInput(input)
			recoveryInput.TargetPlatform = input.TargetPlatform
			recoveryInput.ImagePolicyContext = input.ImagePolicyContext
			reservation := slotEffectReservationV3(slotExecutionInputV3(input))
			input.BudgetAuthorization = true
			input.BudgetPolicy = policy
			maximum := imageagent.UsageVector{Images: 1, ModelCalls: 1}
			reservation.Policy = policy
			reservation.Quote = imageagent.SlotUsageQuote{Maximum: maximum, Fingerprint: "quote-fixed", Operations: []imageagent.SlotUsageOperation{{Name: "render_white_background_source", Fingerprint: "op-fixed", MaximumOutputs: 1, Maximum: maximum}}}
			effects := repo.(imageagent.SlotExternalEffectV3Repository)
			_, won, err := effects.ReserveSlotProviderV3(ctx, reservation)
			require.NoError(t, err)
			require.True(t, won)
			facts := repo.(imageagent.GenerationFactRepository)
			intent := imageagent.GenerationIntent{Identity: reservation.Identity, MemberID: run.MemberID, CatalogHash: catalog.Manifest.Hash, SourceDigest: strings.Repeat("b", 64), PromptVersion: "p1", RouteReference: "r1", CredentialReference: "c1", ConfigurationVersion: "v1", Provider: "grsai", Model: "gpt-image-2.5", Protocol: "grsai-json-sync-v1", Resolution: "1024x1024", Quality: "auto", PriceVersion: "price1", Points: 12, LimitVersion: 1, MonthStart: orgresource.AIPointMonthStart(time.Now())}
			_, err = facts.PrepareGenerationIntent(ctx, intent)
			require.NoError(t, err)
			commercial, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "commercial.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			require.NoError(t, resourceadapter.AutoMigrate(commercial))
			sqlCommercial, _ := commercial.DB()
			t.Cleanup(func() { _ = sqlCommercial.Close() })
			require.NoError(t, commercial.Exec("INSERT INTO saas_organization_resource_buckets (organization_id,resource_type,available,reserved,consumed,created_at,updated_at) VALUES (?, 'ai_point', 100, 0, 0, ?, ?)", run.TenantID, time.Now(), time.Now()).Error)
			limits, err := resourceadapter.NewGormMemberLimitRepository(commercial, resourceadapter.TransactionConfig{})
			require.NoError(t, err)
			_, err = limits.SetMonthlyLimit(ctx, orgresource.SetMemberLimitExecution{OrganizationID: run.TenantID, MemberID: run.MemberID, ActorID: "admin", OperationID: "limit", Target: 20})
			require.NoError(t, err)
			resources, err := resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, facts, generationOutputResourceAuth{})
			require.NoError(t, err)
			receipt, err := resources.ReserveImageGeneration(ctx, intent.Identity)
			require.NoError(t, err)
			_, err = facts.BindGenerationReservation(ctx, intent, receipt)
			require.NoError(t, err)
			_, won, err = facts.BeginGenerationDispatch(ctx, intent)
			require.NoError(t, err)
			require.True(t, won)
			posts, gets := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method == http.MethodPost {
					posts++
					_ = json.NewEncoder(w).Encode(map[string]any{"id": "generated-1", "status": "succeeded", "results": []map[string]string{{"url": "https://output.example/result.png?signature=private"}}})
					return
				}
				gets++
				_, _ = w.Write(tinyPNGBytes(t))
			}))
			defer server.Close()
			client := grsai.NewClient(grsai.Config{Model: "gpt-image-2.5", SubmitURL: server.URL, HTTPClient: server.Client(), MaxAttempts: 1})
			if entry != "unknown" {
				_, err = client.EditImageOnce(ctx, &ai.ImageEditRequest{Model: "gpt-image-2.5", Prompt: "white", Image: tinyPNGBytes(t), ImageContentType: "image/png", N: 1}, func(ctx context.Context, o grsai.GenerationObservation) error {
					_, err := facts.RecordGenerationSuccess(ctx, intent, imageagent.GenerationSuccess{ResponseID: o.ResponseID, RequestID: o.RequestID, ResultDigest: o.ResultDigest}.WithResultLocator(o.ResultURL, o.ResultUnavailable))
					return err
				})
				require.NoError(t, err)
			}
			// Interruption after success persistence, before any download or bundle.
			if entry == "recover_already_charged" {
				settled, e := resources.FinalizeImageGeneration(ctx, intent.Identity)
				require.NoError(t, e)
				_, e = facts.BindGenerationSettlement(ctx, intent, settled)
				require.NoError(t, e)
			}
			if entry != "execute_claimed" {
				_, err = effects.MarkSlotProviderBudgetUnknownV3(ctx, reservation)
				require.NoError(t, err)
				phase, code := imageagent.SlotEffectV3ProviderUnknown, imageagent.SlotProviderOutcomeUnknownCode
				if entry == "recover_staging" {
					phase, code = imageagent.SlotEffectV3StagingUnknown, imageagent.SlotStagingOutcomeUnknownCode
				}
				require.NoError(t, db.Table("image_agent_v3_slot_external_effects").Where("run_id = ?", run.ID).Updates(map[string]any{"phase": string(phase), "blocked_code": code}).Error)
			}
			sqlImage, _ := db.DB()
			require.NoError(t, sqlImage.Close())
			db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			sqlImage, _ = db.DB()
			t.Cleanup(func() { _ = sqlImage.Close() })
			repo = store.NewOrganizationRepository(db)
			facts = repo.(imageagent.GenerationFactRepository)
			effects = repo.(imageagent.SlotExternalEffectV3Repository)
			resources, err = resourceadapter.NewGormImageGenerationRepository(commercial, resourceadapter.TransactionConfig{}, facts, generationOutputResourceAuth{})
			require.NoError(t, err)
			finalizer, err := imageagent.NewGenerationRecovery(facts, resources)
			require.NoError(t, err)
			executor := &recordingStagedExecutor{}
			api := &statefulActivityObjectStore{objects: map[string]activityS3Object{}}
			artifacts := newProductionArtifactStore(t, api)
			if entry == "recover_bundle" {
				prepared, e := prepareGeneratedSlotArtifacts(slotExecutionInputV3(input), imageagent.SlotGeneratedOutput{SlotID: input.Slot.ID, Attempt: 1, SourceAssetID: "source-1", Assets: []imageagent.GeneratedAsset{{Bytes: tinyPNGBytes(t), ContentType: "image/png", Width: 1, Height: 1, Operations: []string{productimage.SourceWhiteBackgroundOperation}}}}, artifacts)
				require.NoError(t, e)
				require.NoError(t, artifacts.PreserveSlotArtifacts(ctx, reservation.Identity, prepared))
			}
			activities := newV3Activities(t, repo, effects, executor, artifacts)
			if entry == "recover_ack" {
				activities.slotEffectsV3 = &lostGenerationStagingACK{SlotExternalEffectV3Repository: effects, GenerationStagingRecoveryRepository: repo.(imageagent.GenerationStagingRecoveryRepository)}
			}
			activities.executionAuthorizer = allowGenerationExecution{}
			activities.generationRecovery = finalizer
			if entry == "revoked" {
				activities.executionAuthorizer = &revokedGenerationAuthorizer{}
			}
			activities.generationOutputRecovery = func(ctx context.Context, in imageagent.SlotExecutionInput, f imageagent.GenerationFact) (imageagent.SlotGeneratedOutput, error) {
				require.Equal(t, "https://output.example/result.png?signature=private", f.Success.ResultURL)
				resp, err := server.Client().Get(server.URL + "/image")
				if err != nil {
					return imageagent.SlotGeneratedOutput{}, err
				}
				_ = resp.Body.Close()
				require.Equal(t, "product", in.TargetPlatform)
				require.Equal(t, *input.ImagePolicyContext, *in.ImagePolicyContext)
				return imageagent.SlotGeneratedOutput{SlotID: in.Slot.ID, Attempt: in.Attempt, SourceAssetID: "source-1", Assets: []imageagent.GeneratedAsset{{Bytes: tinyPNGBytes(t), ContentType: "image/png", Width: 1, Height: 1, SourceURL: "https://source.example/image.png", Operations: []string{productimage.SourceWhiteBackgroundOperation}}}}, nil
			}
			if entry == "altered_input" {
				altered := input
				altered.Slot.Brief = "different payload"
				_, err = activities.ExecuteSlotV3(ctx, altered)
				require.Error(t, err)
				require.Zero(t, gets, "conflicting input must not GET an original result")
				require.Zero(t, api.putCalls, "conflicting input must not poison the immutable bundle")
				require.Empty(t, api.objects)
			}
			if entry == "recover_ack" {
				_, err = activities.RecoverEffectV3(ctx, recoveryInput)
				require.Error(t, err)
				require.Equal(t, 1, gets)
			}
			if strings.HasPrefix(entry, "recover") {
				result, e := activities.RecoverEffectV3(ctx, recoveryInput)
				err = e
				require.Equal(t, EffectRecoveryOutcomePublished, result.Outcome)
			} else {
				_, err = activities.ExecuteSlotV3(ctx, input)
			}
			if entry == "revoked" || entry == "unknown" {
				require.Error(t, err)
				require.Zero(t, gets)
			} else {
				require.NoError(t, err)
				wantGets := 1
				if entry == "recover_bundle" {
					wantGets = 0
				}
				require.Equal(t, wantGets, gets)
				_, err = activities.ExecuteSlotV3(ctx, input)
				require.NoError(t, err)
				require.Equal(t, wantGets, gets)
			}
			require.Zero(t, executor.GenerateCalls(), "recovery cannot invoke generator")
			if entry == "unknown" {
				require.Zero(t, posts)
			} else {
				require.Equal(t, 1, posts)
			}
			stored, err := effects.GetSlotExternalEffectV3(ctx, reservation.Identity)
			require.NoError(t, err)
			if entry == "unknown" || entry == "revoked" {
				require.Equal(t, imageagent.SlotBudgetUnknown, stored.BudgetStatus)
			} else {
				require.Equal(t, imageagent.SlotBudgetCommitted, stored.BudgetStatus)
				require.EqualValues(t, 1, stored.Receipt.Actual.Images)
				require.EqualValues(t, 1, stored.Receipt.Actual.ModelCalls)
			}
			var bucket struct{ Consumed, Reserved int64 }
			require.NoError(t, commercial.Table("saas_organization_resource_buckets").Take(&bucket).Error)
			if entry == "unknown" {
				require.EqualValues(t, 12, bucket.Reserved)
				require.Zero(t, bucket.Consumed)
			} else {
				require.EqualValues(t, 12, bucket.Consumed)
				require.Zero(t, bucket.Reserved)
			}
		})
	}
}
