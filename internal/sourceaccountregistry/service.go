package sourceaccountregistry

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"reflect"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
)

const Timeout = 10 * time.Second

type Scope struct {
	OrganizationID string
	ActorSubject   string
}

type OperationKind string

const (
	OperationRegister OperationKind = "register"
	OperationEnable   OperationKind = "enable"
	OperationDisable  OperationKind = "disable"
)

type Operation struct {
	Scope       Scope
	Key         string
	Kind        OperationKind
	Fingerprint string
}

type PagePosition struct {
	CreatedAt time.Time
	ID        string
}

type PageRequest struct {
	Limit int
	After *PagePosition
}

type Page struct {
	Items []Account
	Next  *PagePosition
}

type MutationResult struct {
	Account  Account
	Replayed bool
}

type Transaction interface {
	Replay() (Account, bool, error)
	CountForCreate() (int, error)
	Insert(Account) error
	LoadForUpdate(string) (Account, error)
	Save(Account, int64) error
	Complete(Account) error
}

type Store interface {
	Run(context.Context, Operation, func(Transaction) (Account, error)) (MutationResult, error)
	Read(context.Context, Scope, string) (Account, error)
	List(context.Context, Scope, PageRequest) (Page, error)
}

type Authorizer interface {
	Authorize(string, []string, string) bool
}

type RegisterInput struct {
	DisplayName string
	Platform    string
}

type ServiceOption func(*Service)

func WithClock(now func() time.Time) ServiceOption {
	return func(service *Service) {
		if now != nil {
			service.now = now
		}
	}
}

func WithIDGenerator(generate func() (string, error)) ServiceOption {
	return func(service *Service) {
		if generate != nil {
			service.generateID = generate
		}
	}
}

type Service struct {
	store      Store
	authorizer Authorizer
	now        func() time.Time
	generateID func() (string, error)
}

func NewService(store Store, authorizer Authorizer, options ...ServiceOption) (*Service, error) {
	if isNilInterface(store) || isNilInterface(authorizer) {
		return nil, ErrUnavailable
	}
	service := &Service{
		store: store, authorizer: authorizer, now: time.Now,
		generateID: func() (string, error) {
			id, err := uuid.NewV7()
			return id.String(), err
		},
	}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

func (s *Service) Register(ctx context.Context, key string, input RegisterInput) (MutationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, authz.PermissionWorkbenchSourceAccountManage)
	if err != nil {
		return MutationResult{}, err
	}
	if !validCanonicalUUID(key) || input.Platform != string(Platform1688) {
		return MutationResult{}, ErrInvalid
	}
	input.DisplayName, err = NormalizeDisplayName(input.DisplayName)
	if err != nil {
		return MutationResult{}, err
	}
	operation, err := newOperation(scope, key, OperationRegister, input.DisplayName, input.Platform)
	if err != nil {
		return MutationResult{}, err
	}
	result, err := s.store.Run(ctx, operation, func(tx Transaction) (Account, error) {
		if replay, found, replayErr := tx.Replay(); replayErr != nil || found {
			if replayErr != nil {
				return Account{}, replayErr
			}
			return validateScopedAccount(replay, scope, "")
		}
		count, countErr := tx.CountForCreate()
		if countErr != nil {
			return Account{}, countErr
		}
		if count >= MaxAccountsPerOrganization {
			return Account{}, ErrResourceLimitReached
		}
		if count < 0 {
			return Account{}, ErrUnavailable
		}
		id, generateErr := s.generateID()
		if generateErr != nil {
			return Account{}, ErrUnavailable
		}
		account, accountErr := NewAccount(id, scope.OrganizationID, scope.ActorSubject, input.DisplayName, Platform(input.Platform), s.now())
		if accountErr != nil {
			return Account{}, accountErr
		}
		if insertErr := tx.Insert(account); insertErr != nil {
			return Account{}, insertErr
		}
		if completeErr := tx.Complete(account); completeErr != nil {
			return Account{}, completeErr
		}
		return account, nil
	})
	if err != nil {
		return MutationResult{}, err
	}
	validated, err := validateScopedAccount(result.Account, scope, "")
	if err != nil {
		return MutationResult{}, err
	}
	result.Account = validated
	return result, nil
}

func (s *Service) Enable(ctx context.Context, key, id string, expectedVersion int64) (MutationResult, error) {
	return s.transition(ctx, key, id, expectedVersion, ManagementStatusEnabled, OperationEnable)
}

func (s *Service) Disable(ctx context.Context, key, id string, expectedVersion int64) (MutationResult, error) {
	return s.transition(ctx, key, id, expectedVersion, ManagementStatusDisabled, OperationDisable)
}

