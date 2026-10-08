package recordpersistence_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	supplyapp "task-processor/internal/app/supplychain"
	"task-processor/internal/authidentity"
	preparationstore "task-processor/internal/integration/persistence/listing/preparation"
	recordstore "task-processor/internal/integration/persistence/listing/record"
	submissionstore "task-processor/internal/integration/persistence/listing/submission"
	catalogstore "task-processor/internal/integration/persistence/product/catalog"
	collectionstore "task-processor/internal/integration/persistence/product/collection"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/listing/submission"
	"task-processor/internal/marketplace/shein/goods"
	model "task-processor/internal/marketplace/shein/model"
	"task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/storecenter"
)

type fixedAuth struct{ scope collection.Scope }

func (a fixedAuth) Authorize(context.Context, string) (collection.Scope, error) { return a.scope, nil }

type unusedSources struct {
	sourcing.PublishedAcquisitionReader
}
type missingAssets struct{}

type executionAuth struct{ scope collection.Scope }

func (a executionAuth) AuthorizeExecution(_ context.Context, scope collection.Scope, permission string) error {
	if scope != a.scope || permission != collection.PermissionRead && permission != preparation.PermissionRead && permission != preparation.PermissionManage && permission != preparation.PermissionSubmit {
		return collection.ErrForbidden
	}
	return nil
}

func (missingAssets) GetApprovedInventory(context.Context, asset.InventoryScope) (asset.ApprovedAssetInventory, error) {
	return asset.ApprovedAssetInventory{}, asset.ErrApprovedAssetsNotReady
}

type currentRules struct{}

type readyAssets struct{}

func (readyAssets) GetApprovedInventory(_ context.Context, scope asset.InventoryScope) (asset.ApprovedAssetInventory, error) {
	values := []asset.ApprovedAsset{}
	for _, id := range []string{"main", "detail", "square"} {
		values = append(values, asset.ApprovedAsset{ID: id, URL: "https://images.example.org/" + id + ".jpg", Width: 900, Height: 900})
	}
	return asset.ApprovedAssetInventory{Scope: scope, Assets: values}, nil
}
func pointer[T any](value T) *T { return &value }

type readyRules struct{}

func (readyRules) ReadTargetRules(ctx context.Context, scope collection.Scope, storeID string, input goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error) {
	binding, _, err := (currentRules{}).ReadTargetRules(ctx, scope, storeID, input)
	return binding, goods.OfficialRuleSnapshot{ApplicationMode: model.ModeSelfOperated, Warehouses: []model.Warehouse{}, Categories: []model.Category{{ID: 123, ProductTypeID: 456, Leaf: pointer(true)}}, Sites: []model.MainSite{{ID: "shein", Sites: []model.Site{{Abbreviation: "shein-us", Status: pointer(1), StoreType: pointer(2), Currency: "USD"}}}}, Fill: model.FillStandards{DefaultLanguage: "en", DefaultTitleMaximum: pointer(150), SupplierCodeInSPU: pointer(false), Fields: []model.FillRule{}, Pictures: []model.PictureRule{{Field: "switch_spu_picture", Enabled: pointer(false)}, {Field: "sku_image_required", Enabled: pointer(false)}}}, Attributes: model.AttributeTemplate{ProductTypeID: 456, MainAttributeStatus: pointer(1), Attributes: []model.Attribute{{ID: 12, Name: "Default", Type: pointer(1), Show: pointer(1), MainLabel: pointer(1), Mode: pointer(2), Status: pointer(3), MaximumSelections: pointer(1), Options: []model.AttributeOption{{ID: 34, Show: pointer(1), Name: "Default"}}}}}, Linked: []model.LinkedRules{{GroupID: "product", Attributes: []model.LinkedAttributeRule{}}, {GroupID: "sku-0-0", Attributes: []model.LinkedAttributeRule{}}}, Brands: []model.Brand{{Code: "brand-a", Name: "Fixture brand"}}}, err
}

func (currentRules) ReadTargetRules(_ context.Context, scope collection.Scope, storeID string, _ goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error) {
	return storecenter.ProductMerchantBinding{OrganizationID: scope.OrganizationID, StoreID: storeID, Site: "shein-us", StoreVersion: 1, ConnectionRevision: 1, ApplicationRevision: "app-v1:self_operated", ApplicationID: "app-a", ApplicationType: storecenter.ApplicationSelfOperated, SupplierIdentityHash: collection.Digest("merchant-a"), ServiceExpiresAt: time.Now().Add(time.Hour)}, goods.OfficialRuleSnapshot{ApplicationMode: model.ModeSelfOperated}, nil
}

