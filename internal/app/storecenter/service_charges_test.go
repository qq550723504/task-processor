package storecenterapp

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	resourceadapter "task-processor/internal/integration/orgresource"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/storecenter"
)

type serviceContextAccess struct{}

func (serviceContextAccess) AuthorizeStoreMember(ctx context.Context, org string) (storecenter.StoreMemberAccess, error) {
	id, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || id.EffectiveOrganizationID != org {
		return storecenter.StoreMemberAccess{}, storecenter.ErrNotFound
	}
	a := serviceTestAuthorizer()
	return storecenter.StoreMemberAccess{OrganizationID: org, ActorID: id.UserID, MemberID: id.EffectiveMemberID, Administrator: a.IsTenantAdmin(id.UserID, id.Roles), CanWrite: authz.AllowedOrganization(ctx, a, id.UserID, org, id.Roles, authz.PermissionWorkbenchStoreUpdate)}, nil
}

type delayedStoreSettlement struct{ orgresource.ConsumerChargePort }

func (p delayedStoreSettlement) Reconcile(context.Context, orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	return orgresource.ConsumerChargeReceipt{}, errors.New("synthetic settlement transport failure")
}

func TestDeletedStoreKeepsOriginalPaidProofForResourceRecovery(t *testing.T) {
	f := newServiceChargeFixture(t)
	operation := uuid.NewString()
	_, err := serviceApplication(t, f, delayedStoreSettlement{f.ledger}).Activate(f.member, storecenter.ServiceLifecycleApplicationRequest{StoreID: f.store.ID(), OperationID: operation, ExpectedStoreVersion: 1})
	require.Error(t, err)
	proof, err := f.repo.ReadServiceChargeProof(context.Background(), "org-a", operation)
	require.NoError(t, err)
	require.Equal(t, "succeeded", proof.State)
	_, err = f.repo.DeleteRecord(f.admin, storecenter.DeleteStoreRequest{OrganizationID: "org-a", StoreID: f.store.ID(), ActorSubject: "admin", OperationKey: uuid.NewString(), ExpectedVersion: 2}, time.Now().UTC())
	require.NoError(t, err)
	_, err = f.repo.Get(f.member, "org-a", f.store.ID())
	require.ErrorIs(t, err, storecenter.ErrNotFound)
	n, err := f.ledger.RecoverDue(context.Background())
	require.NoError(t, err)
	require.Equal(t, 1, n)
	position, err := f.positions.ReadPosition(context.Background(), "org-a", "member-a", orgresource.ResourceStoreRenewalPeriod)
	require.NoError(t, err)
	require.Zero(t, position.Reserved)
	require.EqualValues(t, 1, position.Consumed)
	retained, err := f.repo.ReadServiceChargeProof(context.Background(), "org-a", operation)
	require.NoError(t, err)
	require.Equal(t, proof.EvidenceID, retained.EvidenceID)
	require.Equal(t, proof.Snapshot, retained.Snapshot)
	n, err = f.ledger.RecoverDue(context.Background())
	require.NoError(t, err)
	require.Zero(t, n)
}

type connectedStore struct{}

func (connectedStore) Status(context.Context, storecenter.ConnectionStatusInput) (storecenter.ConnectionStatus, error) {
	return storecenter.ConnectionStatusConnected, nil
}
func serviceContext(user, member, role string) context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-a", UserID: user, EffectiveMemberID: member, Roles: []string{role}, OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", AuthorizationID: member, Roles: []string{role}}}})
}

type serviceChargeFixture struct {
	repo          *storecenter.MemberScopedStoreRepository
	ledger        *orgresource.ConsumerChargeService
	positions     *resourceadapter.GormMemberAllocationRepository
	resourceDB    *gorm.DB
	store         *storecenter.Store
	member, admin context.Context
}

