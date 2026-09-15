package sourceaccountregistry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

func TestRegisterCreatesCurrentPendingResource(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	store := &fakeStore{tx: &fakeTx{count: 3}}
	authorizer := &fakeAuthorizer{allowed: map[string]bool{authz.PermissionWorkbenchSourceAccountManage: true}}
	service := newTestService(t, store, authorizer, now)

	result, err := service.Register(identityContext(now, "org-b", "actor-1", "listingkit_operator"), uuid.NewString(), RegisterInput{
		DisplayName: "  Primary 1688  ",
		Platform:    "1688",
	})
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if result.Replayed {
		t.Fatal("Register() replayed = true, want false")
	}
	account := result.Account
	if parsed, parseErr := uuid.Parse(account.ID); parseErr != nil || parsed.Version() != 7 {
		t.Fatalf("account ID = %q, want UUIDv7", account.ID)
	}
	if account.OrganizationID != "org-b" || account.DisplayName != "Primary 1688" || account.Platform != Platform1688 {
		t.Fatalf("account identity = %#v", account)
	}
	if account.ManagementStatus != ManagementStatusEnabled || account.ConnectionStatus != ConnectionStatusPending || account.Version != 1 {
		t.Fatalf("account state = %#v", account)
	}
	if account.CreatedBy != "actor-1" || account.UpdatedBy != "actor-1" || !account.CreatedAt.Equal(now) || !account.UpdatedAt.Equal(now) {
		t.Fatalf("account audit = %#v", account)
	}
	if store.tx.countCalls != 1 || store.tx.insertCalls != 1 || store.tx.completeCalls != 1 {
		t.Fatalf("transaction calls = count %d insert %d complete %d", store.tx.countCalls, store.tx.insertCalls, store.tx.completeCalls)
	}
	if store.lastOperation.Scope != (Scope{OrganizationID: "org-b", ActorSubject: "actor-1"}) || store.lastOperation.Kind != OperationRegister || store.lastOperation.Fingerprint == "" {
		t.Fatalf("operation = %#v", store.lastOperation)
	}
	if authorizer.lastUser != "actor-1" || authorizer.lastPermission != authz.PermissionWorkbenchSourceAccountManage {
		t.Fatalf("authorization = user %q permission %q", authorizer.lastUser, authorizer.lastPermission)
	}
}

func TestRegisterReplayPrecedesCapacityAndReturnsCurrentProjection(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	current := validAccount(t, now)
	current.ManagementStatus = ManagementStatusDisabled
	current.Version = 2
	store := &fakeStore{tx: &fakeTx{replay: &current, count: MaxAccountsPerOrganization}}
	idGenerationCalls := 0
	service, err := NewService(store, allowAllAuthorizer(), WithClock(func() time.Time { return now }), WithIDGenerator(func() (string, error) {
		idGenerationCalls++
		return "", errors.New("ID generation must not run for a replay")
	}))
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Register(identityContext(now, "org-b", "actor-1", "listingkit_operator"), uuid.NewString(), RegisterInput{DisplayName: "Primary", Platform: "1688"})
	if err != nil {
		t.Fatalf("Register() replay error = %v", err)
	}
	if !result.Replayed || result.Account.ManagementStatus != ManagementStatusDisabled || result.Account.Version != 2 {
		t.Fatalf("Register() replay result = %#v", result)
	}
	if store.tx.countCalls != 0 || store.tx.insertCalls != 0 || store.tx.completeCalls != 0 {
		t.Fatalf("replay performed writes/capacity read: %#v", store.tx)
	}
	if idGenerationCalls != 0 {
		t.Fatalf("replay generated %d account IDs", idGenerationCalls)
	}
}

func TestRegisterRejectsOrganizationLimit(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	store := &fakeStore{tx: &fakeTx{count: MaxAccountsPerOrganization}}
	service := newTestService(t, store, allowAllAuthorizer(), now)

	_, err := service.Register(identityContext(now, "org-b", "actor-1", "listingkit_operator"), uuid.NewString(), RegisterInput{DisplayName: "Primary", Platform: "1688"})
	if !errors.Is(err, ErrResourceLimitReached) {
		t.Fatalf("Register() error = %v, want ErrResourceLimitReached", err)
	}
	if store.tx.insertCalls != 0 || store.tx.completeCalls != 0 {
		t.Fatalf("limit rejection wrote transaction: %#v", store.tx)
	}
}

