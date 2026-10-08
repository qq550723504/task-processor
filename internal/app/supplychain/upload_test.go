package supplychainapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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

type uploadAuth struct {
	scope  collection.Scope
	denied bool
}

func (a *uploadAuth) Authorize(context.Context, string) (collection.Scope, error) {
	return a.scope, nil
}
func (a *uploadAuth) AuthorizeExecution(_ context.Context, scope collection.Scope, _ string) error {
	if a.denied || scope != a.scope {
		return preparation.ErrForbidden
	}
	return nil
}

type uploadCollections struct{ collection.Repository }
type uploadUnusedSources struct {
	sourcing.PublishedAcquisitionReader
}
type uploadSources struct {
	preparation.Repository
	source preparation.SourceItem
}

func (s uploadSources) ReadRetainedSource(_ context.Context, scope collection.Scope, id string) (preparation.SourceItem, error) {
	if id != s.source.ID || scope.ActorID != "actor-a" {
		return preparation.SourceItem{}, preparation.ErrNotFound
	}
	return s.source, nil
}

type uploadCatalog struct{ snapshot catalog.PublishedSnapshot }

func (c uploadCatalog) GetSnapshot(context.Context, catalog.SnapshotIdentity, uint64) (catalog.PublishedSnapshot, error) {
	return c.snapshot, nil
}

type uploadRecords struct {
	record.TargetRepository
	saved record.TargetRecord
}

func (r *uploadRecords) ReadTargetRecord(_ context.Context, _ collection.Scope, id string) (record.TargetRecord, error) {
	if id != r.saved.ID {
		return record.TargetRecord{}, record.ErrNotFound
	}
	return r.saved, nil
}
func (r *uploadRecords) ReadTargetHead(context.Context, collection.Scope, string) (record.TargetRecord, error) {
	return r.saved, nil
}

type uploadAssets struct{ inventory asset.ApprovedAssetInventory }

func (a uploadAssets) GetApprovedInventory(context.Context, asset.InventoryScope) (asset.ApprovedAssetInventory, error) {
	return a.inventory, nil
}

type uploadRules struct {
	binding  storecenter.ProductMerchantBinding
	snapshot goods.OfficialRuleSnapshot
}

func (r uploadRules) ReadTargetRules(context.Context, collection.Scope, string, goods.OfficialDraftInput) (storecenter.ProductMerchantBinding, goods.OfficialRuleSnapshot, error) {
	return r.binding, r.snapshot, nil
}

type uploadProbe struct{}

func (uploadProbe) Probe(_ context.Context, a asset.ApprovedAsset, typ int) (goods.OfficialImageObservation, error) {
	return goods.OfficialImageObservation{AssetID: a.ID, SourceURL: a.URL, Width: 900, Height: 900, Type: typ, ContentHash: collection.Digest(a.ID), Bytes: 1000, MediaType: "image/jpeg"}, nil
}

type uploadMerchant struct {
	binding               storecenter.ProductMerchantBinding
	transforms, publishes int
	loseResponse          bool
	denySend              bool
}

func (m *uploadMerchant) Binding() storecenter.ProductMerchantBinding { return m.binding }
func (m *uploadMerchant) PublishPermission(context.Context, string) (model.PublishPermission, error) {
	return model.PublishPermission{Allowed: true}, nil
}
func (m *uploadMerchant) TransformImage(_ context.Context, input model.TransformImage) (model.TransformedImage, error) {
	if m.denySend {
		return model.TransformedImage{}, record.ErrForbidden
	}
	m.transforms++
	return model.TransformedImage{Original: input.OriginalURL, Transformed: "https://img.shein.com/" + collection.Digest(input) + ".jpg", ResponseHash: collection.Digest(input)}, nil
}
func (m *uploadMerchant) Publish(_ context.Context, input model.PublishProduct) (model.PublishResult, error) {
	if m.denySend {
		return model.PublishResult{}, record.ErrForbidden
	}
	m.publishes++
	if m.loseResponse {
		return model.PublishResult{}, errors.New("response lost")
	}
	return model.PublishResult{SPUName: "spu-a", SKCs: []model.PublishedSKC{{SKCName: "skc-a", SKUs: []model.PublishedSKU{{SupplierSKU: input.SKCs[0].SKUs[0].SupplierSKU, SKUCode: "sku-code-a"}}}}, ResponseHash: collection.Digest(input)}, nil
}

