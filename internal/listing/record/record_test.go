package record

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	listingtask "task-processor/internal/listing/task"
	sheinvalidator "task-processor/internal/marketplace/shein/validator"
	contract "task-processor/internal/marketplace/validator"
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"

	"github.com/stretchr/testify/require"
)

const fixtureStoreID = "11111111-1111-4111-8111-111111111111"

type authFixture struct{ read, write bool }

func (a authFixture) Authorize(_ string, _ []string, permission string) bool {
	if permission == authz.PermissionListingKitAdminRead {
		return a.read
	}
	if permission == authz.PermissionListingKitAdminWrite {
		return a.write
	}
	return false
}
func (authFixture) IsTenantAdmin(string, []string) bool { return false }

type sourceFixture struct {
	calls  int
	change bool
	cancel context.CancelFunc
	title  string
}

func (s *sourceFixture) GetSnapshot(_ context.Context, id catalog.SnapshotIdentity, version uint64) (catalog.PublishedSnapshot, error) {
	s.calls++
	if s.change {
		id.TenantID = "foreign"
	}
	if s.cancel != nil {
		s.cancel()
	}
	title := s.title
	if title == "" {
		title = "draft"
	}
	return catalog.PublishedSnapshot{Identity: id, Version: version, Snapshot: catalog.ProductSnapshot{Title: title}}, nil
}

type assetFixture struct {
	calls  int
	change bool
	miss   bool
	cancel context.CancelFunc
	assets []productasset.ApprovedAsset
}

func (a *assetFixture) GetApprovedInventory(_ context.Context, scope productasset.InventoryScope) (productasset.ApprovedAssetInventory, error) {
	a.calls++
	if a.cancel != nil {
		a.cancel()
	}
	if a.miss {
		return productasset.ApprovedAssetInventory{}, productasset.ErrApprovedAssetsNotReady
	}
	if a.change {
		scope.TenantID = "foreign"
	}
	assets := a.assets
	if assets == nil {
		assets = []productasset.ApprovedAsset{{
			ID: "approved-main", RunID: "run-1", PlanRevision: 1, SlotID: "main", Attempt: 1,
			Role: productasset.RoleMain, URL: "https://example.test/main.jpg",
		}}
	}
	return productasset.ApprovedAssetInventory{Scope: scope, Assets: assets}, nil
}

type storeReferenceFixture struct {
	calls  int
	change bool
	cancel context.CancelFunc
}

func (s *storeReferenceFixture) GetStoreReference(_ context.Context, organizationID, storeID string) (StoreReference, error) {
	s.calls++
	if s.cancel != nil {
		s.cancel()
	}
	if s.change {
		organizationID = "foreign"
	}
	return StoreReference{OrganizationID: organizationID, StoreID: storeID, Platform: "shein"}, nil
}

type storeFixture struct {
	saved  Record
	calls  int
	writes int
	cancel context.CancelFunc
}

func (s *storeFixture) FindOperation(_ context.Context, _ listingtask.Actor, _ string) (Record, error) {
	if s.saved.ID == "" {
		return Record{}, ErrNotFound
	}
	return s.saved.Clone(), nil
}

func (s *storeFixture) Insert(_ context.Context, prepared Prepared) (Record, error) {
	s.calls++
	if s.cancel != nil {
		s.cancel()
	}
	proposed := prepared.Record()
	if s.saved.ID != "" {
		if s.saved.InputHash != proposed.InputHash {
			return Record{}, ErrConflict
		}
		return s.saved.Clone(), nil
	}
	s.writes++
	s.saved = proposed
	return s.saved.Clone(), nil
}

type builderFixture struct {
	calls  int
	fail   error
	cancel context.CancelFunc
	raw    []byte
}

func (b *builderFixture) Build(_ context.Context, _ catalog.ProductSnapshot, _ productasset.ApprovedAssetInventory, _ Input) ([]byte, error) {
	b.calls++
	if b.cancel != nil {
		b.cancel()
	}
	raw := b.raw
	if raw == nil {
		raw = []byte(`{"spu_name":"draft"}`)
	}
	return raw, b.fail
}

