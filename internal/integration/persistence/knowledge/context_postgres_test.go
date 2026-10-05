//go:build integration

package knowledge_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"task-processor/internal/agent"
	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/integration/commercetoolauth"
	"task-processor/internal/integration/knowledgeauth"
	store "task-processor/internal/integration/persistence/knowledge"
	k "task-processor/internal/knowledge"
	"task-processor/internal/workbenchcontext"
)

type contextFixture struct {
	t     *testing.T
	db    *gorm.DB
	repo  *store.Repository
	scope k.Scope
	base  k.Base
}

type contextGrantClient struct {
	roles   []string
	revoked bool
	calls   int
}

func (c *contextGrantClient) ListOwnProjectAuthorizations(context.Context, string, string, string) ([]authidentity.OrganizationGrant, error) {
	c.calls++
	if c.revoked {
		return nil, nil
	}
	return []authidentity.OrganizationGrant{{OrganizationID: "org-a", ProjectID: "project", Roles: c.roles}}, nil
}

type contextUnusedObjects struct{ k.KnowledgeObjectStore }

func TestKnowledgeContextPostgresLivePermissionCitationAndUnavailableSources(t *testing.T) {
	f := newContextFixture(t)
	ctx := context.Background()
	_, err := f.repo.Materialize(ctx, f.request())
	require.ErrorIs(t, err, k.ErrNotReadable)
	pending := f.apply(k.Command{Kind: "source_create", BaseID: f.base.ID, Name: "Pending", Upload: &k.Revision{Filename: "guide.txt", ContentType: "text/plain", SizeBytes: 5, SHA256: k.Digest([]byte("hello"))}})
	_, err = f.repo.Materialize(ctx, f.request())
	require.ErrorIs(t, err, k.ErrNotReadable)
	f.promote(*pending.Revision, "eligible", "")
	for range 3 {
		f.source("another eligible source", "")
	}
	client := &contextGrantClient{roles: []string{"listingkit_operator"}}
	resolver := workbenchcontext.NewResolver(workbenchcontext.NewGrantResolver(client, nil), "project", "v1", nil)
	policy, err := authz.NewListingKitAuthorizer(nil, nil)
	require.NoError(t, err)
	auth, err := knowledgeauth.NewAuthorizer(resolver, policy)
	require.NoError(t, err)
	owner, err := k.NewService(f.repo, contextUnusedObjects{})
	require.NoError(t, err)
	service, err := owner.Context(auth)
	require.NoError(t, err)
	ctx = commercetoolauth.WithOrganizationRequest(ctx, commercetoolauth.OrganizationRequest{Identity: authidentity.AuthenticatedIdentity{UserID: "actor-a", HomeOrganizationID: "org-a", TokenExpiresAt: time.Now().Add(time.Hour)}, BearerToken: "synthetic-test-token", RequestedOrganizationID: "org-a"})
	req := f.request()
	ref, err := service.Materialize(ctx, req)
	require.NoError(t, err)
	bundle, err := service.ReadContext(ctx, f.scope, ref)
	require.NoError(t, err)
	require.Len(t, bundle.Entries, 4)
	citationID := bundle.Entries[0].Citation.ID
	citation, err := service.ReadCitation(ctx, f.scope, ref, citationID)
	require.NoError(t, err)
	require.Equal(t, bundle.Entries[0].Text, citation.Text)
	_, err = service.ReadCitation(ctx, f.scope, ref, uuid.NewString())
	require.ErrorIs(t, err, k.ErrNotFound)
	other, err := service.Materialize(ctx, f.request())
	require.NoError(t, err)
	_, err = service.ReadCitation(ctx, f.scope, other, citationID)
	require.ErrorIs(t, err, k.ErrNotFound)
	changed := req
	changed.Selection = "knowledge-base:" + uuid.NewString()
	_, err = service.Materialize(ctx, changed)
	require.ErrorIs(t, err, k.ErrConflict)
	permit, err := service.AcquireDispatchPermit(ctx, f.scope, ref, "authorized")
	require.NoError(t, err)
	client.roles = []string{"listingkit_viewer"}
	for _, operation := range []func() error{
		func() error { _, err := service.ObserveSelection(ctx, f.scope, f.base.ID); return err },
		func() error { _, err := service.Materialize(ctx, req); return err },
		func() error {
			b, err := service.ReadContext(ctx, f.scope, ref)
			require.Empty(t, b.Entries)
			return err
		},
		func() error {
			c, err := service.ReadCitation(ctx, f.scope, ref, citationID)
			require.Empty(t, c.Text)
			return err
		},
		func() error { _, err := service.AcquireDispatchPermit(ctx, f.scope, ref, "revoked"); return err },
	} {
		before := client.calls
		require.ErrorIs(t, operation(), k.ErrForbidden)
		require.Equal(t, before+1, client.calls)
	}
	// Terminal cleanup remains possible after role/Organization revocation.
	client.revoked = true
	_, err = service.ReadContext(ctx, f.scope, ref)
	require.ErrorIs(t, err, k.ErrForbidden)
	require.NoError(t, service.ReleaseDispatchPermit(ctx, permit))
	client.revoked = false
	client.roles = []string{"listingkit_admin"}
	_, err = service.ReadContext(ctx, k.Scope{OrganizationID: "org-b", ActorID: "actor-a"}, ref)
	require.ErrorIs(t, err, k.ErrForbidden)
	_, err = service.ReadContext(ctx, k.Scope{OrganizationID: "org-a", ActorID: "other"}, ref)
	require.ErrorIs(t, err, k.ErrForbidden)
	f.apply(k.Command{Kind: "base_disable", BaseID: f.base.ID, Version: f.base.Version})
	_, err = service.ReadCitation(ctx, f.scope, ref, citationID)
	require.ErrorIs(t, err, k.ErrInactive)
}

