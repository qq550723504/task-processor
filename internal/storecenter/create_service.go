package storecenter

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
)

func (s *Service) Create(ctx context.Context, request CreateStoreRequest) (CreateStoreResult, error) {
	request, err := normalizeCreateStoreRequest(request)
	if err != nil {
		return CreateStoreResult{}, err
	}
	candidate, err := NewStore(CreateStoreInput{ID: uuid.NewString(), OrganizationID: request.OrganizationID, ActorSubject: request.ActorSubject, Name: request.Name, Platform: request.Platform, Region: request.Region, ExternalStoreID: request.ExternalStoreID, CreateIdempotencyKey: request.IdempotencyKey, OccurredAt: s.utcNow()})
	if err != nil {
		return CreateStoreResult{}, err
	}
	stored, replayed, err := s.repository.CreateOrReplay(ctx, request.OrganizationID, candidate)
	if err != nil {
		return CreateStoreResult{}, mapRepositoryFailure(err)
	}
	if stored == nil || stored.OrganizationID() != request.OrganizationID || stored.CreateIdempotencyKey() != request.IdempotencyKey {
		return CreateStoreResult{}, ErrDependencyUnavailable
	}
	return CreateStoreResult{Store: stored, Replayed: replayed}, nil
}
func normalizeCreateStoreRequest(request CreateStoreRequest) (CreateStoreRequest, error) {
	var err error
	if request.OrganizationID, err = validateOpaqueIdentity("organization ID", request.OrganizationID, MaxOrganizationIDBytes); err != nil {
		return CreateStoreRequest{}, err
	}
	if request.ActorSubject, err = validateOpaqueIdentity("actor subject", request.ActorSubject, MaxSubjectBytes); err != nil {
		return CreateStoreRequest{}, err
	}
	if request.IdempotencyKey, err = canonicalUUID(request.IdempotencyKey); err != nil {
		return CreateStoreRequest{}, fmt.Errorf("idempotency key: %w", err)
	}
	if request.Name, err = normalizeUserValue("name", request.Name, MaxStoreNameCodePoints, true); err != nil {
		return CreateStoreRequest{}, err
	}
	if _, err = normalizePlatform(request.Platform); err != nil {
		return CreateStoreRequest{}, err
	}
	request.Platform = string(PlatformShein)
	if request.Region, err = normalizeUserValue("region", request.Region, MaxStoreRegionCodePoints, true); err != nil {
		return CreateStoreRequest{}, err
	}
	if request.ExternalStoreID, err = normalizeUserValue("external store ID", request.ExternalStoreID, MaxExternalStoreIDCodePoints, false); err != nil {
		return CreateStoreRequest{}, err
	}
	return request, nil
}

func mapRepositoryFailure(err error) error {
	if errors.Is(err, ErrAlreadyExists) {
		return ErrAlreadyExists
	}
	if errors.Is(err, ErrNotFound) {
		return ErrNotFound
	}
	return dependencyError(err)
}
func dependencyError(_ error) error { return ErrDependencyUnavailable }
