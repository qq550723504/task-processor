package submission

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"reflect"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/authidentity"
)

const ExecutionTimeout = 10 * time.Second

type ExecutionRepository interface {
	Acquire(context.Context, ExecutionReservation) (ExecutionAttempt, bool, error)
	Get(context.Context, ExecutionScope, string) (ExecutionAttempt, error)
	Renew(context.Context, ExecutionClaim, time.Time, time.Time) (ExecutionAttempt, error)
	MarkUnknown(context.Context, ExecutionClaim, UnknownReason, time.Time) (ExecutionAttempt, error)
	Complete(context.Context, ExecutionClaim, ExecutionEvidence, time.Time) (ExecutionAttempt, error)
	ResolveUnknown(context.Context, ExecutionScope, string, int64, ExecutionEvidence, time.Time) (ExecutionAttempt, error)
	Expire(context.Context, ExecutionScope, string, time.Time) (ExecutionAttempt, error)
}

type ExecutionKernelOption func(*ExecutionKernel)

type ManualResolutionAuthorizer interface {
	AuthorizeManualResolution(context.Context, ExecutionScope, string) (string, error)
}

func WithExecutionClock(now func() time.Time) ExecutionKernelOption {
	return func(kernel *ExecutionKernel) {
		if now != nil {
			kernel.now = now
		}
	}
}

func WithExecutionIDGenerator(generate func() (string, error)) ExecutionKernelOption {
	return func(kernel *ExecutionKernel) {
		if generate != nil {
			kernel.generateID = generate
		}
	}
}

func WithExecutionClaimTokenGenerator(generate func() (string, error)) ExecutionKernelOption {
	return func(kernel *ExecutionKernel) {
		if generate != nil {
			kernel.generateClaimToken = generate
		}
	}
}

func WithManualResolutionAuthorizer(authorizer ManualResolutionAuthorizer) ExecutionKernelOption {
	return func(kernel *ExecutionKernel) {
		if !isNilManualResolutionAuthorizer(authorizer) {
			kernel.manualResolutionAuthorizer = authorizer
		}
	}
}

type ExecutionKernel struct {
	repository                 ExecutionRepository
	now                        func() time.Time
	generateID                 func() (string, error)
	generateClaimToken         func() (string, error)
	manualResolutionAuthorizer ManualResolutionAuthorizer
}

func NewExecutionKernel(repository ExecutionRepository, options ...ExecutionKernelOption) (*ExecutionKernel, error) {
	if isNilExecutionRepository(repository) {
		return nil, ErrExecutionUnavailable
	}
	kernel := &ExecutionKernel{
		repository: repository,
		now:        time.Now,
		generateID: func() (string, error) {
			id, err := uuid.NewV7()
			return id.String(), err
		},
		generateClaimToken: func() (string, error) {
			var token [32]byte
			if _, err := rand.Read(token[:]); err != nil {
				return "", err
			}
			return base64.RawURLEncoding.EncodeToString(token[:]), nil
		},
	}
	for _, option := range options {
		if option != nil {
			option(kernel)
		}
	}
	return kernel, nil
}

func (k *ExecutionKernel) Acquire(ctx context.Context, command AcquireExecutionCommand) (ExecutionAcquisition, error) {
	ctx, cancel, err := executionContext(ctx)
	if err != nil {
		return ExecutionAcquisition{}, err
	}
	defer cancel()
	if k == nil || isNilExecutionRepository(k.repository) {
		return ExecutionAcquisition{}, ErrExecutionUnavailable
	}
	attemptID, err := k.generateID()
	if err != nil {
		return ExecutionAcquisition{}, ErrExecutionUnavailable
	}
	claimToken, err := k.generateClaimToken()
	if err != nil {
		return ExecutionAcquisition{}, ErrExecutionUnavailable
	}
	reservation, err := NewExecutionReservation(command, attemptID, claimToken, k.now())
	if err != nil {
		return ExecutionAcquisition{}, err
	}
	attempt, acquired, err := k.repository.Acquire(ctx, reservation)
	if err != nil {
		return ExecutionAcquisition{}, err
	}
	if err := ValidateExecutionReplay(attempt, reservation.Attempt); err != nil || ValidatePersistedExecutionAttempt(attempt) != nil {
		return ExecutionAcquisition{}, ErrExecutionUnavailable
	}
	result := ExecutionAcquisition{Attempt: attempt, Replayed: !acquired}
	if !acquired {
		return result, nil
	}
	if attempt.AttemptID != reservation.Attempt.AttemptID || attempt.Status != ExecutionClaimed {
		return ExecutionAcquisition{}, ErrExecutionUnavailable
	}
	if !canonicalExecutionTime(k.now()).Before(attempt.LeaseExpiresAt) {
		return ExecutionAcquisition{}, ErrExecutionClaimRejected
	}
	result.Permit = &SendPermit{
		AttemptID: attempt.AttemptID, FenceEpoch: attempt.FenceEpoch, ClaimToken: claimToken,
		ClaimOwnerID: attempt.ClaimOwnerID, ProviderExecutionKey: attempt.ProviderExecutionKey, LeaseExpiresAt: attempt.LeaseExpiresAt,
	}
	return result, nil
}