func TestPostgresTargetImmutableRevisionsCommandReplayAndConcurrentCAS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	container, err := tcpostgres.Run(ctx, "postgres:16-alpine", tcpostgres.WithDatabase("target605"), tcpostgres.WithUsername("target_owner"), tcpostgres.WithPassword("isolated-target-test"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, container.Terminate(context.Background())) })
	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	pool, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, pool.Close()) })
	require.NoError(t, catalogstore.AutoMigrate(db))
	require.NoError(t, collectionstore.InstallSchema(db))
	require.NoError(t, preparationstore.InstallSchema(db))
	_, err = recordstore.NewRepository(ctx, db)
	require.ErrorIs(t, err, record.ErrUnavailable, "serving construction never installs missing schema")
	require.NoError(t, recordstore.InstallSchema(db))
	repository, err := recordstore.NewRepository(ctx, db)
	require.NoError(t, err)
	scope := collection.Scope{OrganizationID: "org-a", ActorID: "actor-a", MemberID: "member-a"}
	authority := fixedAuth{scope}
	collectionsRepo, err := collectionstore.NewRepository(ctx, db, func(*gorm.DB) (collectionstore.OwnPublisher, error) { return nil, collection.ErrUnavailable })
	require.NoError(t, err)
	collections, err := collection.NewService(collectionsRepo, authority, unusedSources{})
	require.NoError(t, err)
	prepRepo, err := preparationstore.NewRepository(ctx, db)
	require.NoError(t, err)
	preparations, err := preparation.NewService(prepRepo, collections, authority)
	require.NoError(t, err)
	batchID, itemID, productKey, publication := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	snapshot := catalog.ProductSnapshot{Title: "Original"}
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, db.Create(&catalogstore.SnapshotVersionRecord{TenantID: scope.OrganizationID, ProductKey: productKey, Version: 1, PublicationID: publication, PayloadHash: collection.Digest(snapshot), SnapshotJSON: raw}).Error)
	require.NoError(t, db.Exec("INSERT INTO product_collection_batches(organization_id,actor_id,member_id,id,name,kind,revision,created_at) VALUES(?,?,?,?,?,'manual',1,now())", scope.OrganizationID, scope.ActorID, scope.MemberID, batchID, "原始资料").Error)
	require.NoError(t, db.Exec("INSERT INTO product_collection_items(organization_id,actor_id,member_id,id,batch_id,product_key,publication_id,original_version,source_kind,source_operation_id,revision,created_at) VALUES(?,?,?,?,?,?,?,1,'own','',1,now())", scope.OrganizationID, scope.ActorID, scope.MemberID, itemID, batchID, productKey, publication).Error)
	actor := authidentity.WithAuthenticatedIdentity(ctx, authidentity.AuthenticatedIdentity{TenantID: scope.OrganizationID, EffectiveOrganizationID: scope.OrganizationID, UserID: scope.ActorID, EffectiveMemberID: scope.MemberID, TokenExpiresAt: time.Now().Add(time.Minute)})
	transfer, err := preparations.Transfer(actor, uuid.NewString(), preparation.TransferInput{BatchID: batchID, ExpectedRevision: 1})
	require.NoError(t, err)
	page, err := preparations.ListSources(actor, transfer.Preparation.ID, collection.Query{Limit: 100})
	require.NoError(t, err)
	snapshots, err := catalogstore.NewBoundedSnapshotReader(db, record.MaxPayloadBytes)
	require.NoError(t, err)
	selector, err := preparation.NewSourceSelector(preparations, collections, prepRepo, snapshots)
	require.NoError(t, err)
	execution := executionAuth{scope}
	selector, err = selector.WithExecution(execution, collection.ExecutionOwnerAuthority{Authorization: execution})
	require.NoError(t, err)
	service, err := record.NewTargetService(record.TargetDependencies{Sources: selector, ExecutionSources: selector, ExecutionAuthorization: execution, Products: record.OriginalTargetProduct{}, Assets: missingAssets{}, Rules: currentRules{}, Records: repository, Images: recordTestImages{}, Authorizer: authority})
	require.NoError(t, err)
	key := uuid.NewString()
	input := record.TargetInput{SourceID: page.Items[0].ID, StoreID: uuid.NewString(), EffectiveVersion: 1, Draft: goods.OfficialDraftInput{Product: model.PublishProduct{Names: []model.LanguageContent{{Language: "en", Name: "Manual title"}}}}}
	first, err := service.Create(actor, key, input)
	require.NoError(t, err)
	require.EqualValues(t, 1, first.Record.Revision)
	require.NotEmpty(t, first.Record.Result.Issues)
	replay, err := service.Create(actor, key, input)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, first.Record, replay.Record, "jsonb ordering and timestamp precision preserve the immutable receipt")
	changed := input
	changed.ExpectedRevision = 1
	_, err = service.Create(actor, key, changed)
	require.ErrorIs(t, err, record.ErrConflict)
	_, err = service.Create(actor, uuid.NewString(), input)
	require.ErrorIs(t, err, record.ErrConflict, "new command cannot overwrite revision 1 using expected revision 0")
	var outcomes [2]error
	var candidates [2]record.TargetReceipt
	var wait sync.WaitGroup
	for i := range outcomes {
		wait.Add(1)
		go func(i int) {
			defer wait.Done()
			candidates[i], outcomes[i] = service.Create(actor, uuid.NewString(), changed)
		}(i)
	}
	wait.Wait()
	winner := 0
	if outcomes[0] != nil {
		winner = 1
	}
	require.NoError(t, outcomes[winner])
	require.ErrorIs(t, outcomes[1-winner], record.ErrConflict)
	require.EqualValues(t, 2, candidates[winner].Record.Revision)
	head, err := repository.ReadTargetHead(actor, scope, first.Record.TargetID)
	require.NoError(t, err)
	require.Equal(t, candidates[winner].Record, head)
	original, err := repository.ReadTargetCommand(actor, scope, key)
	require.NoError(t, err)
	require.Equal(t, first.Record, original.Record, "new target revision never mutates the original record")
	for _, foreign := range []collection.Scope{{"org-b", scope.ActorID, scope.MemberID}, {scope.OrganizationID, "actor-b", scope.MemberID}, {scope.OrganizationID, scope.ActorID, "rejoined-member"}} {
		_, err = repository.ReadTargetHead(actor, foreign, first.Record.TargetID)
		require.ErrorIs(t, err, record.ErrNotFound)
		_, err = repository.ReadTargetCommand(actor, foreign, key)
		require.ErrorIs(t, err, record.ErrNotFound)
	}
	_, err = repository.SaveTarget(actor, record.TargetPrepared{})
	require.ErrorIs(t, err, record.ErrForbidden)
	var count int64
	require.NoError(t, db.Table("listing_target_records").Count(&count).Error)
	require.EqualValues(t, 2, count, "losing CAS rolls back its record and command together")
	changed.ExpectedRevision = 2
	worker, err := service.CreateForExecution(ctx, scope, uuid.NewString(), changed)
	require.NoError(t, err, "worker commits use live execution proof without a fabricated authenticated identity")
	require.EqualValues(t, 3, worker.Record.Revision)
	_, authenticated := authidentity.AuthenticatedIdentityFromContext(ctx)
	require.False(t, authenticated)
	_, err = service.CreateForExecution(ctx, collection.Scope{scope.OrganizationID, scope.ActorID, "replacement-member"}, uuid.NewString(), changed)
	require.ErrorIs(t, err, record.ErrForbidden)
	t.Run("official intent and full result share the existing execution transaction", func(t *testing.T) {
		ready, err := record.NewTargetService(record.TargetDependencies{Sources: selector, ExecutionSources: selector, ExecutionAuthorization: execution, Products: record.OriginalTargetProduct{}, Assets: readyAssets{}, Rules: readyRules{}, Records: repository, Images: recordTestImages{}, Authorizer: authority})
		require.NoError(t, err)
		input := record.TargetInput{SourceID: page.Items[0].ID, StoreID: uuid.NewString(), EffectiveVersion: 1}
		input.Draft.Product = model.PublishProduct{CategoryID: 123, BrandCode: "brand-a", Names: []model.LanguageContent{{Language: "en", Name: "Manual complete title"}}, Descriptions: []model.LanguageContent{{Language: "en", Name: "Actual test product description"}}, Attributes: []model.AttributeValue{}}
		input.Draft.Product.SKCs = []model.ProductSKC{{
			SupplierCode: "merchant-product", SaleAttribute: model.AttributeValue{AttributeID: 12, AttributeValueID: pointer(int64(34))},
			SKUs: []model.ProductSKU{{SupplierSKU: "merchant-sku", Length: "10", Width: "10", Height: "10", Weight: pointer(100.0), MallState: 1, Prices: []model.ProductPrice{{BasePrice: 12.5, Currency: "USD", SubSite: "shein-us"}}, Stock: []model.ProductStock{{Quantity: 5}}, SaleAttributes: []model.AttributeValue{}}},
		}}
		input.Draft.Images = []goods.OfficialImageSlot{{Group: "skc", AssetID: "main", Type: 1, Sort: 1}, {Group: "skc", AssetID: "detail", Type: 2, Sort: 2}, {Group: "skc", AssetID: "square", Type: 5, Sort: 3}}
		completed, err := ready.CreateForExecution(ctx, scope, uuid.NewString(), input)
		require.NoError(t, err)
		require.True(t, completed.Record.Result.ReadyForUpload, "%v", completed.Record.Result.Issues)
		_, rules, err := (readyRules{}).ReadTargetRules(ctx, scope, input.StoreID, input.Draft)
		require.NoError(t, err)
		inventory, err := (readyAssets{}).GetApprovedInventory(ctx, asset.InventoryScope{TenantID: scope.OrganizationID, ProductKey: productKey, TargetPlatform: "shein", SourceSnapshotVersion: 1})
		require.NoError(t, err)
		observations := []goods.OfficialImageObservation{}
		for i, item := range inventory.Assets {
			observations = append(observations, goods.OfficialImageObservation{AssetID: item.ID, SourceURL: item.URL, Width: 900, Height: 900, Type: input.Draft.Images[i].Type, RemoteURL: "https://img.shein.com/" + item.ID + ".jpg", ContentHash: collection.Digest("bounded image bytes"), Bytes: 1000, MediaType: "image/jpeg", ResponseHash: collection.Digest("confirmed image response")})
		}
		wire := goods.BuildOfficial(input.Draft, rules, inventory, observations)
		require.NotEmpty(t, wire.SubmissionPayload)
		sourceProof, err := selector.SelectForExecution(ctx, scope, input.SourceID)
		require.NoError(t, err)
		intentKey := uuid.NewString()
		intentProof, err := submission.PrepareOfficialIntent(ctx, sourceProof, completed.Record, intentKey, wire.SubmissionPayload, nil)
		require.NoError(t, err)
		require.NoError(t, submissionstore.InstallSchema(db))
		_, err = submissionstore.NewOfficialRepository(ctx, db)
		require.ErrorIs(t, err, submission.ErrExecutionUnavailable)
		require.NoError(t, submissionstore.InstallOfficialSchema(db))
		official, err := submissionstore.NewOfficialRepository(ctx, db)
		require.NoError(t, err)
		intent, err := official.PrepareOfficial(ctx, intentProof)
		require.NoError(t, err)
		replay, err := official.PrepareOfficial(ctx, intentProof)
		require.NoError(t, err)
		require.Equal(t, intent, replay)
		core, err := submissionstore.NewOfficialEffectsRepository(db)
		require.NoError(t, err)
		kernel, err := submission.NewExecutionKernel(core)
		require.NoError(t, err)
		command := submission.AcquireExecutionCommand{Scope: submission.ExecutionScope{OrganizationID: scope.OrganizationID}, IntentKey: intent.Key, Target: intent.Target, Action: submission.OfficialPublishAction, Payload: wire.SubmissionPayload, ClaimOwnerID: "worker-a", Lease: time.Minute}
		acquired, err := kernel.Acquire(ctx, command)
		require.NoError(t, err)
		require.NotNil(t, acquired.Permit)
		readAttempt, err := kernel.ReadIntent(ctx, command.Scope, intent.Key)
		require.NoError(t, err)
		require.Equal(t, acquired.Attempt, readAttempt)
		_, err = kernel.ReadIntent(ctx, submission.ExecutionScope{OrganizationID: "org-b"}, intent.Key)
		require.ErrorIs(t, err, submission.ErrExecutionNotFound)
		claim := submission.ExecutionClaim{Scope: command.Scope, AttemptID: acquired.Attempt.AttemptID, FenceEpoch: acquired.Permit.FenceEpoch, OwnerID: acquired.Permit.ClaimOwnerID, Token: acquired.Permit.ClaimToken}
		result := model.PublishResult{SPUName: "spu-a", SKCs: []model.PublishedSKC{{SKCName: "skc-a", SKUs: []model.PublishedSKU{{SupplierSKU: "merchant-sku", SKUCode: "code-a"}}}}, ResponseHash: collection.Digest("full matched official response")}
		proof, err := submission.NewPublishCompletion(scope, completed.Record.ID, productKey, completed.Record.Merchant, acquired.Attempt, claim, wire.Product, result)
		require.NoError(t, err)
		require.NoError(t, db.Exec("CREATE FUNCTION skip_official_receipt() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NULL; END $$").Error)
		require.NoError(t, db.Exec("CREATE TRIGGER skip_receipt BEFORE INSERT ON listing_submission_official_receipts FOR EACH ROW EXECUTE FUNCTION skip_official_receipt()").Error)
		_, err = official.CompleteOfficial(ctx, proof)
		require.ErrorIs(t, err, submission.ErrExecutionUnavailable)
		actual, err := kernel.Get(ctx, command.Scope, acquired.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionClaimed, actual.Status, "a failed receipt insert rolls back kernel success and target release together")
		require.NoError(t, db.Exec("DROP TRIGGER skip_receipt ON listing_submission_official_receipts").Error)
		require.NoError(t, db.Exec("DROP FUNCTION skip_official_receipt()").Error)
		saved, err := official.CompleteOfficial(ctx, proof)
		require.NoError(t, err)
		require.Equal(t, "code-a", saved.Product.SKCs[0].SKUs[0].SKUCode)
		actual, err = kernel.Get(ctx, command.Scope, acquired.Attempt.AttemptID)
		require.NoError(t, err)
		require.Equal(t, submission.ExecutionSucceeded, actual.Status)
		require.Equal(t, saved.ID, actual.Evidence.Reference)
		retained, err := official.ReadOfficial(ctx, scope, saved.ID)
		require.NoError(t, err)
		require.Equal(t, saved, retained)
		_, err = official.ReadOfficial(ctx, collection.Scope{scope.OrganizationID, scope.ActorID, "replacement-member"}, saved.ID)
		require.ErrorIs(t, err, submission.ErrExecutionNotFound)
		command.IntentKey = uuid.NewString()
		blocked, err := kernel.Acquire(ctx, command)
		require.ErrorIs(t, err, submission.ErrExecutionTargetSucceeded, "another batch or record cannot publish a second product into the same confirmed channel")
		require.Nil(t, blocked.Permit)
		for _, loseResponse := range []bool{false, true} {
			input.StoreID = uuid.NewString()
			local, err := ready.CreateForExecution(ctx, scope, uuid.NewString(), input)
			require.NoError(t, err)
			merchant := &uploadExecutionMerchant{binding: local.Record.Merchant, loseResponse: loseResponse}
			uploader, err := supplyapp.NewUploadService(supplyapp.UploadDependencies{
				Sources: selector, Products: record.OriginalTargetProduct{}, Assets: readyAssets{},
				Rules: uploadExecutionRules{local.Record.Merchant, rules}, Records: repository, Authorization: execution,
				Stores: uploadExecutionStore{merchant}, Images: uploadExecutionProbe{}, Kernel: kernel, Intents: official, Receipts: official,
			})
			require.NoError(t, err)
			key := uuid.NewString()
			result, err := uploader.Upload(ctx, scope, key, local.Record.ID)
			require.NoError(t, err)
			want := submission.ExecutionSucceeded
			if loseResponse {
				want = submission.ExecutionOutcomeUnknown
			}
			require.Equal(t, want, result.Status)
			require.Equal(t, 3, merchant.images)
			require.Equal(t, 1, merchant.products)
			for _, retryKey := range []string{key, uuid.NewString()} {
				replay, err := uploader.Upload(ctx, scope, retryKey, local.Record.ID)
				require.NoError(t, err)
				require.Equal(t, want, replay.Status)
				require.Equal(t, 1, merchant.products)
				require.Equal(t, 3, merchant.images)
			}
		}
	})
}

type recordTestImages struct{}

func (recordTestImages) Probe(_ context.Context, a asset.ApprovedAsset, typ int) (goods.OfficialImageObservation, error) {
	return goods.OfficialImageObservation{AssetID: a.ID, SourceURL: a.URL, Width: 900, Height: 900, Type: typ, ContentHash: collection.Digest(a.ID), Bytes: 1000, MediaType: "image/jpeg"}, nil
}