func newContextFixture(t *testing.T) *contextFixture {
	t.Helper()
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:17-alpine", tcpostgres.WithDatabase("knowledge"), tcpostgres.WithUsername("knowledge_owner"), tcpostgres.WithPassword("isolated-test-password"), tcpostgres.BasicWaitStrategies())
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(context.Background()) })
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(8)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.NoError(t, store.Install(ctx, db))
	repo, err := store.NewRepository(ctx, db)
	require.NoError(t, err)
	f := &contextFixture{t: t, db: db, repo: repo, scope: k.Scope{OrganizationID: "org-a", ActorID: "actor-a"}}
	f.base = *f.apply(k.Command{Kind: "base_create", Name: "Brand"}).Base
	return f
}
func (f *contextFixture) apply(c k.Command) k.Result {
	f.t.Helper()
	c.Scope = f.scope
	c.Key = uuid.NewString()
	c.Fingerprint = k.Digest([]byte(c.Key))
	r, err := f.repo.Apply(context.Background(), c)
	require.NoError(f.t, err)
	return r
}
func (f *contextFixture) source(text, warning string) k.Source {
	f.t.Helper()
	r := f.apply(k.Command{Kind: "source_create", BaseID: f.base.ID, Name: "Guide", Upload: &k.Revision{Filename: "guide.txt", ContentType: "text/plain", SizeBytes: 5, SHA256: k.Digest([]byte("hello"))}})
	f.promote(*r.Revision, text, warning)
	s, err := f.repo.GetSource(context.Background(), f.scope.OrganizationID, r.Source.ID)
	require.NoError(f.t, err)
	return s
}
func (f *contextFixture) promote(rev k.Revision, text, warning string) {
	f.t.Helper()
	ctx := context.Background()
	claimed, ok, err := f.repo.ClaimUpload(ctx, f.scope.OrganizationID, rev.ID, uuid.NewString())
	require.NoError(f.t, err)
	require.True(f.t, ok)
	require.NoError(f.t, f.repo.ConfirmObject(ctx, claimed))
	revs, err := f.repo.ClaimProcessing(ctx, uuid.NewString(), 4)
	require.NoError(f.t, err)
	for _, r := range revs {
		if r.ID == rev.ID {
			require.NoError(f.t, f.repo.Finish(ctx, r, k.ParseResult{Text: text, Warning: warning}))
			return
		}
	}
	f.t.Fatal("target revision was not claimed")
}
func (f *contextFixture) request() k.ContextRequest {
	return k.ContextRequest{Scope: f.scope, Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation", ProductKey: "product", CatalogVersion: "1", PublicationID: "publication", TargetPlatform: "shein"}, Key: uuid.NewString(), Selection: "knowledge-base:" + f.base.ID, PolicyVersion: k.ContextPolicyVersion}
}

func TestKnowledgeSelectionRevisionSetFencesChatMaterialization(t *testing.T) {
	f := newContextFixture(t)
	source := f.source("first revision", "")
	ctx := context.Background()
	observed, err := f.repo.ObserveSelection(ctx, f.scope, f.base.ID)
	require.NoError(t, err)
	require.Equal(t, f.base.ID, observed.BaseID)
	require.Len(t, observed.Digest, 64)
	request := f.request()
	request.ExpectedRevisionSetDigest = observed.Digest
	_, err = f.repo.Materialize(ctx, request)
	require.NoError(t, err)
	created := f.apply(k.Command{Kind: "revision_create", SourceID: source.ID, Version: source.Version, Name: source.Name,
		Upload: &k.Revision{Filename: "next.txt", ContentType: "text/plain", SizeBytes: 5, SHA256: k.Digest([]byte("newer"))}})
	f.promote(*created.Revision, "new current revision", "")
	_, err = f.repo.Materialize(ctx, request)
	require.ErrorIs(t, err, k.ErrSelectionChanged, "same-key adoption must recheck the observed revision set before T1")
	changed := f.request()
	changed.ExpectedRevisionSetDigest = observed.Digest
	_, err = f.repo.Materialize(ctx, changed)
	require.ErrorIs(t, err, k.ErrSelectionChanged)
}

func TestKnowledgeContextPostgresFrozenAdoptionAndContentAdmission(t *testing.T) {
	f := newContextFixture(t)
	ctx := context.Background()
	s := f.source("old exact text", "INCOMPLETE_EXTRACTION")
	req := f.request()
	ref, err := f.repo.Materialize(ctx, req)
	require.NoError(t, err)
	b, err := f.repo.ReadContext(ctx, f.scope, ref)
	require.NoError(t, err)
	require.Len(t, b.Entries, 1)
	require.Equal(t, k.Partial, b.Entries[0].State)
	require.Equal(t, "INCOMPLETE_EXTRACTION", b.Entries[0].Warning)
	original := b.Entries[0]
	r := f.apply(k.Command{Kind: "revision_create", SourceID: s.ID, Version: s.Version, Name: s.Name, Upload: &k.Revision{Filename: "next.txt", ContentType: "text/plain", SizeBytes: 5, SHA256: k.Digest([]byte("newer"))}})
	f.promote(*r.Revision, "new latest text", "")
	f.source("another source", "")
	// A new repository instance models lost response/restart after bundle commit.
	restarted, err := store.NewRepository(ctx, f.db)
	require.NoError(t, err)
	again, err := restarted.Materialize(ctx, req)
	require.NoError(t, err)
	require.Equal(t, ref, again)
	frozen, err := restarted.ReadContext(ctx, f.scope, ref)
	require.NoError(t, err)
	require.Equal(t, original, frozen.Entries[0])
	require.Len(t, frozen.Entries, 1)
	changed := req
	changed.Binding.TargetPlatform = "temu"
	_, err = f.repo.Materialize(ctx, changed)
	require.ErrorIs(t, err, k.ErrConflict)
	changed = req
	changed.PolicyVersion = "v2"
	_, err = f.repo.Materialize(ctx, changed)
	require.ErrorIs(t, err, k.ErrConflict)
	wrong := ref
	wrong.Digest = k.Digest([]byte("wrong"))
	_, err = f.repo.ReadContext(ctx, f.scope, wrong)
	require.ErrorIs(t, err, k.ErrIntegrity)
	_, err = f.repo.ReadContext(ctx, k.Scope{OrganizationID: "org-b", ActorID: "actor-a"}, ref)
	require.ErrorIs(t, err, k.ErrNotFound)
	latest, err := f.repo.GetSource(ctx, f.scope.OrganizationID, s.ID)
	require.NoError(t, err)
	f.apply(k.Command{Kind: "source_disable", SourceID: s.ID, Version: latest.Version})
	blocked, err := f.repo.ReadContext(ctx, f.scope, ref)
	require.ErrorIs(t, err, k.ErrInactive)
	require.Empty(t, blocked.Entries)
	_, err = f.repo.Materialize(ctx, req)
	require.ErrorIs(t, err, k.ErrInactive)
}

func TestKnowledgeContextPostgresConcurrentIdentityAndBounds(t *testing.T) {
	f := newContextFixture(t)
	ctx := context.Background()
	f.source("frozen", "")
	req := f.request()
	var wg sync.WaitGroup
	refs := make(chan k.ContextSnapshotRef, 12)
	errs := make(chan error, 12)
	for range 12 {
		wg.Go(func() { r, e := f.repo.Materialize(ctx, req); refs <- r; errs <- e })
	}
	wg.Wait()
	close(refs)
	close(errs)
	for e := range errs {
		require.NoError(t, e)
	}
	var id string
	for r := range refs {
		if id == "" {
			id = r.ID
		}
		require.Equal(t, id, r.ID)
	}
	var count int64
	require.NoError(t, f.db.Table("public.knowledge_context_bundles").Count(&count).Error)
	require.EqualValues(t, 1, count)
	for range 3 {
		f.source(strings.Repeat("a", k.MaxContextRevisionBytes), "")
	}
	_, err := f.repo.Materialize(ctx, f.request())
	require.ErrorIs(t, err, k.ErrContextTooLarge)
	_, err = f.repo.Materialize(ctx, req)
	require.NoError(t, err) // adoption never recomputes latest source count/bytes
}

func TestKnowledgeDispatchPermitPostgresDisableReleaseAndExpiry(t *testing.T) {
	f := newContextFixture(t)
	ctx := context.Background()
	s := f.source("frozen", "")
	ref, err := f.repo.Materialize(ctx, f.request())
	require.NoError(t, err)
	permit, err := f.repo.AcquireDispatchPermit(ctx, f.scope, ref, "invocation")
	require.NoError(t, err)
	require.WithinDuration(t, time.Now().Add(k.DispatchPermitLease), permit.ExpiresAt, 5*time.Second)
	_, err = f.repo.AcquireDispatchPermit(ctx, f.scope, ref, "invocation")
	require.ErrorIs(t, err, k.ErrConflict)
	disabling := f.apply(k.Command{Kind: "source_disable", SourceID: s.ID, Version: s.Version}).Source
	require.Equal(t, k.Disabling, disabling.State)
	_, err = f.repo.AcquireDispatchPermit(ctx, f.scope, ref, "later")
	require.ErrorIs(t, err, k.ErrInactive)
	_, err = f.repo.ReadContext(ctx, f.scope, ref)
	require.ErrorIs(t, err, k.ErrInactive)
	forged := permit
	forged.ActorID = "other"
	require.ErrorIs(t, f.repo.ReleaseDispatchPermit(ctx, forged), k.ErrNotFound)
	require.NoError(t, f.repo.ReleaseDispatchPermit(ctx, permit))
	require.NoError(t, f.repo.ReleaseDispatchPermit(ctx, permit))
	s, err = f.repo.GetSource(ctx, f.scope.OrganizationID, s.ID)
	require.NoError(t, err)
	require.Equal(t, k.Disabled, s.State)
	_, err = f.repo.AcquireDispatchPermit(ctx, f.scope, ref, "invocation")
	require.Error(t, err)
	// A separate active source is frozen into a new bundle; only expiry recovery
	// changes Knowledge state, with no model/provider dependency in the fixture.
	f.source("expiry", "")
	ref, err = f.repo.Materialize(ctx, f.request())
	require.NoError(t, err)
	permit, err = f.repo.AcquireDispatchPermit(ctx, f.scope, ref, "expiry")
	require.NoError(t, err)
	b := f.apply(k.Command{Kind: "base_disable", BaseID: f.base.ID, Version: f.base.Version}).Base
	require.Equal(t, k.Disabling, b.State)
	require.NoError(t, f.db.Exec("UPDATE public.knowledge_dispatch_permits SET acquired_at=clock_timestamp()-interval '6 minutes',expires_at=clock_timestamp()-interval '1 minute' WHERE id=?", permit.ID).Error)
	restarted, err := store.NewRepository(ctx, f.db)
	require.NoError(t, err)
	require.NoError(t, restarted.RecoverExpiredDispatchPermits(ctx))
	finalBase, err2 := restarted.GetBase(ctx, f.scope.OrganizationID, f.base.ID)
	require.NoError(t, err2)
	require.Equal(t, k.Disabled, finalBase.State)
	var state string
	require.NoError(t, f.db.Raw("SELECT state FROM public.knowledge_dispatch_permits WHERE id=?", permit.ID).Scan(&state).Error)
	require.Equal(t, "EXPIRED", state)
}

func TestKnowledgeDispatchPermitPostgresConcurrentDisableOrdering(t *testing.T) {
	f := newContextFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	f.source("frozen", "")
	ref, err := f.repo.Materialize(ctx, f.request())
	require.NoError(t, err)
	var wg sync.WaitGroup
	var permit k.DispatchPermit
	var acquireErr, disableErr error
	var disabled k.Result
	wg.Go(func() { permit, acquireErr = f.repo.AcquireDispatchPermit(ctx, f.scope, ref, "race") })
	wg.Go(func() {
		disabled, disableErr = f.repo.Apply(ctx, k.Command{Scope: f.scope, Kind: "base_disable", Key: uuid.NewString(), Fingerprint: k.Digest([]byte("disable")), BaseID: f.base.ID, Version: f.base.Version})
	})
	wg.Wait()
	require.NoError(t, disableErr)
	if acquireErr == nil {
		require.Equal(t, k.Disabling, disabled.Base.State)
		require.NoError(t, f.repo.ReleaseDispatchPermit(ctx, permit))
	} else {
		require.True(t, errors.Is(acquireErr, k.ErrInactive), acquireErr)
		require.Equal(t, k.Disabled, disabled.Base.State)
	}
	base, err := f.repo.GetBase(ctx, f.scope.OrganizationID, f.base.ID)
	require.NoError(t, err)
	require.Equal(t, k.Disabled, base.State)
}