type evaluatorFixture struct {
	calls int
	fail  error
}

func (e *evaluatorFixture) Validate(request contract.BoundRequest[[]byte]) (contract.DiagnosticResult, error) {
	e.calls++
	if e.fail != nil {
		return contract.DiagnosticResult{}, e.fail
	}
	return (sheinvalidator.ExactApprovedAssetValidator{}).Validate(request)
}

func fixtureContext() context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID: "B", EffectiveOrganizationID: "B", HomeOrganizationID: "A", UserID: "actor", Roles: []string{"role"}, TokenExpiresAt: time.Now().Add(time.Minute),
	})
}

var fixtureInput = Input{ProductKey: "product", SnapshotVersion: 1, StoreID: fixtureStoreID, Country: "US", Language: "en", Action: contract.SaveDraft}

func fixtureService(source *sourceFixture, assets *assetFixture, stores *storeReferenceFixture, records *storeFixture, builder *builderFixture, evaluator *evaluatorFixture, auth authFixture) (*Service, error) {
	return NewService(ServiceDependencies{
		Products: source, Assets: assets, Stores: stores, Records: records, Builder: builder, Evaluator: evaluator,
		Authorizer: auth, Now: time.Now, RuleRevision: sheinvalidator.DiagnosticRuleVersion, PolicyRevision: sheinvalidator.BindingVersion,
	})
}

func TestCreateChecksWritePermissionBeforeAnyDomainRead(t *testing.T) {
	for _, auth := range []authFixture{{read: true}, {read: false}} {
		source, assets, stores := &sourceFixture{}, &assetFixture{}, &storeReferenceFixture{}
		records, builder, evaluator := &storeFixture{}, &builderFixture{}, &evaluatorFixture{}
		service, err := fixtureService(source, assets, stores, records, builder, evaluator, auth)
		require.NoError(t, err)
		_, err = service.Create(fixtureContext(), "op", fixtureInput)
		require.ErrorIs(t, err, ErrForbidden)
		require.Zero(t, source.calls)
		require.Zero(t, assets.calls)
		require.Zero(t, stores.calls)
		require.Zero(t, records.writes)
	}
}

func TestCreateExactInputsAndCancellationNeverWrite(t *testing.T) {
	for _, phase := range []string{"store identity", "store cancel", "source identity", "source cancel", "asset identity", "asset miss", "asset cancel", "builder cancel", "encoding failure", "evaluation failure", "already cancelled"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(fixtureContext())
			defer cancel()
			source, assets, stores := &sourceFixture{}, &assetFixture{}, &storeReferenceFixture{}
			records, builder, evaluator := &storeFixture{}, &builderFixture{}, &evaluatorFixture{}
			switch phase {
			case "store identity":
				stores.change = true
			case "store cancel":
				stores.cancel = cancel
			case "source identity":
				source.change = true
			case "source cancel":
				source.cancel = cancel
			case "asset identity":
				assets.change = true
			case "asset miss":
				assets.miss = true
			case "asset cancel":
				assets.cancel = cancel
			case "builder cancel":
				builder.cancel = cancel
			case "encoding failure":
				builder.fail = errors.New("cannot encode")
			case "evaluation failure":
				evaluator.fail = errors.New("cannot evaluate")
			case "already cancelled":
				cancel()
			}
			service, err := fixtureService(source, assets, stores, records, builder, evaluator, authFixture{write: true})
			require.NoError(t, err)
			_, err = service.Create(ctx, "op", fixtureInput)
			require.Error(t, err)
			require.Zero(t, records.writes)
		})
	}
}