func newServiceChargeFixture(t *testing.T) serviceChargeFixture {
	t.Helper()
	open := func(name string) *gorm.DB {
		db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), name)), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
		require.NoError(t, err)
		pool, err := db.DB()
		require.NoError(t, err)
		pool.SetMaxOpenConns(1)
		t.Cleanup(func() { _ = pool.Close() })
		return db
	}
	storeDB, resourceDB := open("store.db"), open("resource.db")
	require.NoError(t, storecenter.AutoMigrateStoreRepository(storeDB))
	require.NoError(t, resourceadapter.AutoMigrate(resourceDB))
	require.NoError(t, resourceDB.Table("saas_organization_resource_buckets").Create(map[string]any{"organization_id": "org-a", "resource_type": "store_renewal_period", "available": 4, "reserved": 0, "consumed": 0, "created_at": time.Now().UTC(), "updated_at": time.Now().UTC()}).Error)
	positions, err := resourceadapter.NewGormMemberAllocationRepository(resourceDB, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	_, err = positions.Transfer(context.Background(), orgresource.MemberResourceTransfer{OrganizationID: "org-a", MemberID: "member-a", ActorID: "admin", OperationID: "allocate", ResourceType: orgresource.ResourceStoreRenewalPeriod, Action: orgresource.MemberResourceAllocate, Quantity: 3})
	require.NoError(t, err)
	repo, err := storecenter.NewMemberScopedStoreRepository(storeDB, serviceContextAccess{})
	require.NoError(t, err)
	member, admin := serviceContext("operator", "member-a", authz.EnterpriseRoleKey("org-a", 1)), serviceContext("admin", "member-admin", "listingkit_admin")
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: uuid.NewString(), OrganizationID: "org-a", ActorSubject: "operator", Name: "Store", Platform: "shein", Region: "SG", ExternalStoreID: "external", CreateIdempotencyKey: uuid.NewString(), OccurredAt: time.Now().UTC().Add(-time.Minute)})
	require.NoError(t, err)
	store, _, err = repo.CreateOrReplay(member, "org-a", store)
	require.NoError(t, err)
	owner, err := NewServiceChargeOwner(repo)
	require.NoError(t, err)
	chargeRepo, err := resourceadapter.NewGormConsumerChargeRepository(resourceDB, resourceadapter.TransactionConfig{})
	require.NoError(t, err)
	ledger, err := orgresource.NewConsumerChargeService(chargeRepo, map[orgresource.ResourceConsumer]orgresource.ConsumerChargeOwner{orgresource.ConsumerStoreService: owner})
	require.NoError(t, err)
	return serviceChargeFixture{repo: repo, ledger: ledger, positions: positions, resourceDB: resourceDB, store: store, member: member, admin: admin}
}
func serviceApplication(t *testing.T, f serviceChargeFixture, port orgresource.ConsumerChargePort) *storecenter.ServiceLifecycleApplication {
	t.Helper()
	executor, err := NewServiceLifecycleExecutor(f.repo, port, connectedStore{})
	require.NoError(t, err)
	application, err := storecenter.NewServiceLifecycleApplication(f.repo, executor, connectedStore{}, serviceTestAuthorizer(), storecenter.Phase1ServiceQuantityPolicy{}, time.Now)
	require.NoError(t, err)
	return application
}
func TestServiceChargesUseSeparateOwnersAndOriginalFundingOnReplay(t *testing.T) {
	f := newServiceChargeFixture(t)
	application := serviceApplication(t, f, f.ledger)
	activation := storecenter.ServiceLifecycleApplicationRequest{StoreID: f.store.ID(), OperationID: uuid.NewString(), ExpectedStoreVersion: 1}
	first, err := application.Activate(f.member, activation)
	require.NoError(t, err)
	require.Equal(t, "2", first.Snapshot.BalanceAfter)
	renewal := storecenter.ServiceLifecycleApplicationRequest{StoreID: f.store.ID(), OperationID: uuid.NewString(), ExpectedStoreVersion: 2, Quantity: 2}
	renewed, err := application.Renew(f.member, renewal)
	require.NoError(t, err)
	require.EqualValues(t, 3, renewed.Snapshot.StoreVersion)
	require.Equal(t, "0", renewed.Snapshot.BalanceAfter)
	require.Equal(t, 60*24*time.Hour, renewed.Snapshot.ServiceState.ExpiresAt.Sub(*first.Snapshot.ServiceState.ExpiresAt))
	replay, err := application.Renew(f.member, renewal)
	require.NoError(t, err)
	require.True(t, replay.Replayed)
	require.Equal(t, renewed.Snapshot, replay.Snapshot)
	_, err = application.Renew(f.member, storecenter.ServiceLifecycleApplicationRequest{StoreID: f.store.ID(), OperationID: uuid.NewString(), ExpectedStoreVersion: 3, Quantity: 1})
	require.ErrorIs(t, err, orgresource.ErrInsufficientBalance)
	adminRenewed, err := application.Renew(f.admin, storecenter.ServiceLifecycleApplicationRequest{StoreID: f.store.ID(), OperationID: uuid.NewString(), ExpectedStoreVersion: 3, Quantity: 1})
	require.NoError(t, err)
	require.EqualValues(t, 4, adminRenewed.Snapshot.StoreVersion)
	position, err := f.positions.ReadPosition(context.Background(), "org-a", "member-a", orgresource.ResourceStoreRenewalPeriod)
	require.NoError(t, err)
	require.Zero(t, position.Free)
	require.Zero(t, position.Reserved)
	require.EqualValues(t, 3, position.Consumed)
	var bucket struct{ Available, Allocated, Reserved, Consumed int64 }
	require.NoError(t, f.resourceDB.Table("saas_organization_resource_buckets").Take(&bucket).Error)
	require.Zero(t, bucket.Available)
	require.Zero(t, bucket.Allocated)
	require.Zero(t, bucket.Reserved)
	require.EqualValues(t, 4, bucket.Consumed)
	promoted := serviceContext("operator", "member-a", "listingkit_admin")
	replayedActivation, err := application.Activate(promoted, activation)
	require.NoError(t, err)
	require.Equal(t, "2", replayedActivation.Snapshot.BalanceAfter, "must return the original funding receipt, even after promotion and other spending")
	_, err = application.Renew(serviceContext("operator", "member-a", "listingkit_viewer"), renewal)
	require.ErrorIs(t, err, storecenter.ErrServiceLifecyclePermissionDenied)
}

