package storecenterapp_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	storeapp "task-processor/internal/app/storecenter"
	"task-processor/internal/integration/shein"
	"task-processor/internal/storecenter"
)

type officialProvider struct {
	exchanges, queries    int
	exchangeErr, queryErr error
	key                   string
}

func (*officialProvider) Application() storecenter.OfficialApplication {
	return storecenter.OfficialApplication{AppID: "test-app", Version: storeapp.BoundOfficialRevision("config-1", storecenter.ApplicationSelfOperated), CallbackURL: "https://localhost/callback"}
}
func (*officialProvider) AuthorizationURL(state string) (string, error) {
	return "https://openapi-sem.sheincorp.com/#/empower?state=" + state, nil
}
func (p *officialProvider) Exchange(_ context.Context, token, state string) (storecenter.OfficialMerchantCredential, error) {
	p.exchanges++
	return storecenter.OfficialMerchantCredential{AppID: "test-app", OpenKeyID: p.key, SecretKey: "synthetic-merchant-secret", SupplierID: "123"}, p.exchangeErr
}
func (p *officialProvider) QueryStore(context.Context, storecenter.OfficialMerchantCredential) (storecenter.OfficialStoreInformation, error) {
	p.queries++
	return storecenter.OfficialStoreInformation{}, p.queryErr
}

