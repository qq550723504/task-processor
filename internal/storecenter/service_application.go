package storecenter

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

var (
	ErrServiceLifecycleIdentityRequired = errors.New("verified Store lifecycle identity is required")
	ErrServiceLifecyclePermissionDenied = errors.New("Store lifecycle permission is denied")
)

// ServiceLifecycleApplicationRequest contains only caller-owned, stable
// behavior fields. Organization and actor identities are always taken from the
// verified request context, while the request fingerprint is computed here.
type ServiceLifecycleApplicationRequest struct {
	OperationID          string
	StoreID              string
	Quantity             int64
	ExpectedStoreVersion int64
}

type ServiceLifecycleStoreReader interface {
	Get(context.Context, string, string) (*Store, error)
}

type ServiceLifecycleAuthorizer interface {
	Authorize(userID string, roles []string, permission string) bool
}

// ServiceQuantityPolicy owns the server-side command limit. The limit is not
// accepted from an HTTP or tool caller and is evaluated only after durable
// idempotency replay has missed.
type ServiceQuantityPolicy interface {
	MaxQuantity(context.Context, string, ServiceCommand) (int64, error)
}

// ServiceLifecycleApplication orders authorization, durable replay and
// volatile validation around the atomic Store+Resource executor.
type ServiceLifecycleApplication struct {
	stores      ServiceLifecycleStoreReader
	executor    ServiceLifecycleExecutor
	connections ConnectionStatusProvider
	authorizer  ServiceLifecycleAuthorizer
	policy      ServiceQuantityPolicy
	now         func() time.Time
}

func NewServiceLifecycleApplication(
	stores ServiceLifecycleStoreReader,
	executor ServiceLifecycleExecutor,
	connections ConnectionStatusProvider,
	authorizer ServiceLifecycleAuthorizer,
	policy ServiceQuantityPolicy,
	now func() time.Time,
) (*ServiceLifecycleApplication, error) {
	if isNilDependency(stores) || isNilDependency(executor) || isNilDependency(connections) || isNilDependency(authorizer) || isNilDependency(policy) || now == nil {
		return nil, errors.New("Store lifecycle application dependencies are required")
	}
	return &ServiceLifecycleApplication{
		stores: stores, executor: executor, connections: connections,
		authorizer: authorizer, policy: policy, now: now,
	}, nil
}

func (application *ServiceLifecycleApplication) Activate(ctx context.Context, request ServiceLifecycleApplicationRequest) (ServiceOperationResult, error) {
	return application.execute(ctx, ServiceCommandActivate, request)
}

func (application *ServiceLifecycleApplication) Renew(ctx context.Context, request ServiceLifecycleApplicationRequest) (ServiceOperationResult, error) {
	return application.execute(ctx, ServiceCommandRenew, request)
}

func (application *ServiceLifecycleApplication) Reactivate(ctx context.Context, request ServiceLifecycleApplicationRequest) (ServiceOperationResult, error) {
	return application.execute(ctx, ServiceCommandReactivate, request)
}