type uploadStore struct {
	merchant *uploadMerchant
	auth     *uploadAuth
}

func (s uploadStore) ExecutionMerchant(_ context.Context, scope collection.Scope, _ string, _ string, expected *storecenter.ProductMerchantBinding) (ExecutionMerchant, error) {
	if s.auth.denied || scope != s.auth.scope || expected != nil && *expected != s.merchant.binding {
		return nil, record.ErrForbidden
	}
	return s.merchant, nil
}

type uploadKernel struct {
	attempts  map[string]submission.ExecutionAttempt
	onAcquire func(submission.AcquireExecutionCommand)
}

func (k *uploadKernel) Acquire(_ context.Context, c submission.AcquireExecutionCommand) (submission.ExecutionAcquisition, error) {
	if a, ok := k.attempts[c.IntentKey]; ok {
		return submission.ExecutionAcquisition{Attempt: a, Replayed: true}, nil
	}
	for _, a := range k.attempts {
		if a.Target == c.Target {
			if a.Status == submission.ExecutionSucceeded {
				return submission.ExecutionAcquisition{}, submission.ErrExecutionTargetSucceeded
			}
			if a.Status == submission.ExecutionClaimed || a.Status == submission.ExecutionOutcomeUnknown {
				return submission.ExecutionAcquisition{}, submission.ErrExecutionTargetClaimed
			}
		}
	}
	id, err := uuid.NewV7()
	if err != nil {
		return submission.ExecutionAcquisition{}, err
	}
	r, err := submission.NewExecutionReservation(c, id.String(), "private-claim-token", time.Now())
	if err != nil {
		return submission.ExecutionAcquisition{}, err
	}
	r.Attempt.FenceEpoch = 1
	k.attempts[c.IntentKey] = r.Attempt
	if k.onAcquire != nil {
		k.onAcquire(c)
	}
	return submission.ExecutionAcquisition{Attempt: r.Attempt, Permit: &submission.SendPermit{AttemptID: id.String(), FenceEpoch: 1, ClaimOwnerID: c.ClaimOwnerID, ClaimToken: "private-claim-token", LeaseExpiresAt: r.Attempt.LeaseExpiresAt}}, nil
}
func (k *uploadKernel) ReadIntent(_ context.Context, _ submission.ExecutionScope, key string) (submission.ExecutionAttempt, error) {
	a, ok := k.attempts[key]
	if !ok {
		return a, submission.ErrExecutionNotFound
	}
	return a, nil
}
func (k *uploadKernel) MarkUnknown(_ context.Context, claim submission.ExecutionClaim, reason submission.UnknownReason) (submission.ExecutionAttempt, error) {
	for key, a := range k.attempts {
		if a.AttemptID == claim.AttemptID {
			a, err := submission.TransitionExecutionToUnknown(a, reason, time.Now())
			if err != nil {
				return a, err
			}
			k.attempts[key] = a
			return a, nil
		}
	}
	return submission.ExecutionAttempt{}, submission.ErrExecutionNotFound
}
func (k *uploadKernel) Expire(_ context.Context, _ submission.ExecutionScope, id string) (submission.ExecutionAttempt, error) {
	for key, a := range k.attempts {
		if a.AttemptID == id {
			if time.Now().Before(a.LeaseExpiresAt) {
				return a, nil
			}
			a, err := submission.TransitionExecutionToUnknown(a, submission.UnknownLeaseExpired, time.Now())
			if err != nil {
				return a, err
			}
			k.attempts[key] = a
			return a, nil
		}
	}
	return submission.ExecutionAttempt{}, submission.ErrExecutionNotFound
}

type uploadOfficial struct {
	intents    map[string]submission.OfficialIntent
	receipts   map[string]submission.OfficialReceipt
	kernel     *uploadKernel
	failCommit bool
}