func TestRegisterRejectsInvalidOrUnauthorizedInput(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	tests := []struct {
		name       string
		ctx        context.Context
		key        string
		input      RegisterInput
		authorizer *fakeAuthorizer
		want       error
	}{
		{name: "missing identity", ctx: context.Background(), key: uuid.NewString(), input: RegisterInput{DisplayName: "Primary", Platform: "1688"}, authorizer: allowAllAuthorizer(), want: ErrAuthenticationRequired},
		{name: "expired", ctx: identityContext(now.Add(-time.Hour), "org-b", "actor-1", "listingkit_operator"), key: uuid.NewString(), input: RegisterInput{DisplayName: "Primary", Platform: "1688"}, authorizer: allowAllAuthorizer(), want: ErrAuthenticationRequired},
		{name: "tenant mismatch", ctx: authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{TenantID: "org-a", EffectiveOrganizationID: "org-b", UserID: "actor-1", Roles: []string{"listingkit_operator"}, TokenExpiresAt: now.Add(time.Hour)}), key: uuid.NewString(), input: RegisterInput{DisplayName: "Primary", Platform: "1688"}, authorizer: allowAllAuthorizer(), want: ErrForbidden},
		{name: "viewer", ctx: identityContext(now, "org-b", "actor-1", "listingkit_viewer"), key: uuid.NewString(), input: RegisterInput{DisplayName: "Primary", Platform: "1688"}, authorizer: &fakeAuthorizer{allowed: map[string]bool{}}, want: ErrForbidden},
		{name: "noncanonical key", ctx: identityContext(now, "org-b", "actor-1", "listingkit_operator"), key: "NOT-A-UUID", input: RegisterInput{DisplayName: "Primary", Platform: "1688"}, authorizer: allowAllAuthorizer(), want: ErrInvalid},
		{name: "wrong platform", ctx: identityContext(now, "org-b", "actor-1", "listingkit_operator"), key: uuid.NewString(), input: RegisterInput{DisplayName: "Primary", Platform: "amazon"}, authorizer: allowAllAuthorizer(), want: ErrInvalid},
		{name: "control name", ctx: identityContext(now, "org-b", "actor-1", "listingkit_operator"), key: uuid.NewString(), input: RegisterInput{DisplayName: "Primary\n1688", Platform: "1688"}, authorizer: allowAllAuthorizer(), want: ErrInvalid},
		{name: "oversize name", ctx: identityContext(now, "org-b", "actor-1", "listingkit_operator"), key: uuid.NewString(), input: RegisterInput{DisplayName: strings.Repeat("a", MaxDisplayNameBytes+1), Platform: "1688"}, authorizer: allowAllAuthorizer(), want: ErrInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := newTestService(t, &fakeStore{tx: &fakeTx{}}, tt.authorizer, now)
			_, err := service.Register(tt.ctx, tt.key, tt.input)
			if !errors.Is(err, tt.want) {
				t.Fatalf("Register() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestLifecycleUsesExactVersionAndNeverReplaysOldTarget(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	account := validAccount(t, now)
	tx := &fakeTx{loaded: account}
	store := &fakeStore{tx: tx}
	service := newTestService(t, store, allowAllAuthorizer(), now.Add(time.Minute))

	disabled, err := service.Disable(identityContext(now, "org-b", "actor-1", "listingkit_operator"), uuid.NewString(), account.ID, 1)
	if err != nil {
		t.Fatalf("Disable() error = %v", err)
	}
	if disabled.Account.ManagementStatus != ManagementStatusDisabled || disabled.Account.Version != 2 || disabled.Account.UpdatedBy != "actor-1" {
		t.Fatalf("Disable() = %#v", disabled)
	}
	if tx.saveExpected != 1 || tx.saved.ManagementStatus != ManagementStatusDisabled || tx.completeCalls != 1 {
		t.Fatalf("disable transaction = %#v", tx)
	}

	current := tx.saved
	current.ManagementStatus = ManagementStatusDisabled
	current.Version = 4
	tx = &fakeTx{replay: &current}
	store.tx = tx
	replayed, err := service.Enable(identityContext(now, "org-b", "actor-1", "listingkit_operator"), uuid.NewString(), account.ID, 3)
	if err != nil {
		t.Fatalf("Enable() replay error = %v", err)
	}
	if !replayed.Replayed || replayed.Account.ManagementStatus != ManagementStatusDisabled || replayed.Account.Version != 4 {
		t.Fatalf("Enable() replay = %#v, want current disabled version 4", replayed)
	}
	if tx.loadCalls != 0 || tx.saveCalls != 0 || tx.completeCalls != 0 {
		t.Fatalf("replay performed state mutation: %#v", tx)
	}
}

func TestLifecycleRejectsStaleAndInvalidTransitions(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	account := validAccount(t, now)
	tests := []struct {
		name     string
		status   ManagementStatus
		expected int64
		disable  bool
		want     error
	}{
		{name: "stale", status: ManagementStatusEnabled, expected: 2, disable: true, want: ErrVersionConflict},
		{name: "disable disabled", status: ManagementStatusDisabled, expected: 1, disable: true, want: ErrInvalidTransition},
		{name: "enable enabled", status: ManagementStatusEnabled, expected: 1, disable: false, want: ErrInvalidTransition},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loaded := account
			loaded.ManagementStatus = tt.status
			store := &fakeStore{tx: &fakeTx{loaded: loaded}}
			service := newTestService(t, store, allowAllAuthorizer(), now.Add(time.Minute))
			var err error
			if tt.disable {
				_, err = service.Disable(identityContext(now, "org-b", "actor-1", "listingkit_operator"), uuid.NewString(), account.ID, tt.expected)
			} else {
				_, err = service.Enable(identityContext(now, "org-b", "actor-1", "listingkit_operator"), uuid.NewString(), account.ID, tt.expected)
			}
			if !errors.Is(err, tt.want) {
				t.Fatalf("lifecycle error = %v, want %v", err, tt.want)
			}
			if store.tx.saveCalls != 0 || store.tx.completeCalls != 0 {
				t.Fatalf("rejected lifecycle wrote: %#v", store.tx)
			}
		})
	}
}

func TestReadUsesScopedStoreAndReadPermission(t *testing.T) {
	now := time.Date(2026, 9, 9, 1, 2, 3, 0, time.UTC)
	account := validAccount(t, now)
	store := &fakeStore{readAccount: account, page: Page{Items: []Account{account}}}
	authorizer := &fakeAuthorizer{allowed: map[string]bool{authz.PermissionWorkbenchSourceAccountRead: true}}
	service := newTestService(t, store, authorizer, now)
	ctx := identityContext(now, "org-b", "actor-1", "listingkit_viewer")

	got, err := service.Get(ctx, account.ID)
	if err != nil || got.ID != account.ID || store.readScope.OrganizationID != "org-b" {
		t.Fatalf("Get() = %#v, %v, scope %#v", got, err, store.readScope)
	}
	page, err := service.List(ctx, PageRequest{Limit: 20})
	if err != nil || len(page.Items) != 1 || store.listScope.OrganizationID != "org-b" {
		t.Fatalf("List() = %#v, %v, scope %#v", page, err, store.listScope)
	}
	if authorizer.lastUser != "actor-1" || authorizer.lastPermission != authz.PermissionWorkbenchSourceAccountRead {
		t.Fatalf("read authorization = user %q permission %q", authorizer.lastUser, authorizer.lastPermission)
	}
}

func TestConfiguredAuthorityAcrossSourceAccountOperations(t *testing.T) {
	now := time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)
	for _, operation := range []string{"register", "register replay", "get", "list", "enable", "disable", "enable replay", "disable replay"} {
		for _, mode := range []string{"configured user", "configured role", "ordinary viewer", "configuration removed", "expired", "tenant mismatch", "missing organization"} {
			t.Run(operation+"/"+mode, func(t *testing.T) {
				users, roles := []string{"actor-1"}, []string(nil)
				identity := authidentity.AuthenticatedIdentity{TenantID: "org-b", EffectiveOrganizationID: "org-b", UserID: "actor-1", Roles: []string{"custom_member"}, TokenExpiresAt: now.Add(time.Hour)}
				var want error
				switch mode {
				case "configured role":
					users, roles = nil, []string{"custom_support"}
					identity.Roles = []string{"custom_support"}
				case "ordinary viewer":
					users = nil
					identity.Roles = []string{"listingkit_viewer"}
					if operation != "get" && operation != "list" {
						want = ErrForbidden
					}
				case "configuration removed":
					users, want = nil, ErrForbidden
				case "expired":
					identity.TokenExpiresAt, want = now, ErrAuthenticationRequired
				case "tenant mismatch":
					identity.TenantID, want = "org-a", ErrForbidden
				case "missing organization":
					identity.TenantID, identity.EffectiveOrganizationID, want = "", "", ErrForbidden
				}
				// Reconstruct the real authorizer, including after configuration removal.
				authorizer, err := authz.NewListingKitAuthorizer(users, roles)
				if err != nil {
					t.Fatal(err)
				}
				account := validAccount(t, now)
				if strings.HasPrefix(operation, "enable") {
					account.ManagementStatus = ManagementStatusDisabled
				}
				store := &fakeStore{tx: &fakeTx{loaded: account}, readAccount: account, page: Page{Items: []Account{account}}}
				if strings.HasSuffix(operation, "replay") {
					store.tx.replay = &account
				}
				service := newTestService(t, store, authorizer, now)
				ctx := authidentity.WithAuthenticatedIdentity(context.Background(), identity)
				var result MutationResult
				switch {
				case strings.HasPrefix(operation, "register"):
					result, err = service.Register(ctx, uuid.NewString(), RegisterInput{DisplayName: "Configured authority", Platform: "1688"})
				case operation == "get":
					_, err = service.Get(ctx, account.ID)
				case operation == "list":
					_, err = service.List(ctx, PageRequest{Limit: 20})
				case strings.HasPrefix(operation, "enable"):
					result, err = service.Enable(ctx, uuid.NewString(), account.ID, 1)
				case strings.HasPrefix(operation, "disable"):
					result, err = service.Disable(ctx, uuid.NewString(), account.ID, 1)
				}
				if !errors.Is(err, want) {
					t.Fatalf("operation error=%v, want %v", err, want)
				}
				if want != nil {
					if store.lastOperation.Scope != (Scope{}) || store.readScope != (Scope{}) || store.listScope != (Scope{}) {
						t.Fatal("denied authorization reached persistence")
					}
				} else if strings.HasSuffix(operation, "replay") {
					if !result.Replayed || store.tx.insertCalls != 0 || store.tx.saveCalls != 0 || store.tx.completeCalls != 0 {
						t.Fatal("authorized replay changed persistence")
					}
				}
			})
		}
	}
}