func (s *Service) transition(ctx context.Context, key, id string, expectedVersion int64, target ManagementStatus, kind OperationKind) (MutationResult, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, authz.PermissionWorkbenchSourceAccountManage)
	if err != nil {
		return MutationResult{}, err
	}
	if !validCanonicalUUID(key) || !validUUIDv7(id) || expectedVersion <= 0 {
		return MutationResult{}, ErrInvalid
	}
	operation, err := newOperation(scope, key, kind, id, expectedVersion, target)
	if err != nil {
		return MutationResult{}, err
	}
	result, err := s.store.Run(ctx, operation, func(tx Transaction) (Account, error) {
		if replay, found, replayErr := tx.Replay(); replayErr != nil || found {
			if replayErr != nil {
				return Account{}, replayErr
			}
			return validateScopedAccount(replay, scope, id)
		}
		current, loadErr := tx.LoadForUpdate(id)
		if loadErr != nil {
			return Account{}, loadErr
		}
		current, loadErr = validateScopedAccount(current, scope, id)
		if loadErr != nil {
			return Account{}, loadErr
		}
		updated, transitionErr := current.Transition(expectedVersion, target, scope.ActorSubject, s.now())
		if transitionErr != nil {
			return Account{}, transitionErr
		}
		if saveErr := tx.Save(updated, expectedVersion); saveErr != nil {
			return Account{}, saveErr
		}
		if completeErr := tx.Complete(updated); completeErr != nil {
			return Account{}, completeErr
		}
		return updated, nil
	})
	if err != nil {
		return MutationResult{}, err
	}
	result.Account, err = validateScopedAccount(result.Account, scope, id)
	if err != nil {
		return MutationResult{}, err
	}
	return result, nil
}

func (s *Service) Get(ctx context.Context, id string) (Account, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, authz.PermissionWorkbenchSourceAccountRead)
	if err != nil {
		return Account{}, err
	}
	if !validUUIDv7(id) {
		return Account{}, ErrInvalid
	}
	account, err := s.store.Read(ctx, scope, id)
	if err != nil {
		return Account{}, err
	}
	return validateScopedAccount(account, scope, id)
}

func (s *Service) List(ctx context.Context, request PageRequest) (Page, error) {
	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()
	scope, err := s.authorize(ctx, authz.PermissionWorkbenchSourceAccountRead)
	if err != nil {
		return Page{}, err
	}
	if request.Limit < 1 || request.Limit > MaxPageLimit || request.After != nil && (request.After.CreatedAt.IsZero() || !validUUIDv7(request.After.ID)) {
		return Page{}, ErrInvalid
	}
	page, err := s.store.List(ctx, scope, request)
	if err != nil {
		return Page{}, err
	}
	if len(page.Items) > request.Limit {
		return Page{}, ErrUnavailable
	}
	for index, account := range page.Items {
		page.Items[index], err = validateScopedAccount(account, scope, "")
		if err != nil {
			return Page{}, err
		}
	}
	if page.Items == nil {
		page.Items = []Account{}
	}
	if page.Next != nil && (page.Next.CreatedAt.IsZero() || !validUUIDv7(page.Next.ID)) {
		return Page{}, ErrUnavailable
	}
	return page, nil
}

func (s *Service) authorize(ctx context.Context, permission string) (Scope, error) {
	if err := ctx.Err(); err != nil {
		return Scope{}, err
	}
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.TokenExpiresAt.IsZero() || !s.now().Before(identity.TokenExpiresAt) {
		return Scope{}, ErrAuthenticationRequired
	}
	if !validScopeValue(identity.EffectiveOrganizationID, MaxOrganizationIDBytes) || !validScopeValue(identity.UserID, MaxActorSubjectBytes) || identity.TenantID != identity.EffectiveOrganizationID {
		return Scope{}, ErrForbidden
	}
	if !s.authorizer.Authorize("", identity.Roles, permission) {
		return Scope{}, ErrForbidden
	}
	return Scope{OrganizationID: identity.EffectiveOrganizationID, ActorSubject: identity.UserID}, nil
}

func validateScopedAccount(account Account, scope Scope, id string) (Account, error) {
	if err := account.Validate(); err != nil || account.OrganizationID != scope.OrganizationID || id != "" && account.ID != id {
		return Account{}, ErrUnavailable
	}
	return account, nil
}

func newOperation(scope Scope, key string, kind OperationKind, publicInput ...any) (Operation, error) {
	if !validCanonicalUUID(key) || !validScopeValue(scope.OrganizationID, MaxOrganizationIDBytes) || !validScopeValue(scope.ActorSubject, MaxActorSubjectBytes) {
		return Operation{}, ErrInvalid
	}
	parts := append([]any{kind}, publicInput...)
	hash := sha256.New()
	var length [8]byte
	for _, part := range parts {
		encoded, err := json.Marshal(part)
		if err != nil {
			return Operation{}, ErrInvalid
		}
		binary.BigEndian.PutUint64(length[:], uint64(len(encoded)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write(encoded)
	}
	return Operation{Scope: scope, Key: key, Kind: kind, Fingerprint: hex.EncodeToString(hash.Sum(nil))}, nil
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}
