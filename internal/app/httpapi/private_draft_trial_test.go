package httpapi

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	podhttp "task-processor/internal/app/pod/httpapi"
	supplyhttp "task-processor/internal/app/supplychain/httpapi"
	"task-processor/internal/httproute"
	"task-processor/internal/listing/preparation"
	record "task-processor/internal/listing/record/target"
	"task-processor/internal/product/collection"
	markethttp "task-processor/internal/product/supplymarket/httpapi"
	"task-processor/internal/storecenter"
	"testing"
	"time"
)

func TestPrivateDraftTrialOnlyAdmitsSelectionReads(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	var supplied currentApplicationOptions
	WithPrivateDraftTrial(PrivateDraftTrialDependencies{})(&supplied)
	fullSupply, trial := supplied.supplyRouteFeatures()
	reads := supplyhttp.PrivateDraftReadRoutes(nil, nil)
	require.Len(t, reads, 7)
	check := func(routes []httproute.Descriptor) error {
		return validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{PrivateDraftTrial: trial, SupplyChain: fullSupply})
	}
	require.NoError(t, check(append(append([]httproute.Descriptor{}, base...), reads...)))
	for _, r := range supplyhttp.SupplyRoutes(nil, nil) {
		if !supplyhttp.IsPrivateDraftReadRoute(r.Method, r.Path) {
			require.Error(t, check(append(append([]httproute.Descriptor{}, base...), r)))
		}
	}
	for i := range reads {
		changed := append([]httproute.Descriptor{}, reads...)
		changed[i].Permission = ""
		require.Error(t, check(append(append([]httproute.Descriptor{}, base...), changed...)))
	}
}

func TestPrivateDraftTrialDoesNotInstallExecutionWorker(t *testing.T) {
	var supplied currentApplicationOptions
	WithPrivateDraftTrial(PrivateDraftTrialDependencies{})(&supplied)
	require.NotPanics(t, func() { supplied.installSupplyWorker(&supplyChainModule{}) })
}

func TestPrivateDraftTrialDoesNotAdmitMarketPODServices(t *testing.T) {
	var base []httproute.Descriptor
	for _, r := range currentWorkbenchApplicationRoutes {
		base = append(base, httproute.Descriptor{Method: r.Method, Path: r.Path})
	}
	for _, pod := range []bool{false, true} {
		routes := append(append([]httproute.Descriptor{}, base...), supplyhttp.PrivateDraftReadRoutes(nil, nil)...)
		routes = append(routes, markethttp.Routes(nil, nil, nil)...)
		if pod {
			routes = append(routes, podhttp.Routes(nil, nil)...)
		}
		require.Error(t, validateCurrentApplicationRoutesInternal(routes, false, false, false, false, false, false, false, currentApplicationOptionalRoutes{PrivateDraftTrial: true, SupplyMarket: true, POD: pod}))
	}
}

type trialTestAuthorizer struct {
	scope collection.Scope
	err   error
}

func (a trialTestAuthorizer) Authorize(context.Context, string) (collection.Scope, error) {
	return a.scope, a.err
}

type trialTestStores struct {
	storecenter.Repository
	store *storecenter.Store
	err   error
}

func (s *trialTestStores) Get(context.Context, string, string) (*storecenter.Store, error) {
	return s.store, s.err
}

func TestPrivateDraftTrialRejectsScopeRevocationAndNonTestRecords(t *testing.T) {
	ctx := context.Background()
	scope := collection.Scope{OrganizationID: "test-org", ActorID: "actor", MemberID: "member"}
	id := "11111111-1111-4111-8111-111111111111"
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: id, OrganizationID: scope.OrganizationID, ActorSubject: scope.ActorID, Name: "离线测试店铺", Platform: "shein", Region: "US", CreateIdempotencyKey: id, OccurredAt: time.Now()})
	require.NoError(t, err)
	repo := &trialTestStores{store: store}
	adapter := privateDraftTrialStore{scope, id, repo}
	merchant, err := adapter.RulesMerchant(ctx, scope, id, nil)
	require.NoError(t, err)
	require.NoError(t, adapter.validateRecord(ctx, scope, record.TargetRecord{Merchant: merchant.Binding()}))
	_, err = merchant.Categories(ctx)
	require.Error(t, err, "offline cannot query official rules")
	for _, changed := range []collection.Scope{{OrganizationID: "other", ActorID: scope.ActorID, MemberID: scope.MemberID}, {OrganizationID: scope.OrganizationID, ActorID: "other", MemberID: scope.MemberID}, {OrganizationID: scope.OrganizationID, ActorID: scope.ActorID, MemberID: "replacement"}} {
		_, err = (privateDraftTrialAuthorizer{trialTestAuthorizer{changed, nil}, scope}).Authorize(ctx, preparation.PermissionRead)
		require.Error(t, err)
		_, err = adapter.RulesMerchant(ctx, changed, id, nil)
		require.Error(t, err)
	}
	_, err = (privateDraftTrialAuthorizer{trialTestAuthorizer{scope, errors.New("revoked")}, scope}).Authorize(ctx, preparation.PermissionRead)
	require.Error(t, err)
	_, err = (privateDraftTrialAuthorizer{trialTestAuthorizer{scope, nil}, scope}).Authorize(ctx, preparation.PermissionManage)
	require.Error(t, err)
	bad := merchant.Binding()
	bad.ApplicationID = "real-app"
	require.Error(t, adapter.validateRecord(ctx, scope, record.TargetRecord{Merchant: bad}))
	repo.err = storecenter.ErrNotFound
	require.Error(t, adapter.validateRecord(ctx, scope, record.TargetRecord{Merchant: merchant.Binding()}))
	repo.err = nil
	snapshot := store.Snapshot()
	snapshot.ConnectionRef = "real-connection"
	repo.store, err = storecenter.RehydrateStore(snapshot)
	require.NoError(t, err)
	_, err = adapter.RulesMerchant(ctx, scope, id, nil)
	require.Error(t, err)
}