func newTestService(t *testing.T, store Store, authorizer Authorizer, now time.Time) *Service {
	t.Helper()
	service, err := NewService(store, authorizer, WithClock(func() time.Time { return now }), WithIDGenerator(func() (string, error) {
		id, idErr := uuid.NewV7()
		return id.String(), idErr
	}))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

func identityContext(now time.Time, org, actor, role string) context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		TenantID: org, EffectiveOrganizationID: org, UserID: actor, Roles: []string{role}, TokenExpiresAt: now.Add(time.Hour),
	})
}

func validAccount(t *testing.T, now time.Time) Account {
	t.Helper()
	id, err := uuid.NewV7()
	if err != nil {
		t.Fatal(err)
	}
	return Account{ID: id.String(), OrganizationID: "org-b", Platform: Platform1688, DisplayName: "Primary", ManagementStatus: ManagementStatusEnabled, ConnectionStatus: ConnectionStatusPending, Version: 1, CreatedBy: "actor-1", UpdatedBy: "actor-1", CreatedAt: now, UpdatedAt: now}
}

type fakeAuthorizer struct {
	allowed        map[string]bool
	lastUser       string
	lastRoles      []string
	lastPermission string
}

func allowAllAuthorizer() *fakeAuthorizer {
	return &fakeAuthorizer{allowed: map[string]bool{authz.PermissionWorkbenchSourceAccountRead: true, authz.PermissionWorkbenchSourceAccountManage: true}}
}

