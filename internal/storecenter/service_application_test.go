package storecenter

import (
	"context"
	"errors"
	"testing"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

const (
	testServiceOperationID = "11111111-1111-4111-8111-111111111111"
	testServiceStoreID     = "22222222-2222-4222-8222-222222222222"
)

func TestServiceLifecycleApplicationReplaysBeforeVolatileReads(t *testing.T) {
	t.Parallel()
	events := []string{}
	want := ServiceOperationResult{Snapshot: ServiceOperationSnapshot{
		OrganizationID: "org-a", OperationID: testServiceOperationID, StoreID: testServiceStoreID,
		Command: ServiceCommandActivate, Quantity: "1", StoreVersion: 3,
	}, Replayed: true}
	executor := &serviceLifecycleExecutorStub{events: &events, replayResult: want, replayFound: true}
	application, err := NewServiceLifecycleApplication(
		serviceLifecycleStoreReaderFunc(func(context.Context, string, string) (*Store, error) {
			t.Fatal("Store read ran before a durable replay")
			return nil, nil
		}),
		executor,
		connectionStatusProviderFunc(func(context.Context, ConnectionStatusInput) (ConnectionStatus, error) {
			t.Fatal("connection lookup ran before a durable replay")
			return "", nil
		}),
		serviceLifecycleAuthorizerFunc(func(string, []string, string) bool {
			events = append(events, "authorize")
			return true
		}),
		serviceQuantityPolicyFunc(func(context.Context, string, ServiceCommand) (int64, error) {
			t.Fatal("quantity policy ran before a durable replay")
			return 0, nil
		}),
		time.Now,
	)
	if err != nil {
		t.Fatalf("NewServiceLifecycleApplication() error = %v", err)
	}

	got, err := application.Activate(serviceLifecycleContext(), ServiceLifecycleApplicationRequest{
		OperationID: testServiceOperationID, StoreID: testServiceStoreID, ExpectedStoreVersion: 2,
	})
	if err != nil || got != want {
		t.Fatalf("Activate() = (%+v, %v), want replay %+v", got, err, want)
	}
	if len(events) != 2 || events[0] != "authorize" || events[1] != "replay" {
		t.Fatalf("events = %v, want authorization then replay", events)
	}
}

func TestServiceLifecycleApplicationActivationBuildsTrustedExecution(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 4, 5, 6, 7, 0, time.UTC)
	store := serviceLifecycleStore(t, "opaque-connection")
	executor := &serviceLifecycleExecutorStub{}
	var connectionInput ConnectionStatusInput
	application, err := NewServiceLifecycleApplication(
		serviceLifecycleStoreReaderFunc(func(_ context.Context, organizationID, storeID string) (*Store, error) {
			if organizationID != "org-a" || storeID != testServiceStoreID {
				t.Fatalf("Store read identity = %q/%q", organizationID, storeID)
			}
			return store, nil
		}),
		executor,
		connectionStatusProviderFunc(func(_ context.Context, input ConnectionStatusInput) (ConnectionStatus, error) {
			connectionInput = input
			return ConnectionStatusConnected, nil
		}),
		serviceLifecycleAuthorizerFunc(func(userID string, roles []string, permission string) bool {
			return userID == "user-a" && len(roles) == 1 && roles[0] == "listingkit_operator" && permission == authz.PermissionWorkbenchStoreLifecycle
		}),
		serviceQuantityPolicyFunc(func(_ context.Context, organizationID string, command ServiceCommand) (int64, error) {
			if organizationID != "org-a" || command != ServiceCommandActivate {
				t.Fatalf("quantity policy identity = %q/%q", organizationID, command)
			}
			return 12, nil
		}),
		func() time.Time { return now },
	)
	if err != nil {
		t.Fatalf("NewServiceLifecycleApplication() error = %v", err)
	}

	_, err = application.Activate(serviceLifecycleContext(), ServiceLifecycleApplicationRequest{
		OperationID: testServiceOperationID, StoreID: testServiceStoreID, ExpectedStoreVersion: 2,
	})
	if err != nil {
		t.Fatalf("Activate() error = %v", err)
	}
	got := executor.executed
	if got.OrganizationID != "org-a" || got.ActorSubject != "user-a" || got.Command != ServiceCommandActivate || got.Quantity != 1 || got.MaxQuantity != 12 || got.ExpectedStoreVersion != 2 || got.ExpectedConnectionRef != "opaque-connection" || got.ConnectionStatus != ConnectionStatusConnected || !got.OccurredAt.Equal(now) || len(got.RequestFingerprint) != 64 {
		t.Fatalf("execution = %+v, want trusted canonical activation", got)
	}
	if connectionInput.OrganizationID != "org-a" || connectionInput.StoreID != testServiceStoreID || connectionInput.Platform != PlatformShein || connectionInput.ConnectionRef != "opaque-connection" {
		t.Fatalf("connection input = %+v", connectionInput)
	}
}