func TestOfficialUnknownExchangeNeverRepeatsAndNewConsentFencesOldAttempt(t *testing.T) {
	db, repo, access, stored := serviceFixture(t)
	_ = db
	provider := &officialProvider{key: "merchant-a", exchangeErr: storecenter.ErrOfficialExchangeUnknown}
	protection, err := shein.NewCredentialProtection("test-key", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	app, err := newTestOfficialConnections(repo, provider, protection)
	if err != nil {
		t.Fatal(err)
	}
	command := storecenter.OfficialConnectionCommand{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: uuid.NewString(), ExpectedStoreVersion: stored.Version(), ApplicationID: "test-app"}
	begin, err := app.Begin(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	state := authorizationState(t, begin.AuthorizationURL)
	request := storecenter.CompleteOfficialConnection{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: command.AttemptID, AppID: "test-app", State: state, TempToken: "synthetic-temp"}
	if _, err := app.Complete(context.Background(), request); !errors.Is(err, storecenter.ErrOfficialExchangeUnknown) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := app.Complete(context.Background(), request); !errors.Is(err, storecenter.ErrOfficialExchangeUnknown) || provider.exchanges != 1 {
		t.Fatalf("reissued one-time exchange: %d %v", provider.exchanges, err)
	}
	access.member = "rejoined"
	if _, err := app.Complete(context.Background(), request); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("new membership inherited consent: %v", err)
	}
	access.member = "member-a"
	latest, err := repo.Get(context.Background(), "org-a", stored.ID())
	if err != nil {
		t.Fatal(err)
	}
	command.AttemptID = uuid.NewString()
	command.ExpectedStoreVersion = latest.Version()
	if _, err := app.Begin(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if _, err := app.Complete(context.Background(), request); !errors.Is(err, storecenter.ErrNotFound) || provider.exchanges != 1 {
		t.Fatalf("old attempt survived new consent: %v", err)
	}
}

func TestOfficialEncryptedCredentialRecoveryQueriesOnlyAndDisconnectFences(t *testing.T) {
	db, repo, _, stored := serviceFixture(t)
	provider := &officialProvider{key: "merchant-recovery", queryErr: context.DeadlineExceeded}
	protection, err := shein.NewCredentialProtection("test-key", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	app, err := newTestOfficialConnections(repo, provider, protection)
	if err != nil {
		t.Fatal(err)
	}
	command := storecenter.OfficialConnectionCommand{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: uuid.NewString(), ExpectedStoreVersion: stored.Version(), ApplicationID: "test-app"}
	begin, err := app.Begin(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.CompleteOfficialConnection{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: command.AttemptID, AppID: "test-app", State: authorizationState(t, begin.AuthorizationURL), TempToken: "synthetic-temp"}
	if _, err := app.Complete(context.Background(), request); !errors.Is(err, storecenter.ErrOfficialConnectionUnavailable) {
		t.Fatalf("query timeout: %v", err)
	}
	var cipher string
	if err := db.Table("workbench_store_connection_attempts").Select("ciphertext").Where("attempt_id = ?", command.AttemptID).Scan(&cipher).Error; err != nil {
		t.Fatal(err)
	}
	if cipher == "" || cipher == "synthetic-merchant-secret" {
		t.Fatal("credential not sealed")
	}
	provider.queryErr = nil
	// A page reload has lost state/tempToken. Recovery may only query the
	// already encrypted credential, never repeat the one-time exchange.
	result, err := app.ResumeQuery(context.Background(), "org-a", stored.ID(), command.AttemptID)
	if err != nil || result.Status != storecenter.ConnectionStatusConnected || provider.exchanges != 1 || provider.queries != 2 {
		t.Fatalf("safe recovery: %+v %v", result, err)
	}
	current, _ := repo.Get(context.Background(), "org-a", stored.ID())
	input := storecenter.ConnectionStatusInput{OrganizationID: "org-a", StoreID: stored.ID(), Platform: storecenter.PlatformShein, ConnectionRef: current.ConnectionRef()}
	if status, err := app.Status(context.Background(), input); err != nil || status != storecenter.ConnectionStatusConnected || provider.queries != 3 {
		t.Fatalf("fresh provider status: %s %v", status, err)
	}
	provider.queryErr = storecenter.ErrOfficialCredentialExpired
	if status, err := app.Status(context.Background(), input); err != nil || status != storecenter.ConnectionStatusExpired {
		t.Fatalf("revoked provider status: %s %v", status, err)
	}
	command.ExpectedStoreVersion = current.Version()
	command.AttemptID = uuid.NewString()
	if _, err := app.Disconnect(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	calls := provider.queries
	if _, err := app.Complete(context.Background(), request); !errors.Is(err, storecenter.ErrNotFound) {
		t.Fatalf("disconnected attempt revived: %v", err)
	}
	if _, err := app.Status(context.Background(), input); err == nil || provider.queries != calls {
		t.Fatalf("old reference remained usable: %v", err)
	}
}

func TestOfficialCallbackStateAppAndTTLRejectBeforeDispatch(t *testing.T) {
	_, repo, _, stored := serviceFixture(t)
	provider := &officialProvider{key: "merchant-scope"}
	protection, _ := shein.NewCredentialProtection("test-key", make([]byte, 32))
	app, _ := newTestOfficialConnections(repo, provider, protection)
	command := storecenter.OfficialConnectionCommand{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: uuid.NewString(), ExpectedStoreVersion: stored.Version(), ApplicationID: "test-app"}
	begin, err := app.Begin(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	request := storecenter.CompleteOfficialConnection{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: command.AttemptID, AppID: "other-app", State: authorizationState(t, begin.AuthorizationURL), TempToken: "synthetic-temp"}
	if _, err := app.Complete(context.Background(), request); err == nil {
		t.Fatal("different app accepted")
	}
	request.AppID = "test-app"
	request.State = "different-state"
	if _, err := app.Complete(context.Background(), request); err == nil {
		t.Fatal("wrong state accepted")
	}
	request.State = authorizationState(t, begin.AuthorizationURL)
	h := sha256.Sum256([]byte(request.State))
	if _, _, err := repo.ClaimOfficialExchange(context.Background(), "org-a", stored.ID(), command.AttemptID, "test-app", "config-1", hex.EncodeToString(h[:]), time.Now().Add(6*time.Minute)); err == nil {
		t.Fatal("expired consent dispatched")
	}
	if provider.exchanges != 0 {
		t.Fatal("rejected callback contacted provider")
	}
}
func authorizationState(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	_, query, ok := splitFragment(u.Fragment)
	if !ok {
		t.Fatal("missing state")
	}
	v, err := url.ParseQuery(query)
	if err != nil {
		t.Fatal(err)
	}
	return v.Get("state")
}
func splitFragment(s string) (string, string, bool) {
	for i, c := range s {
		if c == '?' {
			return s[:i], s[i+1:], true
		}
	}
	return "", "", false
}

func TestOfficialOneMerchantCannotAttachToAnotherEnterpriseOrStore(t *testing.T) {
	db, repo, access, stored := serviceFixture(t)
	protection, _ := shein.NewCredentialProtection("test-key", make([]byte, 32))
	provider := &officialProvider{key: "merchant-global"}
	app, _ := newTestOfficialConnections(repo, provider, protection)
	connect := func(org string, s *storecenter.Store) error {
		cmd := storecenter.OfficialConnectionCommand{OrganizationID: org, StoreID: s.ID(), AttemptID: uuid.NewString(), ExpectedStoreVersion: s.Version(), ApplicationID: "test-app"}
		begin, err := app.Begin(context.Background(), cmd)
		if err != nil {
			return err
		}
		_, err = app.Complete(context.Background(), storecenter.CompleteOfficialConnection{OrganizationID: org, StoreID: s.ID(), AttemptID: cmd.AttemptID, AppID: "test-app", State: authorizationState(t, begin.AuthorizationURL), TempToken: "synthetic-temp"})
		return err
	}
	if err := connect("org-a", stored); err != nil {
		t.Fatal(err)
	}
	other, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: uuid.NewString(), OrganizationID: "org-b", ActorSubject: "operator", Name: "Other", Platform: "shein", Region: "SG", ExternalStoreID: "record-metadata", CreateIdempotencyKey: uuid.NewString(), OccurredAt: time.Now().UTC().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	access.member = "member-b"
	other, _, err = repo.CreateOrReplay(context.Background(), "org-b", other)
	if err != nil {
		t.Fatal(err)
	}
	if err := connect("org-b", other); !errors.Is(err, storecenter.ErrAlreadyExists) {
		t.Fatalf("merchant attached to two enterprises: %v", err)
	}
	var saved int64
	if err := db.Table("workbench_store_connection_attempts").Where("organization_id = ? AND ciphertext <> ''", "org-b").Count(&saved).Error; err != nil || saved != 0 {
		t.Fatalf("rejected association saved credentials: %d %v", saved, err)
	}
	access.member = "member-a"
	original, _ := repo.Get(context.Background(), "org-a", stored.ID())
	status, err := app.Status(context.Background(), storecenter.ConnectionStatusInput{OrganizationID: "org-a", StoreID: stored.ID(), Platform: storecenter.PlatformShein, ConnectionRef: original.ConnectionRef()})
	if err != nil || status != storecenter.ConnectionStatusConnected {
		t.Fatalf("original association broken: %v", err)
	}
}
func TestOfficialMissingConfigurationLeavesReadsUsableAndDoesNotCreateConsent(t *testing.T) {
	db, repo, _, stored := serviceFixture(t)
	app, err := storeapp.NewUnconfiguredOfficialConnections(repo)
	if err != nil {
		t.Fatal(err)
	}
	if result, err := app.Read(context.Background(), "org-a", stored.ID()); err != nil || result.Status != storecenter.ConnectionStatusUnavailable {
		t.Fatalf("read without setup: %+v %v", result, err)
	}
	if _, err := app.Begin(context.Background(), storecenter.OfficialConnectionCommand{OrganizationID: "org-a", StoreID: stored.ID(), AttemptID: uuid.NewString(), ExpectedStoreVersion: stored.Version(), ApplicationID: "test-app"}); !errors.Is(err, storecenter.ErrOfficialConnectionUnavailable) {
		t.Fatalf("missing setup admitted consent: %v", err)
	}
	var count int64
	if err := db.Table("workbench_store_connection_attempts").Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("missing setup wrote consent: %d %v", count, err)
	}
}

type officialAccess struct{ member string }

func (a *officialAccess) AuthorizeStoreMember(_ context.Context, org string) (storecenter.StoreMemberAccess, error) {
	return storecenter.StoreMemberAccess{OrganizationID: org, ActorID: "operator", MemberID: a.member, CanWrite: true}, nil
}
func serviceFixture(t *testing.T) (*gorm.DB, *storecenter.MemberScopedStoreRepository, *officialAccess, *storecenter.Store) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "store.db")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	pool, _ := db.DB()
	pool.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = pool.Close() })
	if err := storecenter.AutoMigrateStoreRepository(db); err != nil {
		t.Fatal(err)
	}
	access := &officialAccess{member: "member-a"}
	repo, err := storecenter.NewMemberScopedStoreRepository(db, access)
	if err != nil {
		t.Fatal(err)
	}
	store, err := storecenter.NewStore(storecenter.CreateStoreInput{ID: uuid.NewString(), OrganizationID: "org-a", ActorSubject: "operator", Name: "Store", Platform: "shein", Region: "SG", ExternalStoreID: "record-metadata", CreateIdempotencyKey: uuid.NewString(), OccurredAt: time.Now().UTC().Add(-time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	store, _, err = repo.CreateOrReplay(context.Background(), "org-a", store)
	if err != nil {
		t.Fatal(err)
	}
	return db, repo, access, store
}

func newTestOfficialConnections(repo storecenter.OfficialConnectionStore, provider storecenter.OfficialConnectionProvider, protection storecenter.OfficialCredentialProtection) (*storeapp.OfficialConnections, error) {
	registry, err := storeapp.NewOfficialApplicationRegistry([]storeapp.OfficialApplicationRegistration{{Provider: provider, Protection: protection, Type: storecenter.ApplicationSelfOperated}})
	if err != nil {
		return nil, err
	}
	return storeapp.NewOfficialConnections(repo, registry)
}