func (a *fakeAuthorizer) Authorize(user string, roles []string, permission string) bool {
	a.lastUser, a.lastRoles, a.lastPermission = user, append([]string(nil), roles...), permission
	return a.allowed[permission]
}

type fakeStore struct {
	tx            *fakeTx
	lastOperation Operation
	readScope     Scope
	listScope     Scope
	readAccount   Account
	page          Page
}

func (s *fakeStore) Run(_ context.Context, operation Operation, fn func(Transaction) (Account, error)) (MutationResult, error) {
	s.lastOperation = operation
	account, err := fn(s.tx)
	return MutationResult{Account: account, Replayed: s.tx.replayed}, err
}

func (s *fakeStore) Read(_ context.Context, scope Scope, _ string) (Account, error) {
	s.readScope = scope
	return s.readAccount, nil
}

func (s *fakeStore) List(_ context.Context, scope Scope, _ PageRequest) (Page, error) {
	s.listScope = scope
	return s.page, nil
}

type fakeTx struct {
	replay        *Account
	replayed      bool
	count         int
	loaded        Account
	saved         Account
	saveExpected  int64
	countCalls    int
	insertCalls   int
	loadCalls     int
	saveCalls     int
	completeCalls int
}

func (t *fakeTx) Replay() (Account, bool, error) {
	if t.replay == nil {
		return Account{}, false, nil
	}
	t.replayed = true
	return *t.replay, true, nil
}

func (t *fakeTx) CountForCreate() (int, error)            { t.countCalls++; return t.count, nil }
func (t *fakeTx) Insert(account Account) error            { t.insertCalls++; t.saved = account; return nil }
func (t *fakeTx) LoadForUpdate(_ string) (Account, error) { t.loadCalls++; return t.loaded, nil }
func (t *fakeTx) Save(account Account, expectedVersion int64) error {
	t.saveCalls++
	t.saved, t.saveExpected = account, expectedVersion
	return nil
}
func (t *fakeTx) Complete(_ Account) error { t.completeCalls++; return nil }
