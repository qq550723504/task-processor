package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	officialstore "task-processor/internal/integration/persistence/listing/official"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
	"task-processor/internal/app/productsourcing"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/authidentity"
	"task-processor/internal/authruntime/zitadel"
	"task-processor/internal/authz"
	prepstore "task-processor/internal/integration/persistence/listing/preparation"
	recordstore "task-processor/internal/integration/persistence/listing/record"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	assetstore "task-processor/internal/integration/persistence/product/asset"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/marketplace/shein/goods"
	sheinmodel "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/review"
	"task-processor/internal/storecenter"
	"task-processor/internal/workbenchcontext"
)

func TestSupplyProductAgentDurableExecutionUsesRealOwners(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("issue398_owner"), tcpostgres.WithUsername("issue398_owner"), tcpostgres.WithPassword("isolated-supply-agent"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	t.Setenv("ISSUE398_TEST_DSN", fmt.Sprintf("host=127.0.0.1 port=%s user=issue398_owner password=isolated-supply-agent sslmode=disable", port.Port()))
	for _, mode := range []string{"supply canonical", "supply member replaced", "supply first reconstruction"} {
		t.Run(mode, func(t *testing.T) { testProductAgentOwners(t, mode) })
	}
}

type supplyAgentFixtureRules struct {
	merchant storecenter.ProductMerchantBinding
}
type supplyStageMerchant struct {
	supplyapp.RulesMerchant
	binding storecenter.ProductMerchantBinding
}

func (r supplyStageMerchant) Binding() storecenter.ProductMerchantBinding { return r.binding }
func (r supplyAgentFixtureRules) RulesMerchant(context.Context, collection.Scope, string, *storecenter.ProductMerchantBinding) (supplyapp.RulesMerchant, error) {
	return supplyStageMerchant{binding: r.merchant}, nil
}

func (r supplyAgentFixtureRules) ReadTargetRules(context.Context, collection.Scope, string, goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error) {
	return r.merchant, goods.OfficialRuleSnapshot{ApplicationMode: sheinmodel.ModeSelfOperated}, nil
}

type supplyAgentUnusedProbe struct{}

func (supplyAgentUnusedProbe) Probe(context.Context, asset.ApprovedAsset, int) (goods.OfficialImageObservation, error) {
	panic("this fixture selects no images")
}

func testSupplyAgentOwners(t *testing.T, mode string, f *acquisitionHTTPFixture, acquisition acquisitionResultDTO, a *productAgentApplication, deps routeAuthDependencies, permissions *authz.ListingKitAuthorizer, calls *atomic.Int32) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	scope := collection.Scope{OrganizationID: "B", ActorID: "admin", MemberID: "member-B-admin"}
	actor := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, Roles: []string{"listingkit_admin"}, TokenExpiresAt: time.Now().Add(time.Minute)})
	actor, err := (productReviewCapabilityBinder{}).Bind(actor, "Bearer admin")
	require.NoError(t, err)
	live := &productReviewLiveOrganizationAccess{resolver: deps.organizationResolver, now: time.Now}
	collectionAuth, err := collection.NewContextAuthorizer(live, permissions)
	require.NoError(t, err)
	require.NoError(t, collectionstore.InstallSchema(f.owner))
	for _, install := range []func(*gorm.DB) error{prepstore.InstallSchema, recordstore.InstallSchema, prepstore.InstallOperationSchema, submissionstore.InstallSchema, officialstore.InstallOfficialSchema} {
		require.NoError(t, install(f.owner))
	}
	collectionRepo, err := collectionstore.NewRepository(ctx, f.owner, func(*gorm.DB) (collectionstore.OwnPublisher, error) { return nil, collection.ErrUnavailable })
	require.NoError(t, err)
	receipts, err := productsourcing.NewPublishedAcquisitionReader(ctx, f.owner, live, permissions)
	require.NoError(t, err)
	collections, err := collection.NewService(collectionRepo, collectionAuth, receipts)
	require.NoError(t, err)
	batch, err := collections.Mutate(actor, uuid.NewString(), collection.Mutation{Action: "create_batch", Name: "供应链智能体组合验证"})
	require.NoError(t, err)
	_, err = collections.Mutate(actor, uuid.NewString(), collection.Mutation{Action: "add_acquisition", BatchID: batch.BatchID, SourceOperationID: acquisition.OperationID})
	require.NoError(t, err)
	prepAuth, err := preparation.NewContextAuthorizer(live, permissions)
	require.NoError(t, err)
	prepRepo, err := prepstore.NewRepository(ctx, f.owner)
	require.NoError(t, err)
	preparations, err := preparation.NewService(prepRepo, collections, prepAuth)
	require.NoError(t, err)
	transferred, err := preparations.Transfer(actor, uuid.NewString(), preparation.TransferInput{BatchID: batch.BatchID, ExpectedRevision: 2})
	require.NoError(t, err)
	page, err := preparations.ListSources(actor, transferred.Preparation.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	var replaced atomic.Bool
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		member := scope.MemberID
		if replaced.Load() {
			member = "replacement-member"
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"pagination":{"totalResult":"1"},"authorizations":[{"id":%q,"project":{"id":"project"},"organization":{"id":"B"},"user":{"id":"admin"},"state":"STATE_ACTIVE","roles":[{"key":"listingkit_admin"}]}]}`, member)
	}))
	defer iam.Close()
	authority := supplyapp.OrganizationExecutionAuthorizer{Client: zitadel.NewAuthorizationClient(iam.URL, iam.Client()), ServiceToken: func(context.Context) (string, error) { return "isolated-iam", nil }, ProjectID: "project", Permissions: permissions}
	snapshots, err := catalogstore.NewBoundedSnapshotReader(f.owner, record.MaxPayloadBytes)
	require.NoError(t, err)
	selector, err := preparation.NewSourceSelector(preparations, collections, prepRepo, snapshots)
	require.NoError(t, err)
	selector, err = selector.WithExecution(authority, collection.ExecutionOwnerAuthority{Authorization: authority})
	require.NoError(t, err)
	operationsRepo, err := prepstore.NewOperationRepository(ctx, f.owner)
	require.NoError(t, err)
	operations, err := preparation.NewOperationService(preparations, collections, operationsRepo)
	require.NoError(t, err)
	operations, err = operations.WithExecution(authority)
	require.NoError(t, err)
	records, err := recordstore.NewRepository(ctx, f.owner)
	require.NoError(t, err)
	core, err := buildProductReviewCore(f.owner, deps.organizationResolver.(*workbenchcontext.Resolver), permissions)
	require.NoError(t, err)
	effective := supplyapp.EffectiveProductReader{Reviews: core.store, Snapshots: snapshots}
	assets, err := assetstore.NewBoundedApprovedInventoryReader(f.owner, record.MaxPayloadBytes)
	require.NoError(t, err)
	storeID := uuid.NewString()
	rules := supplyAgentFixtureRules{storecenter.ProductMerchantBinding{OrganizationID: scope.OrganizationID, StoreID: storeID, Site: "shein-us", StoreVersion: 1, ConnectionRevision: 1, ApplicationID: "app-a", ApplicationType: storecenter.ApplicationSelfOperated, ApplicationRevision: "v1:self_operated", SupplierIdentityHash: collection.Digest("fixture-merchant"), ServiceExpiresAt: time.Now().UTC().Add(time.Hour)}}
	targets, err := record.NewTargetService(record.TargetDependencies{Sources: selector, ExecutionSources: selector, ExecutionAuthorization: authority, Products: effective, Assets: assets, Rules: rules, Records: records, Images: supplyAgentUnusedProbe{}, Authorizer: prepAuth})
	require.NoError(t, err)
	var target record.TargetReceipt
	if mode != "supply first reconstruction" {
		target, err = targets.Create(actor, uuid.NewString(), record.TargetInput{SourceID: page.Items[0].ID, StoreID: storeID, EffectiveVersion: 1})
		require.NoError(t, err)
	}
	app := &supplyapp.Application{Preparations: preparations, Authorization: prepAuth, PublicationStores: rules, StageProjection: supplyapp.ReviewProjection{Facts: prepRepo, Reviews: core.store}, Sources: selector, Operations: operations, Records: records, Rules: rules, Products: effective, Execution: supplyapp.OperationApplication{}}
	app.Execution.Repository = operationsRepo
	bridge, err := connectSupplyProductAgent(ctx, f.owner, app, a, authority)
	require.NoError(t, err)
	template, err := a.configuration.Execute(actor, agentconfig.Command{Scope: agent.Scope{OrganizationID: "B", ActorID: "admin"}, Key: uuid.NewString(), AgentID: a.definition.ID, Operation: "create-template", Input: agentconfig.TemplateInput{Name: "SHEIN 标题", TargetPlatform: "shein"}})
	require.NoError(t, err)
	profile, config, choice, err := bridge.titleSelection(ctx, agent.Scope{"B", "admin"}, agentconfig.TemplateRef{TemplateID: template.TemplateID, Revision: "1"})
	require.NoError(t, err)
	operation, err := operations.Create(actor, uuid.NewString(), preparation.OperationInput{PreparationID: transferred.Preparation.ID, ExpectedRevision: 1, StoreID: storeID, Action: preparation.OperationOptimize, TitleTemplateID: template.TemplateID, TitleTemplateRevision: "1", TitleQuoteHash: bridge.titleQuote(agent.Scope{"B", "admin"}, profile, config, choice)})
	require.NoError(t, err)
	items, err := operations.ListItems(actor, operation.Operation.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	if mode == "supply first reconstruction" {
		require.Equal(t, preparation.ItemPending, items.Items[0].Status)
		// Simulate a crash after the immutable Create commit but before pinning.
		adaptKey := preparation.ItemCommandID(operation.Operation.ID, page.Items[0].ID, preparation.OperationAdapt)
		created, err := targets.CreateForExecution(ctx, scope, adaptKey, record.TargetInput{SourceID: page.Items[0].ID, StoreID: storeID, EffectiveVersion: 1})
		require.NoError(t, err)
		proof, err := operations.AuthorizeExecution(ctx, "B", operation.Operation.ID)
		require.NoError(t, err)
		_, err = operationsRepo.BeginOperationItem(ctx, proof, page.Items[0].ID)
		require.NoError(t, err)
		// The composite FK rejects moving the original record to another actor.
		require.Error(t, f.owner.Exec("UPDATE listing_target_records SET actor_id='another-actor' WHERE id=?", created.Record.ID).Error)
		otherRules := rules
		otherRules.merchant.StoreID = uuid.NewString()
		otherTargets, err := record.NewTargetService(record.TargetDependencies{Sources: selector, ExecutionSources: selector, ExecutionAuthorization: authority, Products: effective, Assets: assets, Rules: otherRules, Records: records, Images: supplyAgentUnusedProbe{}, Authorizer: prepAuth})
		require.NoError(t, err)
		foreign, err := otherTargets.CreateForExecution(ctx, scope, uuid.NewString(), record.TargetInput{SourceID: page.Items[0].ID, StoreID: otherRules.merchant.StoreID, EffectiveVersion: 1})
		require.NoError(t, err)
		proof, err = operations.AuthorizeExecution(ctx, "B", operation.Operation.ID)
		require.NoError(t, err)
		_, err = operationsRepo.BindOperationTarget(ctx, proof, page.Items[0].ID, foreign.Record.ID, foreign.Record.Revision)
		require.ErrorIs(t, err, preparation.ErrConflict)
		activity := supplyapp.OperationActivities{Operations: operations, Repository: operationsRepo, Sources: selector, Products: effective, Targets: records, Creator: targets, Optimizer: bridge}
		out, err := activity.Process(ctx, supplyapp.OperationExecution{OrganizationID: "B", OperationID: operation.Operation.ID}, page.Items[0].ID)
		require.NoError(t, err)
		require.Equal(t, preparation.ItemReview, out.Status)
		require.Equal(t, created.Record.ID, out.RecordID, "restart must reuse the committed original record")
		require.EqualValues(t, 1, out.RecordRevision)
		_, err = activity.Process(ctx, supplyapp.OperationExecution{OrganizationID: "B", OperationID: operation.Operation.ID}, page.Items[0].ID)
		require.NoError(t, err)
		require.EqualValues(t, 4, calls.Load(), "same operation cannot invoke the model twice")
		proof, err = operations.AuthorizeExecution(ctx, "B", operation.Operation.ID)
		require.NoError(t, err)
		_, err = operationsRepo.BindOperationTarget(ctx, proof, out.SourceID, uuid.NewString(), out.RecordRevision)
		require.ErrorIs(t, err, preparation.ErrConflict)
		_, err = operationsRepo.BindOperationTarget(ctx, proof, uuid.NewString(), out.RecordID, out.RecordRevision)
		require.Error(t, err)
		facts, err := app.Stages(actor, transferred.Preparation.ID, storeID, "review", collection.Query{Limit: 20}, "")
		require.NoError(t, err)
		require.EqualValues(t, 1, facts.Counts["review"])
		require.NotNil(t, facts.Items[0].Review)
		saved, err := records.ReadTargetRecord(ctx, scope, out.RecordID)
		require.NoError(t, err)
		require.ErrorIs(t, app.StageProjection.RequireUploadReady(ctx, scope, saved), record.ErrNotReady)
		return
	}
	require.Equal(t, target.Record.ID, items.Items[0].RecordID, "optimization must pin the original immutable target before execution")
	key := preparation.ItemCommandID(operation.Operation.ID, page.Items[0].ID, preparation.OperationOptimize)
	if mode == "supply member replaced" {
		replaced.Store(true)
		_, err = bridge.Optimize(ctx, operation.Operation, items.Items[0], key)
		require.Error(t, err)
		require.Zero(t, calls.Load(), "a replacement IAM member cannot execute the original command")
		return
	}
	id, err := bridge.Optimize(ctx, operation.Operation, items.Items[0], key)
	require.NoError(t, err)
	require.True(t, collection.ValidID(id))
	proposal, err := core.store.Read(ctx, review.Scope{Org: "B", Actor: "admin"}, id)
	require.NoError(t, err)
	require.Equal(t, "pending", proposal.State)
	require.Equal(t, "Reviewed bottle title", proposal.Title)
	require.Equal(t, acquisition.PublicationID, proposal.BasePublicationID)
	require.EqualValues(t, 4, calls.Load())
	replayed, err := bridge.Optimize(ctx, operation.Operation, items.Items[0], key)
	require.NoError(t, err)
	require.Equal(t, id, replayed)
	require.EqualValues(t, 4, calls.Load(), "same durable command must reuse Agent and Review receipts")
	human, err := review.NewCandidateService(core.reader, core.sourceReader, core.store, permissions)
	require.NoError(t, err)
	accepted, err := human.Decide(actor, uuid.NewString(), id, review.DecisionInput{Action: "accept", ExpectedRevision: proposal.Revision})
	require.NoError(t, err)
	applied, err := human.Apply(actor, uuid.NewString(), id, review.ApplyInput{ExpectedRevision: accepted.Revision})
	require.NoError(t, err)
	require.EqualValues(t, 2, applied.Receipt.ProductVersion)
	targetTwo, err := targets.Create(actor, uuid.NewString(), record.TargetInput{SourceID: page.Items[0].ID, StoreID: storeID, ExpectedRevision: target.Record.Revision, EffectiveVersion: 2, ApplyReceiptID: id})
	require.NoError(t, err)
	second, err := operations.Create(actor, uuid.NewString(), operation.Operation.Input)
	require.NoError(t, err)
	secondItems, err := operations.ListItems(actor, second.Operation.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	require.Equal(t, targetTwo.Record.ID, secondItems.Items[0].RecordID)
	secondID, err := bridge.Optimize(ctx, second.Operation, secondItems.Items[0], preparation.ItemCommandID(second.Operation.ID, page.Items[0].ID, preparation.OperationOptimize))
	require.NoError(t, err)
	require.NotEqual(t, id, secondID)
	secondProposal, err := core.store.Read(ctx, review.Scope{Org: "B", Actor: "admin"}, secondID)
	require.NoError(t, err)
	require.EqualValues(t, 2, secondProposal.Input.BaseVersion)
	require.Equal(t, applied.Receipt.PublicationID, secondProposal.BasePublicationID)
	require.Equal(t, "Reconstructed bottle title", secondProposal.Title)
	require.EqualValues(t, 8, calls.Load())
	acceptedTwo, err := human.Decide(actor, uuid.NewString(), secondID, review.DecisionInput{Action: "accept", ExpectedRevision: secondProposal.Revision})
	require.NoError(t, err)
	appliedTwo, err := human.Apply(actor, uuid.NewString(), secondID, review.ApplyInput{ExpectedRevision: acceptedTwo.Revision})
	require.NoError(t, err)
	require.EqualValues(t, 3, appliedTwo.Receipt.ProductVersion)
	selected, err := selector.SelectForExecution(ctx, scope, page.Items[0].ID)
	require.NoError(t, err)
	final, err := effective.ReadEffectiveTargetProduct(ctx, selected, 3, secondID)
	require.NoError(t, err)
	require.Equal(t, "Reconstructed bottle title", final.Snapshot.Title)
	_, err = core.store.ReadAppliedPublication(ctx, review.Scope{Org: "B", Actor: "other-admin", Admin: true}, acquisition.ProductKey, 3, final.PublicationID)
	require.ErrorIs(t, err, review.ErrNotFound, "even administrators cannot widen the lineage owner")
	_, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.False(t, authenticated, "worker execution must never fabricate a JWT identity")
}
