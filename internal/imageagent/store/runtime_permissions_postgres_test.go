package store

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/imageagent"
	assetpersistence "task-processor/internal/integration/persistence/product/asset"
)

func TestOrganizationImageRuntimeRoleHasOnlyCurrentAPIGrants(t *testing.T) {
	dsn := os.Getenv("ISSUE487_TEST_DSN")
	if dsn == "" {
		t.Skip("result=SKIP: requires isolated ISSUE487_TEST_DSN")
	}
	require.Contains(t, dsn, "host=127.0.0.1")
	require.Contains(t, dsn, "user=issue487_owner")
	ctx := context.Background()
	root, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	rootPool, err := root.DB()
	require.NoError(t, err)
	name := "issue487_image_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	role := "image_agent_runtime_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:16]
	require.NoError(t, root.Exec("CREATE DATABASE "+name).Error)
	owner, err := gorm.Open(postgres.Open(dsn+" dbname="+name), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	ownerPool, err := owner.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ownerPool.Close())
		require.NoError(t, root.Exec("DROP DATABASE "+name+" WITH (FORCE)").Error)
		require.NoError(t, root.Exec("DROP ROLE "+role).Error)
		require.NoError(t, rootPool.Close())
	})
	require.NoError(t, AutoMigrateOrganizationScope(owner))
	require.NoError(t, assetpersistence.AutoMigrate(owner))
	require.NoError(t, owner.Exec(`CREATE TABLE unrelated_secret(value text)`).Error)
	require.NoError(t, owner.Exec("CREATE ROLE "+role+" LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION NOBYPASSRLS").Error)
	sum := sha256.Sum256([]byte(dsn))
	password := hex.EncodeToString(sum[:])
	require.NoError(t, owner.Exec("ALTER ROLE "+role+" PASSWORD '"+password+"'").Error)
	require.NoError(t, grantOrganizationRuntimePermissions(ctx, owner, role))
	runtimeDB, err := gorm.Open(postgres.Open(dsn+" dbname="+name+" user="+role+" password="+password), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := runtimeDB.DB()
	require.NoError(t, err)
	pool.SetMaxOpenConns(8)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, verifyOrganizationRuntimePermissions(ctx, runtimeDB, role))
	require.Error(t, verifyOrganizationRuntimePermissions(ctx, owner, role))
	run := manualRun("392ed0a2-0f01-4c94-9aa4-eb50271fae9c", "org-runtime")
	run.ScopeProtocol = imageagent.OrganizationScopeProtocol
	run.MemberID = "member-runtime"
	run.BusinessTaskID = "operation-runtime"
	run.TargetPlatform = "product"
	run.Status, run.CurrentNode = imageagent.RunStatusAwaitingPlanApproval, "confirm_plan"
	run.ActivePlanRevision = 1
	run.Version = 1
	plan := imageSetPlanForStore(t)
	limits := agentconfig.ImageRunLimits{Images: 1, Points: 20, ElapsedSeconds: 300}
	run.Budget = imageagent.ImageSetBudget(limits)
	scope := imageagent.ScopeForRun(*run)
	repository := NewOrganizationRepository(runtimeDB)
	_, err = repository.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan,
		Catalog: imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{
			{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"},
			{ID: "style-1", Type: imageagent.AuthorizedAssetStyle, URL: "https://style.example/style.png"},
		}},
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan}, CommitID: "start:runtime-start",
		EventType: "run.initialized", EventPayload: []byte(`{}`),
	})
	require.NoError(t, err, "current API Start must initialize only its admitted tables")
	current, err := repository.GetProjection(ctx, scope)
	require.NoError(t, err, "current API GET/Approve must read its projection")
	now := time.Now().UTC().Truncate(time.Microsecond)
	planDigest, err := imageagent.ImageSetPlanDigest(plan)
	require.NoError(t, err)
	receipt := agentconfig.ImageRunAdmissionReceipt{ID: "74e01be8-f4e6-461f-8acb-18fe37329f1c", AdmittedAt: now, Deadline: now.Add(300 * time.Second), Command: agentconfig.ImageRunAdmissionCommand{Scope: agent.Scope{OrganizationID: scope.TenantID, ActorID: scope.OwnerUserID}, Snapshot: plan.Set.Configuration, MemberID: run.MemberID, RunID: run.ID, ConfirmActionID: "1b912e40-d50a-48f5-a12b-62c73a42e4b8", SourceDigest: imageagent.ImageSetSourceDigest(plan.Set.Source, plan), InputDigest: plan.Set.InputDigest, PlanDigest: planDigest, QuoteDigest: plan.Set.QuoteDigest, Limits: limits}}
	receipt.Digest, err = hashJSON(receipt)
	require.NoError(t, err)
	next := current
	next.Run.ImageAdmission, next.Run.StartedAt = &receipt, receipt.AdmittedAt
	next.Run.Status, next.Run.CurrentNode, next.Run.Version = imageagent.RunStatusExecuting, "execute_slots", current.Run.Version+1
	_, err = repository.CommitProjection(ctx, imageagent.ProjectionCommit{Scope: scope, CommitID: "confirm:" + receipt.Command.ConfirmActionID, ExpectedProjectionVersion: current.ProjectionVersion, ExpectedRunVersion: current.Run.Version, Snapshot: next, RunMutation: &imageagent.RunMutation{ImageAdmission: &receipt, Status: next.Run.Status, CurrentNode: next.Run.CurrentNode, ActivePlanRevision: 1}, EventType: "run.confirmed", EventPayload: json.RawMessage(`{}`)})
	require.NoError(t, err, "current API confirms the set with the original receipt in its existing transaction")
	ownerRepository := NewOrganizationRepository(owner)
	identity := imageagent.SlotExternalEffectIdentity{RunScope: scope, PlanRevision: 1, SlotID: plan.Slots[0].ID, Attempt: 1}
	reservation := imageagent.SlotEffectV3Reservation{Identity: identity, IdempotencyKey: "original-attempt", InputFingerprint: strings.Repeat("b", 64)}
	originalEffect, won, err := ownerRepository.(imageagent.SlotExternalEffectV3Repository).ReserveSlotProviderV3(ctx, reservation)
	require.NoError(t, err)
	require.True(t, won)
	quote := plan.Slots[0].Recipe.Quote
	intent := imageagent.GenerationIntent{Identity: identity, MemberID: run.MemberID, CatalogHash: current.AssetCatalog.Manifest.Hash, SourceDigest: imageagent.ImageSourceBundleDigest(plan.Slots[0].Recipe.References), PromptVersion: imageagent.ImageSetSchema, InputProtocol: imageagent.ImageSetSchema, InputDigest: imageagent.ImageGenerationInputDigestFromFingerprint(reservation.InputFingerprint), RouteReference: quote.RouteReference, CredentialReference: quote.CredentialReference, ConfigurationVersion: quote.ConfigurationVersion, Provider: quote.Provider, Model: quote.Model, Protocol: quote.Protocol, Resolution: quote.Resolution, Quality: quote.Quality, PriceVersion: quote.PriceVersion, Points: quote.Points, LimitVersion: 1, MonthStart: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)}
	ownerFacts := ownerRepository.(imageagent.GenerationFactRepository)
	originalFact, err := ownerFacts.PrepareGenerationIntent(ctx, intent)
	require.NoError(t, err)
	originalFact, err = ownerFacts.BindGenerationReservation(ctx, intent, imageagent.GenerationReservationReceipt{IntentID: originalFact.IntentID, Fingerprint: originalFact.Fingerprint, OrganizationID: scope.TenantID, MemberID: run.MemberID, OperationID: "image-reserve:" + originalFact.IntentID, ReservationID: "reservation", ResourceType: "ai_point", Points: intent.Points, PriceVersion: intent.PriceVersion, LimitVersion: intent.LimitVersion, MonthStart: intent.MonthStart})
	require.NoError(t, err)
	_, won, err = ownerFacts.BeginGenerationDispatch(ctx, intent)
	require.NoError(t, err)
	require.True(t, won)
	originalFact, err = ownerFacts.RecordGenerationSuccess(ctx, intent, imageagent.GenerationSuccess{ResponseID: "response", RequestID: "request", ResultDigest: strings.Repeat("d", 64), ResultURL: "https://output.example.org/result.png"})
	require.NoError(t, err)
	originalFact, err = ownerFacts.BindGenerationSettlement(ctx, intent, imageagent.GenerationSettlementReceipt{IntentID: originalFact.IntentID, Fingerprint: originalFact.Fingerprint, OperationID: "image-finalize:" + originalFact.IntentID, ReservationID: "reservation", State: "committed", Points: intent.Points, ProofDigest: originalFact.TerminalProofDigest()})
	require.NoError(t, err)
	apiFact, err := repository.(imageagent.GenerationFactRepository).ReadGenerationFact(ctx, identity)
	require.NoError(t, err, "candidate preview must read the original settled generation fact through the admitted API role")
	require.Equal(t, originalFact, apiFact)
	apiEffect, err := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, identity)
	require.NoError(t, err, "candidate preview must read original materialization through the same admitted API role")
	require.Equal(t, originalEffect, apiEffect)
	wrongIdentity := identity
	wrongIdentity.TenantID = "another-org"
	_, err = repository.(imageagent.GenerationFactRepository).ReadGenerationFact(ctx, wrongIdentity)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	_, err = repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, wrongIdentity)
	require.ErrorIs(t, err, imageagent.ErrRunNotFound)
	for _, tc := range []struct{ name, grant, revoke string }{
		{"delete_run", "GRANT DELETE ON image_agent_v2_runs TO " + role, "REVOKE DELETE ON image_agent_v2_runs FROM " + role},
		{"write_attempt", "GRANT INSERT ON image_agent_v2_attempts TO " + role, "REVOKE INSERT ON image_agent_v2_attempts FROM " + role},
		{"other_table", "GRANT SELECT ON unrelated_secret TO " + role, "REVOKE SELECT ON unrelated_secret FROM " + role},
		{"missing_commit_insert", "REVOKE INSERT ON image_agent_v2_projection_commits FROM " + role, "GRANT INSERT ON image_agent_v2_projection_commits TO " + role},
		{"missing_approval_read", "REVOKE SELECT ON product_approval_receipts FROM " + role, "GRANT SELECT ON product_approval_receipts TO " + role},
		{"approval_write", "GRANT INSERT ON product_approved_assets TO " + role, "REVOKE INSERT ON product_approved_assets FROM " + role},
		{"whole_run_update", "GRANT UPDATE ON image_agent_v2_runs TO " + role, "REVOKE UPDATE ON image_agent_v2_runs FROM " + role},
		{"budget_column", "GRANT UPDATE(budget_json) ON image_agent_v2_runs TO " + role, "REVOKE UPDATE(budget_json) ON image_agent_v2_runs FROM " + role},
		{"missing_candidate_read", "REVOKE SELECT ON image_agent_v3_slot_external_effects FROM " + role, "GRANT SELECT ON image_agent_v3_slot_external_effects TO " + role},
		{"candidate_insert", "GRANT INSERT ON image_agent_v3_slot_external_effects TO " + role, "REVOKE INSERT ON image_agent_v3_slot_external_effects FROM " + role},
		{"candidate_update", "GRANT UPDATE ON image_agent_v3_slot_external_effects TO " + role, "REVOKE UPDATE ON image_agent_v3_slot_external_effects FROM " + role},
		{"candidate_delete", "GRANT DELETE ON image_agent_v3_slot_external_effects TO " + role, "REVOKE DELETE ON image_agent_v3_slot_external_effects FROM " + role},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, owner.Exec(tc.grant).Error)
			require.Error(t, verifyOrganizationRuntimePermissions(ctx, runtimeDB, role))
			require.NoError(t, owner.Exec(tc.revoke).Error)
			if tc.name == "whole_run_update" {
				for _, grant := range organizationRuntimeColumnGrants {
					require.NoError(t, owner.Exec("GRANT UPDATE("+grant.columns+") ON TABLE public."+grant.name+" TO "+role).Error)
				}
			}
			require.NoError(t, verifyOrganizationRuntimePermissions(ctx, runtimeDB, role))
		})
	}
	require.Error(t, runtimeDB.Exec("DELETE FROM image_agent_v2_runs").Error)
	require.Error(t, runtimeDB.Exec("SELECT value FROM unrelated_secret").Error)
	require.Error(t, runtimeDB.Exec("SELECT id FROM image_agent_v2_asset_catalog").Error)
	require.Error(t, runtimeDB.Exec("UPDATE image_agent_v2_runs SET budget_json='{}'").Error)
	require.Error(t, runtimeDB.Exec("INSERT INTO product_approval_receipts(tenant_id,action_id,payload_hash,asset_ids_json) VALUES('x','y','z','[]')").Error)
	require.Error(t, runtimeDB.Exec("UPDATE image_agent_v3_slot_external_effects SET generation_fact_json='{}'").Error)
	require.Error(t, runtimeDB.Exec("DELETE FROM image_agent_v3_slot_external_effects").Error)
}