func TestServiceLifecycleApplicationRejectsUnavailableConnectionBeforeAtomicExecution(t *testing.T) {
	t.Parallel()
	store := serviceLifecycleStore(t, "opaque-connection")
	executor := &serviceLifecycleExecutorStub{}
	application, err := NewServiceLifecycleApplication(
		serviceLifecycleStoreReaderFunc(func(context.Context, string, string) (*Store, error) { return store, nil }),
		executor,
		connectionStatusProviderFunc(func(context.Context, ConnectionStatusInput) (ConnectionStatus, error) {
			return ConnectionStatusUnavailable, nil
		}),
		serviceLifecycleAuthorizerFunc(func(string, []string, string) bool { return true }),
		serviceQuantityPolicyFunc(func(context.Context, string, ServiceCommand) (int64, error) { return 1, nil }),
		time.Now,
	)
	if err != nil {
		t.Fatalf("NewServiceLifecycleApplication() error = %v", err)
	}

	_, err = application.Activate(serviceLifecycleContext(), ServiceLifecycleApplicationRequest{
		OperationID: testServiceOperationID, StoreID: testServiceStoreID, ExpectedStoreVersion: 2,
	})
	if !errors.Is(err, ErrConnectionUnavailable) {
		t.Fatalf("Activate() error = %v, want ErrConnectionUnavailable", err)
	}
	if executor.executed != (ServiceExecution{}) {
		t.Fatalf("unavailable connection reached atomic executor: %+v", executor.executed)
	}
	if executor.replayCalls != 2 {
		t.Fatalf("unavailable connection replay calls = %d, want final durable replay", executor.replayCalls)
	}
}

func TestServiceLifecycleApplicationReplaysConcurrentCommitBeforeReturningConnectionUnavailable(t *testing.T) {
	t.Parallel()
	store := serviceLifecycleStore(t, "opaque-connection")
	want := ServiceOperationResult{Snapshot: ServiceOperationSnapshot{
		OrganizationID: "org-a", OperationID: testServiceOperationID, StoreID: testServiceStoreID,
		Command: ServiceCommandActivate, Quantity: "1", StoreVersion: 3,
	}, Replayed: true}
	executor := &serviceLifecycleExecutorStub{replaySequence: []serviceReplayOutcome{
		{},
		{result: want, found: true},
	}}
	application, err := NewServiceLifecycleApplication(
		serviceLifecycleStoreReaderFunc(func(context.Context, string, string) (*Store, error) { return store, nil }),
		executor,
		connectionStatusProviderFunc(func(context.Context, ConnectionStatusInput) (ConnectionStatus, error) {
			return ConnectionStatusUnavailable, nil
		}),
		serviceLifecycleAuthorizerFunc(func(string, []string, string) bool { return true }),
		serviceQuantityPolicyFunc(func(context.Context, string, ServiceCommand) (int64, error) { return 1, nil }),
		time.Now,
	)
	if err != nil {
		t.Fatalf("NewServiceLifecycleApplication() error = %v", err)
	}

	got, err := application.Activate(serviceLifecycleContext(), ServiceLifecycleApplicationRequest{
		OperationID: testServiceOperationID, StoreID: testServiceStoreID, ExpectedStoreVersion: 2,
	})
	if err != nil || got != want {
		t.Fatalf("Activate() = (%+v, %v), want concurrent replay %+v", got, err, want)
	}
	if executor.executed != (ServiceExecution{}) {
		t.Fatalf("concurrent replay reached atomic executor: %+v", executor.executed)
	}
}