type revokeOnReserve struct {
	orgresource.ConsumerChargePort
	once func()
}

func (p *revokeOnReserve) Reserve(ctx context.Context, id orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	receipt, err := p.ConsumerChargePort.Reserve(ctx, id)
	if err == nil && p.once != nil {
		action := p.once
		p.once = nil
		action()
	}
	return receipt, err
}
func TestServiceChargeReleasesOriginalMemberWhenGrantRevokedBeforeBinding(t *testing.T) {
	f := newServiceChargeFixture(t)
	port := &revokeOnReserve{ConsumerChargePort: f.ledger, once: func() {
		require.NoError(t, f.repo.SetMemberGrant(f.admin, storecenter.MemberStoreGrantCommand{OrganizationID: "org-a", StoreID: f.store.ID(), MemberID: "member-a", OperationID: uuid.NewString(), ExpectedVersion: 1}))
	}}
	application := serviceApplication(t, f, port)
	request := storecenter.ServiceLifecycleApplicationRequest{StoreID: f.store.ID(), OperationID: uuid.NewString(), ExpectedStoreVersion: 1}
	_, err := application.Activate(f.member, request)
	require.ErrorIs(t, err, storecenter.ErrNotFound)
	position, err := f.positions.ReadPosition(context.Background(), "org-a", "member-a", orgresource.ResourceStoreRenewalPeriod)
	require.NoError(t, err)
	require.EqualValues(t, 3, position.Free)
	require.Zero(t, position.Reserved)
	require.Zero(t, position.Consumed)
	require.NoError(t, f.repo.SetMemberGrant(f.admin, storecenter.MemberStoreGrantCommand{OrganizationID: "org-a", StoreID: f.store.ID(), MemberID: "member-a", OperationID: uuid.NewString(), ExpectedVersion: 2, Active: true}))
	_, err = application.Activate(f.member, request)
	require.ErrorIs(t, err, storecenter.ErrNotFound, "restoring assignment cannot revive a failed command")
	store, err := f.repo.Get(f.member, "org-a", f.store.ID())
	require.NoError(t, err)
	require.EqualValues(t, 1, store.Version())
	require.Nil(t, store.Snapshot().ServiceExpiresAt)
}

type serviceRolePolicyFixture struct{}

func (serviceRolePolicyFixture) RoleModules(_ context.Context, org string, keys []string) (map[string][]string, error) {
	result := map[string][]string{}
	for _, key := range keys {
		if key == authz.EnterpriseRoleKey(org, 1) {
			result[key] = []string{"stores"}
		}
	}
	return result, nil
}
func serviceTestAuthorizer() *authz.ListingKitAuthorizer {
	a, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		panic(err)
	}
	a.SetRolePolicyReader(serviceRolePolicyFixture{})
	return a
}