func (application *ServiceLifecycleApplication) execute(ctx context.Context, command ServiceCommand, request ServiceLifecycleApplicationRequest) (ServiceOperationResult, error) {
	identity, err := application.authorize(ctx)
	if err != nil {
		return ServiceOperationResult{}, err
	}
	request, err = normalizeServiceLifecycleApplicationRequest(command, request)
	if err != nil {
		return ServiceOperationResult{}, err
	}
	fingerprint := serviceLifecycleRequestFingerprint(identity.EffectiveOrganizationID, command, request)

	replay := ServiceReplay{
		OrganizationID: identity.EffectiveOrganizationID,
		OperationID:    request.OperationID, RequestFingerprint: fingerprint,
	}
	if result, found, replayErr := application.executor.ReplayServiceLifecycle(ctx, replay); replayErr != nil || found {
		return result, replayErr
	}

	maxQuantity, err := application.policy.MaxQuantity(ctx, identity.EffectiveOrganizationID, command)
	if err != nil {
		return application.replayBeforeTransientFailure(ctx, replay, dependencyError(err))
	}
	if err := validateServiceQuantity(request.Quantity, maxQuantity); err != nil {
		return ServiceOperationResult{}, err
	}

	store, err := application.stores.Get(ctx, identity.EffectiveOrganizationID, request.StoreID)
	if errors.Is(err, ErrNotFound) {
		return ServiceOperationResult{}, ErrNotFound
	}
	if err != nil {
		return application.replayBeforeTransientFailure(ctx, replay, dependencyError(err))
	}
	if store == nil || store.OrganizationID() != identity.EffectiveOrganizationID || store.ID() != request.StoreID {
		return application.replayBeforeTransientFailure(ctx, replay, dependencyError(errors.New("Store lifecycle read identity mismatch")))
	}

	connectionStatus := ConnectionStatus("")
	if command == ServiceCommandActivate {
		connectionStatus = resolveConnectionStatus(ctx, application.connections, ConnectionStatusInput{
			OrganizationID: identity.EffectiveOrganizationID,
			StoreID:        request.StoreID,
			Platform:       store.Platform(),
			ConnectionRef:  store.ConnectionRef(),
		}, connectionStatusTimeout)
		if connectionStatus == ConnectionStatusUnavailable {
			return application.replayBeforeTransientFailure(ctx, replay, ErrConnectionUnavailable)
		}
	}

	return application.executor.ExecuteServiceLifecycle(ctx, ServiceExecution{
		OrganizationID:        identity.EffectiveOrganizationID,
		OperationID:           request.OperationID,
		StoreID:               request.StoreID,
		Command:               command,
		Quantity:              request.Quantity,
		MaxQuantity:           maxQuantity,
		ExpectedStoreVersion:  request.ExpectedStoreVersion,
		ExpectedConnectionRef: store.ConnectionRef(),
		ConnectionStatus:      connectionStatus,
		ActorSubject:          identity.UserID,
		OccurredAt:            application.now().UTC(),
		RequestFingerprint:    fingerprint,
	})
}

func (application *ServiceLifecycleApplication) replayBeforeTransientFailure(ctx context.Context, replay ServiceReplay, fallback error) (ServiceOperationResult, error) {
	if result, found, replayErr := application.executor.ReplayServiceLifecycle(ctx, replay); replayErr != nil || found {
		return result, replayErr
	}
	return ServiceOperationResult{}, fallback
}

func (application *ServiceLifecycleApplication) authorize(ctx context.Context) (authidentity.AuthenticatedIdentity, error) {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || strings.TrimSpace(identity.EffectiveOrganizationID) == "" || !hasEffectiveOrganizationGrant(identity) {
		return authidentity.AuthenticatedIdentity{}, ErrServiceLifecycleIdentityRequired
	}
	if !application.authorizer.Authorize(identity.UserID, identity.Roles, authz.PermissionWorkbenchStoreLifecycle) {
		return authidentity.AuthenticatedIdentity{}, ErrServiceLifecyclePermissionDenied
	}
	return identity, nil
}

func hasEffectiveOrganizationGrant(identity authidentity.AuthenticatedIdentity) bool {
	for _, grant := range identity.OrganizationGrants {
		if grant.OrganizationID == identity.EffectiveOrganizationID {
			return true
		}
	}
	return false
}

func normalizeServiceLifecycleApplicationRequest(command ServiceCommand, request ServiceLifecycleApplicationRequest) (ServiceLifecycleApplicationRequest, error) {
	var err error
	if request.OperationID, err = canonicalUUID(request.OperationID); err != nil {
		return ServiceLifecycleApplicationRequest{}, err
	}
	if request.StoreID, err = canonicalUUID(request.StoreID); err != nil {
		return ServiceLifecycleApplicationRequest{}, err
	}
	if request.ExpectedStoreVersion <= 0 || request.Quantity <= 0 {
		if command != ServiceCommandActivate || request.ExpectedStoreVersion <= 0 || request.Quantity != 0 {
			return ServiceLifecycleApplicationRequest{}, ErrServiceQuantityInvalid
		}
	}
	switch command {
	case ServiceCommandActivate:
		if request.Quantity != 0 {
			return ServiceLifecycleApplicationRequest{}, ErrServiceQuantityInvalid
		}
		request.Quantity = 1
	case ServiceCommandRenew, ServiceCommandReactivate:
	default:
		return ServiceLifecycleApplicationRequest{}, ErrInvalidServiceTransition
	}
	return request, nil
}

func serviceLifecycleRequestFingerprint(organizationID string, command ServiceCommand, request ServiceLifecycleApplicationRequest) string {
	return hashTuple(
		"store-service-lifecycle-v1",
		organizationID,
		string(command),
		request.StoreID,
		strconv.FormatInt(request.Quantity, 10),
		strconv.FormatInt(request.ExpectedStoreVersion, 10),
		"service-period-days=30",
	)
}