func TestServiceLifecycleApplicationPreservesDisconnectedConnectionForAtomicDecision(t *testing.T) {
	t.Parallel()
	store := serviceLifecycleStore(t, "opaque-connection")
	executor := &serviceLifecycleExecutorStub{executeErr: ErrConnectionNotFresh}
	application, err := NewServiceLifecycleApplication(
		serviceLifecycleStoreReaderFunc(func(context.Context, string, string) (*Store, error) { return store, nil }),
		executor,
		connectionStatusProviderFunc(func(context.Context, ConnectionStatusInput) (ConnectionStatus, error) {
			return ConnectionStatusDisconnected, nil
		}),
		serviceLifecycleAuthorizerFunc(func(string, []string, string) bool { return true }),
		serviceQuantityPolicyFunc(func(context.Context, string, ServiceCommand) (int64, error) { return 1, nil }),
		time.Now,
	)
	if err != nil {
		t.Fatalf("NewServiceLifecycleApplication() error = %v", err)
	}

	_, err = application.Activate(serviceLifecycleContext(), ServiceLifecycleApplicationRequest{
		OperationID: testServiceOperationID, StoreID: testServiceStoreID, ExpectedStoreVersion: 2,
	})
	if !errors.Is(err, ErrConnectionNotFresh) {
		t.Fatalf("Activate() error = %v, want ErrConnectionNotFresh", err)
	}
	if executor.executed.ConnectionStatus != ConnectionStatusDisconnected {
		t.Fatalf("atomic executor connection status = %q, want disconnected", executor.executed.ConnectionStatus)
	}
}

func TestServiceLifecycleApplicationRenewSkipsConnectionLookupAndBindsFingerprint(t *testing.T) {
	t.Parallel()
	store := serviceLifecycleStore(t, "opaque-connection")
	executor := &serviceLifecycleExecutorStub{}
	application, err := NewServiceLifecycleApplication(
		serviceLifecycleStoreReaderFunc(func(context.Context, string, string) (*Store, error) { return store, nil }),
		executor,
		connectionStatusProviderFunc(func(context.Context, ConnectionStatusInput) (ConnectionStatus, error) {
			t.Fatal("Renew queried volatile connection status")
			return "", nil
		}),
		serviceLifecycleAuthorizerFunc(func(string, []string, string) bool { return true }),
		serviceQuantityPolicyFunc(func(context.Context, string, ServiceCommand) (int64, error) { return 12, nil }),
		time.Now,
	)
	if err != nil {
		t.Fatalf("NewServiceLifecycleApplication() error = %v", err)
	}

	_, err = application.Renew(serviceLifecycleContext(), ServiceLifecycleApplicationRequest{
		OperationID: testServiceOperationID, StoreID: testServiceStoreID, Quantity: 3, ExpectedStoreVersion: 2,
	})
	if err != nil {
		t.Fatalf("Renew() error = %v", err)
	}
	first := executor.executed.RequestFingerprint
	if executor.executed.ConnectionStatus != "" || executor.executed.Quantity != 3 || executor.executed.Command != ServiceCommandRenew {
		t.Fatalf("Renew execution = %+v", executor.executed)
	}

	executor.replayFingerprints = nil
	_, err = application.Renew(serviceLifecycleContext(), ServiceLifecycleApplicationRequest{
		OperationID: testServiceOperationID, StoreID: testServiceStoreID, Quantity: 4, ExpectedStoreVersion: 2,
	})
	if err != nil {
		t.Fatalf("changed Renew() error = %v", err)
	}
	if first == executor.executed.RequestFingerprint {
		t.Fatal("quantity change did not change request fingerprint")
	}
}

func TestServiceLifecycleApplicationRejectsUnverifiedOrUnauthorizedIdentity(t *testing.T) {
	t.Parallel()
	reader := serviceLifecycleStoreReaderFunc(func(context.Context, string, string) (*Store, error) {
		t.Fatal("unauthorized request read Store state")
		return nil, nil
	})
	executor := &serviceLifecycleExecutorStub{}
	application, err := NewServiceLifecycleApplication(
		reader,
		executor,
		connectionStatusProviderFunc(func(context.Context, ConnectionStatusInput) (ConnectionStatus, error) { return "", nil }),
		serviceLifecycleAuthorizerFunc(func(string, []string, string) bool { return false }),
		serviceQuantityPolicyFunc(func(context.Context, string, ServiceCommand) (int64, error) { return 12, nil }),
		time.Now,
	)
	if err != nil {
		t.Fatalf("NewServiceLifecycleApplication() error = %v", err)
	}

	// Deliberately invalid activation quantity proves authorization is the first
	// application boundary and no request-shape oracle runs for denied callers.
	request := ServiceLifecycleApplicationRequest{OperationID: testServiceOperationID, StoreID: testServiceStoreID, Quantity: 2, ExpectedStoreVersion: 2}
	if _, err := application.Activate(context.Background(), request); !errors.Is(err, ErrServiceLifecycleIdentityRequired) {
		t.Fatalf("unverified identity error = %v", err)
	}
	if _, err := application.Activate(serviceLifecycleContext(), request); !errors.Is(err, ErrServiceLifecyclePermissionDenied) {
		t.Fatalf("unauthorized identity error = %v", err)
	}
	if executor.replayCalls != 0 {
		t.Fatalf("unauthorized replay calls = %d, want 0", executor.replayCalls)
	}
}