func (r *uploadOfficial) PrepareOfficial(ctx context.Context, proof submission.OfficialIntentCommit) (submission.OfficialIntent, error) {
	v, err := proof.Read(ctx)
	if err != nil {
		return v, err
	}
	if prior, ok := r.intents[v.Key]; ok {
		if prior.InputHash != v.InputHash {
			return v, submission.ErrExecutionIntentConflict
		}
		return prior, nil
	}
	r.intents[v.Key] = v
	return v, nil
}
func (r *uploadOfficial) ReadOfficialIntent(_ context.Context, scope collection.Scope, key string) (submission.OfficialIntent, error) {
	v, ok := r.intents[key]
	if !ok || v.Owner != scope {
		return v, submission.ErrExecutionNotFound
	}
	return v, nil
}
func (r *uploadOfficial) CompleteOfficial(ctx context.Context, proof submission.OfficialCompletion) (submission.OfficialReceipt, error) {
	_, v, err := proof.Read(ctx)
	if err != nil {
		return v, err
	}
	if r.failCommit {
		return v, submission.ErrExecutionUnavailable
	}
	r.receipts[v.ID] = v
	a := r.kernel.attempts[v.IntentKey]
	a.Status = submission.ExecutionSucceeded
	r.kernel.attempts[v.IntentKey] = a
	return v, nil
}
func (r *uploadOfficial) ReadOfficial(_ context.Context, scope collection.Scope, id string) (submission.OfficialReceipt, error) {
	v, ok := r.receipts[id]
	if !ok || v.Owner != scope {
		return v, submission.ErrExecutionNotFound
	}
	return v, nil
}
func (r *uploadOfficial) FindOfficialTarget(_ context.Context, org string, target submission.ExecutionTarget) (submission.OfficialReceipt, error) {
	for _, v := range r.receipts {
		if v.Owner.OrganizationID == org && v.Target == target {
			return v, nil
		}
	}
	return submission.OfficialReceipt{}, submission.ErrExecutionNotFound
}
func uploadPointer[T any](v T) *T { return &v }
func uploadFixture(t *testing.T) (*UploadService, collection.Scope, *uploadRecords, *uploadMerchant, *uploadKernel, *uploadOfficial, *uploadAuth) {
	t.Helper()
	scope := collection.Scope{"org-a", "actor-a", "member-a"}
	auth := &uploadAuth{scope: scope}
	source := preparation.SourceItem{ID: uuid.NewString(), PreparationID: uuid.NewString(), CollectionItemID: uuid.NewString(), CollectionRevision: 1, Source: collection.Source{ProductKey: "product-a", PublicationID: uuid.NewString(), Version: 1, Kind: "own"}}
	snapshot := catalog.PublishedSnapshot{Identity: catalog.SnapshotIdentity{TenantID: scope.OrganizationID, ProductKey: "product-a"}, PublicationID: source.Source.PublicationID, Version: 1, Snapshot: catalog.ProductSnapshot{Title: "Original"}}
	collections, err := collection.NewService(uploadCollections{}, auth, uploadUnusedSources{})
	require.NoError(t, err)
	sources := uploadSources{source: source}
	preparations, err := preparation.NewService(sources, collections, auth)
	require.NoError(t, err)
	selector, err := preparation.NewSourceSelector(preparations, collections, sources, uploadCatalog{snapshot})
	require.NoError(t, err)
	selector, err = selector.WithExecution(auth, collection.ExecutionOwnerAuthority{Authorization: auth})
	require.NoError(t, err)
	binding := storecenter.ProductMerchantBinding{OrganizationID: scope.OrganizationID, StoreID: uuid.NewString(), Site: "shein-us", StoreVersion: 1, ConnectionRevision: 1, ApplicationRevision: "v1:self_operated", ApplicationID: "app-a", ApplicationType: storecenter.ApplicationSelfOperated, SupplierIdentityHash: collection.Digest("merchant-a"), ServiceExpiresAt: time.Now().Add(time.Hour)}
	binding.ServiceExpiresAt = binding.ServiceExpiresAt.UTC().Truncate(time.Microsecond)
	rules := goods.OfficialRuleSnapshot{ApplicationMode: model.ModeSelfOperated, Warehouses: []model.Warehouse{},
		Categories: []model.Category{{ID: 123, ProductTypeID: 456, Leaf: uploadPointer(true)}}, Sites: []model.MainSite{{ID: "shein", Sites: []model.Site{{Abbreviation: "shein-us", Status: uploadPointer(1), StoreType: uploadPointer(2), Currency: "USD"}}}},
		Fill:       model.FillStandards{DefaultLanguage: "en", DefaultTitleMaximum: uploadPointer(150), SupplierCodeInSPU: uploadPointer(false), Fields: []model.FillRule{}, Pictures: []model.PictureRule{{Field: "switch_spu_picture", Enabled: uploadPointer(false)}, {Field: "sku_image_required", Enabled: uploadPointer(false)}}},
		Attributes: model.AttributeTemplate{ProductTypeID: 456, MainAttributeStatus: uploadPointer(1), Attributes: []model.Attribute{{ID: 12, Name: "Default", Type: uploadPointer(1), Show: uploadPointer(1), MainLabel: uploadPointer(1), Mode: uploadPointer(2), Status: uploadPointer(3), MaximumSelections: uploadPointer(1), Options: []model.AttributeOption{{ID: 34, Name: "Default", Show: uploadPointer(1)}}}}},
		Linked:     []model.LinkedRules{{GroupID: "product", Attributes: []model.LinkedAttributeRule{}}, {GroupID: "sku-0-0", Attributes: []model.LinkedAttributeRule{}}}, Brands: []model.Brand{{Code: "brand-a", Name: "Fixture brand"}},
	}
	inventory := asset.ApprovedAssetInventory{Scope: asset.InventoryScope{TenantID: scope.OrganizationID, ProductKey: "product-a", TargetPlatform: "shein", SourceSnapshotVersion: 1}, Assets: []asset.ApprovedAsset{}}
	for _, id := range []string{"main", "detail", "square"} {
		inventory.Assets = append(inventory.Assets, asset.ApprovedAsset{ID: id, URL: "https://images.example.org/" + id + ".jpg", Width: 900, Height: 900})
	}
	input := record.TargetInput{SourceID: source.ID, StoreID: binding.StoreID, EffectiveVersion: 1, Draft: goods.OfficialDraftInput{Product: model.PublishProduct{CategoryID: 123, BrandCode: "brand-a", Names: []model.LanguageContent{{Language: "en", Name: "Manual title"}}, Attributes: []model.AttributeValue{}}}}
	input.Draft.Product.Descriptions = []model.LanguageContent{{Language: "en", Name: "Human supplied description"}}
	input.Draft.Product.SKCs = []model.ProductSKC{{SupplierCode: "spu-a", SaleAttribute: model.AttributeValue{AttributeID: 12, AttributeValueID: uploadPointer(int64(34))}, SKUs: []model.ProductSKU{{SupplierSKU: "sku-a", Length: "10", Width: "10", Height: "10", Weight: uploadPointer(100.0), MallState: 1, Prices: []model.ProductPrice{{BasePrice: 12.5, Currency: "USD", SubSite: "shein-us"}}, Stock: []model.ProductStock{{Quantity: 5}}, SaleAttributes: []model.AttributeValue{}}}}}
	input.Draft.Images = []goods.OfficialImageSlot{{Group: "skc", AssetID: "main", Type: 1, Sort: 1}, {Group: "skc", AssetID: "detail", Type: 2, Sort: 2}, {Group: "skc", AssetID: "square", Type: 5, Sort: 3}}
	saved := record.TargetRecord{ID: uuid.NewString(), TargetID: record.TargetIdentity(scope, source.ID, binding.StoreID), Revision: 1, Source: source, EffectiveVersion: 1, ProductHash: collection.Digest(snapshot.Snapshot), InventoryHash: collection.Digest(inventory), RulesHash: collection.Digest(rules), Merchant: binding, Input: input, Result: goods.BuildOfficial(input.Draft, rules, inventory, nil), CreatedAt: time.Now().UTC()}
	require.True(t, saved.Result.ReadyForUpload, "%v", saved.Result.Issues)
	records := &uploadRecords{saved: saved}
	merchant := &uploadMerchant{binding: binding}
	kernel := &uploadKernel{attempts: map[string]submission.ExecutionAttempt{}}
	official := &uploadOfficial{intents: map[string]submission.OfficialIntent{}, receipts: map[string]submission.OfficialReceipt{}, kernel: kernel}
	service, err := NewUploadService(UploadDependencies{Sources: selector, Products: record.OriginalTargetProduct{}, Assets: uploadAssets{inventory}, Rules: uploadRules{binding, rules}, Records: records, Authorization: auth, Stores: uploadStore{merchant, auth}, Images: uploadProbe{}, Kernel: kernel, Intents: official, Receipts: official})
	require.NoError(t, err)
	return service, scope, records, merchant, kernel, official, auth
}
func TestOfficialUploadCompletesCorrelatedImagesAndPreservesOneOriginalChannel(t *testing.T) {
	s, scope, records, merchant, _, _, _ := uploadFixture(t)
	ctx := context.Background()
	key := uuid.NewString()
	result, err := s.Upload(ctx, scope, key, records.saved.ID)
	require.NoError(t, err)
	require.Equal(t, submission.ExecutionSucceeded, result.Status)
	require.True(t, result.CurrentRecordUploaded)
	require.Equal(t, "sku-code-a", result.Product.SKCs[0].SKUs[0].SKUCode)
	require.Equal(t, 3, merchant.transforms)
	require.Equal(t, 1, merchant.publishes)
	result, err = s.Upload(ctx, scope, key, records.saved.ID)
	require.NoError(t, err)
	require.True(t, result.CurrentRecordUploaded)
	require.Equal(t, 1, merchant.publishes)
	records.saved.ID = uuid.NewString()
	records.saved.Revision++
	records.saved.Input.Draft.Product.Names[0].Name = "Changed later"
	result, err = s.Upload(ctx, scope, uuid.NewString(), records.saved.ID)
	require.NoError(t, err)
	require.False(t, result.CurrentRecordUploaded, "the original platform product is retained; changed local content was never sent")
	require.Equal(t, 1, merchant.publishes)
	require.Equal(t, 3, merchant.transforms)
}
func TestLostOfficialResponseCannotBeResentByOriginalOrNewOperation(t *testing.T) {
	s, scope, records, merchant, _, _, _ := uploadFixture(t)
	merchant.loseResponse = true
	key := uuid.NewString()
	result, err := s.Upload(context.Background(), scope, key, records.saved.ID)
	require.NoError(t, err)
	require.Equal(t, submission.ExecutionOutcomeUnknown, result.Status)
	require.Equal(t, 1, merchant.publishes)
	result, err = s.Upload(context.Background(), scope, key, records.saved.ID)
	require.NoError(t, err)
	require.Equal(t, submission.ExecutionOutcomeUnknown, result.Status)
	require.Equal(t, 1, merchant.publishes)
	result, err = s.Upload(context.Background(), scope, uuid.NewString(), records.saved.ID)
	require.NoError(t, err)
	require.Equal(t, submission.ExecutionOutcomeUnknown, result.Status)
	require.Equal(t, 1, merchant.publishes, "a different operation key does not bypass the stable product fence")
}