func (k *ExecutionKernel) Get(ctx context.Context, scope ExecutionScope, attemptID string) (ExecutionAttempt, error) {
	ctx, cancel, err := executionContext(ctx)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	defer cancel()
	if k == nil || !validExecutionScope(scope) || !validExecutionAttemptID(attemptID) {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	attempt, err := k.repository.Get(ctx, scope, attemptID)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	if ValidatePersistedExecutionAttempt(attempt) != nil || attempt.OrganizationID != scope.OrganizationID || attempt.AttemptID != attemptID {
		return ExecutionAttempt{}, ErrExecutionUnavailable
	}
	return attempt, nil
}

func (k *ExecutionKernel) Renew(ctx context.Context, claim ExecutionClaim, lease time.Duration) (ExecutionAttempt, error) {
	ctx, cancel, err := executionContext(ctx)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	defer cancel()
	if k == nil || !validExecutionClaim(claim) || lease < MinExecutionLease || lease > MaxExecutionLease {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	now := canonicalExecutionTime(k.now())
	renewed, err := k.repository.Renew(ctx, claim, canonicalExecutionTime(now.Add(lease)), now)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	if !canonicalExecutionTime(k.now()).Before(renewed.LeaseExpiresAt) {
		return ExecutionAttempt{}, ErrExecutionClaimRejected
	}
	return renewed, nil
}

func (k *ExecutionKernel) MarkUnknown(ctx context.Context, claim ExecutionClaim, reason UnknownReason) (ExecutionAttempt, error) {
	ctx, cancel, err := executionContext(ctx)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	defer cancel()
	if k == nil || !validExecutionClaim(claim) || !validUnknownReason(reason) {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	return k.repository.MarkUnknown(ctx, claim, reason, canonicalExecutionTime(k.now()))
}

func (k *ExecutionKernel) Complete(ctx context.Context, claim ExecutionClaim, evidence ExecutionEvidence) (ExecutionAttempt, error) {
	ctx, cancel, err := executionContext(ctx)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	defer cancel()
	if k == nil || !validExecutionClaim(claim) {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	return k.repository.Complete(ctx, claim, evidence, canonicalExecutionTime(k.now()))
}

func (k *ExecutionKernel) ResolveUnknown(ctx context.Context, scope ExecutionScope, attemptID string, fenceEpoch int64, evidence ExecutionEvidence) (ExecutionAttempt, error) {
	ctx, cancel, err := executionContext(ctx)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	defer cancel()
	if k == nil || !validExecutionScope(scope) || !validExecutionAttemptID(attemptID) || fenceEpoch <= 0 {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	if evidence.Kind == EvidenceManualResolution {
		if evidence.AuthorizedBy != "" {
			return ExecutionAttempt{}, ErrExecutionInvalid
		}
		if isNilManualResolutionAuthorizer(k.manualResolutionAuthorizer) {
			return ExecutionAttempt{}, ErrExecutionEvidenceRequired
		}
		authorizedBy, authorizeErr := k.manualResolutionAuthorizer.AuthorizeManualResolution(ctx, scope, attemptID)
		if authorizeErr != nil {
			return ExecutionAttempt{}, authorizeErr
		}
		if !authidentity.IsBoundedIdentifier(authorizedBy) {
			return ExecutionAttempt{}, ErrExecutionEvidenceRequired
		}
		evidence.AuthorizedBy = authorizedBy
	}
	return k.repository.ResolveUnknown(ctx, scope, attemptID, fenceEpoch, evidence, canonicalExecutionTime(k.now()))
}

func (k *ExecutionKernel) Expire(ctx context.Context, scope ExecutionScope, attemptID string) (ExecutionAttempt, error) {
	ctx, cancel, err := executionContext(ctx)
	if err != nil {
		return ExecutionAttempt{}, err
	}
	defer cancel()
	if k == nil || !validExecutionScope(scope) || !validExecutionAttemptID(attemptID) {
		return ExecutionAttempt{}, ErrExecutionInvalid
	}
	return k.repository.Expire(ctx, scope, attemptID, canonicalExecutionTime(k.now()))
}

func validExecutionScope(scope ExecutionScope) bool {
	return authidentity.IsBoundedIdentifier(scope.OrganizationID)
}

func validExecutionClaim(claim ExecutionClaim) bool {
	return validExecutionScope(claim.Scope) && validExecutionAttemptID(claim.AttemptID) && claim.FenceEpoch > 0 &&
		authidentity.IsBoundedIdentifier(claim.OwnerID) && claim.Token != "" && len(claim.Token) <= 512
}

func executionContext(ctx context.Context) (context.Context, context.CancelFunc, error) {
	if ctx == nil {
		return nil, nil, ErrExecutionInvalid
	}
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	bounded, cancel := context.WithTimeout(ctx, ExecutionTimeout)
	return bounded, cancel, nil
}

func isNilExecutionRepository(repository ExecutionRepository) bool {
	if repository == nil {
		return true
	}
	value := reflect.ValueOf(repository)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func isNilManualResolutionAuthorizer(authorizer ManualResolutionAuthorizer) bool {
	if authorizer == nil {
		return true
	}
	value := reflect.ValueOf(authorizer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