func TestCreateReplayReturnsPersistedRecordAndChangedInputConflicts(t *testing.T) {
	source, assets, stores := &sourceFixture{}, &assetFixture{}, &storeReferenceFixture{}
	records, builder, evaluator := &storeFixture{}, &builderFixture{}, &evaluatorFixture{}
	service, err := fixtureService(source, assets, stores, records, builder, evaluator, authFixture{write: true})
	require.NoError(t, err)
	first, err := service.Create(fixtureContext(), "op", fixtureInput)
	require.NoError(t, err)
	second, err := service.Create(fixtureContext(), "op", fixtureInput)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 2, builder.calls)
	require.Equal(t, 2, evaluator.calls)
	require.Equal(t, 1, records.writes)

	changed := fixtureInput
	changed.Action = contract.Publish
	_, err = service.Create(fixtureContext(), "op", changed)
	require.ErrorIs(t, err, ErrConflict)
	require.Equal(t, 1, records.writes)
}

func TestCreateSameOperationConflictsWhenBoundProductOrAssetFactDrifts(t *testing.T) {
	for _, drift := range []string{"product", "asset"} {
		t.Run(drift, func(t *testing.T) {
			source, assets, stores := &sourceFixture{}, &assetFixture{}, &storeReferenceFixture{}
			records, builder, evaluator := &storeFixture{}, &builderFixture{}, &evaluatorFixture{}
			service, err := fixtureService(source, assets, stores, records, builder, evaluator, authFixture{write: true})
			require.NoError(t, err)
			_, err = service.Create(fixtureContext(), "fact-drift", fixtureInput)
			require.NoError(t, err)
			if drift == "product" {
				source.title = "changed immutable product fact"
			} else {
				assets.assets = []productasset.ApprovedAsset{{ID: "approved-main", RunID: "run-1", PlanRevision: 1, SlotID: "main", Attempt: 1, Role: productasset.RoleMain, URL: "https://example.test/changed.jpg"}}
			}
			_, err = service.Create(fixtureContext(), "fact-drift", fixtureInput)
			require.ErrorIs(t, err, ErrConflict)
			require.Equal(t, 1, records.writes)
		})
	}
}

func TestCreateBoundsProductAssetInventoryAndGeneratedPackageIndependently(t *testing.T) {
	largeAssets := make([]productasset.ApprovedAsset, 16000)
	for index := range largeAssets {
		identity := fmt.Sprintf("asset-%05d-%s", index, strings.Repeat("x", 80))
		largeAssets[index] = productasset.ApprovedAsset{ID: identity, RunID: "run", PlanRevision: 1, SlotID: identity, Attempt: 1, Role: productasset.RoleGallery, URL: "https://controlled.invalid/" + identity}
	}
	for _, phase := range []string{"product", "assets", "package"} {
		t.Run(phase, func(t *testing.T) {
			source, assets, stores := &sourceFixture{}, &assetFixture{}, &storeReferenceFixture{}
			records, builder, evaluator := &storeFixture{}, &builderFixture{}, &evaluatorFixture{}
			switch phase {
			case "product":
				source.title = strings.Repeat("p", MaxPayloadBytes)
			case "assets":
				assets.assets = largeAssets
			case "package":
				builder.raw = []byte(strings.Repeat("x", MaxPayloadBytes+1))
			}
			service, err := fixtureService(source, assets, stores, records, builder, evaluator, authFixture{write: true})
			require.NoError(t, err)
			_, err = service.Create(fixtureContext(), "bounded-"+phase, fixtureInput)
			require.ErrorIs(t, err, ErrTooLarge)
			require.Zero(t, records.writes)
		})
	}
}

func TestInputRequiresExactStoreAndAction(t *testing.T) {
	for _, mutate := range []func(*Input){
		func(input *Input) { input.StoreID = "" },
		func(input *Input) { input.StoreID = "11111111-1111-4111-8111-11111111111A" },
		func(input *Input) { input.Action = "" },
		func(input *Input) { input.Action = contract.Preview },
	} {
		input := fixtureInput
		mutate(&input)
		require.ErrorIs(t, input.Validate(), ErrInvalid)
	}
	require.NoError(t, fixtureInput.Validate())
}