type serviceLifecycleExecutorStub struct {
	events             *[]string
	replayResult       ServiceOperationResult
	replayFound        bool
	replayErr          error
	replayCalls        int
	replayFingerprints []string
	replaySequence     []serviceReplayOutcome
	executed           ServiceExecution
	executeResult      ServiceOperationResult
	executeErr         error
}

type serviceReplayOutcome struct {
	result ServiceOperationResult
	found  bool
	err    error
}

func (s *serviceLifecycleExecutorStub) ReplayServiceLifecycle(_ context.Context, replay ServiceReplay) (ServiceOperationResult, bool, error) {
	callIndex := s.replayCalls
	s.replayCalls++
	s.replayFingerprints = append(s.replayFingerprints, replay.RequestFingerprint)
	if s.events != nil {
		*s.events = append(*s.events, "replay")
	}
	if callIndex < len(s.replaySequence) {
		outcome := s.replaySequence[callIndex]
		return outcome.result, outcome.found, outcome.err
	}
	return s.replayResult, s.replayFound, s.replayErr
}

func (s *serviceLifecycleExecutorStub) ExecuteServiceLifecycle(_ context.Context, execution ServiceExecution) (ServiceOperationResult, error) {
	s.executed = execution
	return s.executeResult, s.executeErr
}

type serviceLifecycleStoreReaderFunc func(context.Context, string, string) (*Store, error)

func (f serviceLifecycleStoreReaderFunc) Get(ctx context.Context, organizationID, storeID string) (*Store, error) {
	return f(ctx, organizationID, storeID)
}

type connectionStatusProviderFunc func(context.Context, ConnectionStatusInput) (ConnectionStatus, error)

func (f connectionStatusProviderFunc) Status(ctx context.Context, input ConnectionStatusInput) (ConnectionStatus, error) {
	return f(ctx, input)
}

type serviceLifecycleAuthorizerFunc func(string, []string, string) bool

func (f serviceLifecycleAuthorizerFunc) Authorize(userID string, roles []string, permission string) bool {
	return f(userID, roles, permission)
}

type serviceQuantityPolicyFunc func(context.Context, string, ServiceCommand) (int64, error)

func (f serviceQuantityPolicyFunc) MaxQuantity(ctx context.Context, organizationID string, command ServiceCommand) (int64, error) {
	return f(ctx, organizationID, command)
}

func serviceLifecycleContext() context.Context {
	return authidentity.WithAuthenticatedIdentity(context.Background(), authidentity.AuthenticatedIdentity{
		UserID: "user-a", EffectiveOrganizationID: "org-a", Roles: []string{"listingkit_operator"},
		OrganizationGrants: []authidentity.OrganizationGrant{{OrganizationID: "org-a", Roles: []string{"listingkit_operator"}}},
	})
}

func serviceLifecycleStore(t *testing.T, connectionRef string) *Store {
	t.Helper()
	now := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC)
	store, err := RehydrateStore(StoreSnapshot{
		ID: testServiceStoreID, OrganizationID: "org-a", Name: "Store", Platform: PlatformShein, Region: "SG",
		LifecycleStatus: StoreStatusActive, ConnectionRef: connectionRef,
		QuotaAllocationID: "33333333-3333-4333-8333-333333333333", Version: 2,
		CreatedBy: "user-a", UpdatedBy: "user-a", CreatedAt: now, UpdatedAt: now,
		CreateIdempotencyKey: "44444444-4444-4444-8444-444444444444",
	})
	if err != nil {
		t.Fatalf("RehydrateStore() error = %v", err)
	}
	return store
}