func TestRulesDriftAndRejoinedMembershipStopBeforeOfficialMutation(t *testing.T) {
	s, scope, records, merchant, _, _, _ := uploadFixture(t)
	records.saved.RulesHash = collection.Digest("different approved rules")
	_, err := s.Upload(context.Background(), scope, uuid.NewString(), records.saved.ID)
	require.ErrorIs(t, err, record.ErrConflict)
	require.Zero(t, merchant.transforms)
	require.Zero(t, merchant.publishes)
	scope.MemberID = "new-membership"
	_, err = s.Upload(context.Background(), scope, uuid.NewString(), records.saved.ID)
	require.ErrorIs(t, err, record.ErrForbidden)
	require.Zero(t, merchant.transforms)
}
func TestPostPermitAuthorizationDriftDoesNotDispatchAndRetainsUnknown(t *testing.T) {
	s, scope, records, merchant, kernel, _, auth := uploadFixture(t)
	kernel.onAcquire = func(c submission.AcquireExecutionCommand) {
		if c.Action == submission.OfficialImageAction {
			auth.denied = true
			merchant.denySend = true
		}
	}
	result, err := s.Upload(context.Background(), scope, uuid.NewString(), records.saved.ID)
	require.NoError(t, err)
	require.Equal(t, submission.ExecutionOutcomeUnknown, result.Status)
	require.Zero(t, merchant.transforms)
	require.Zero(t, merchant.publishes)
}
func TestOfficialSuccessPersistenceFailureRetainsUnknownAndNoSecondMutation(t *testing.T) {
	s, scope, records, merchant, _, official, _ := uploadFixture(t)
	official.failCommit = true
	key := uuid.NewString()
	result, err := s.Upload(context.Background(), scope, key, records.saved.ID)
	require.NoError(t, err)
	require.Equal(t, submission.ExecutionOutcomeUnknown, result.Status)
	require.Equal(t, 1, merchant.transforms)
	require.Zero(t, merchant.publishes)
	_, err = s.Upload(context.Background(), scope, key, records.saved.ID)
	require.NoError(t, err)
	require.Equal(t, 1, merchant.transforms)
}
