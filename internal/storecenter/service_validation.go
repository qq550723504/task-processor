package storecenter

import (
	"errors"
	"github.com/google/uuid"
	"reflect"
	"strconv"
	"time"
)

func normalizeUpdateStoreRequest(request UpdateStoreRequest) (UpdateStoreRequest, error) {
	identity, err := normalizeMutationIdentity(request.OrganizationID, request.ActorSubject, request.StoreID, request.ExpectedVersion)
	if err != nil {
		return UpdateStoreRequest{}, err
	}
	request.OrganizationID, request.ActorSubject, request.StoreID = identity.OrganizationID, identity.ActorSubject, identity.StoreID
	if request.Name, err = normalizeUserValue("name", request.Name, MaxStoreNameCodePoints, true); err != nil {
		return UpdateStoreRequest{}, err
	}
	if request.Region, err = normalizeUserValue("region", request.Region, MaxStoreRegionCodePoints, true); err != nil {
		return UpdateStoreRequest{}, err
	}
	return request, nil
}

func normalizeLifecycleRequest(request StoreLifecycleRequest) (StoreLifecycleRequest, error) {
	identity, err := normalizeMutationIdentity(request.OrganizationID, request.ActorSubject, request.StoreID, request.ExpectedVersion)
	if err != nil {
		return StoreLifecycleRequest{}, err
	}
	request.OrganizationID, request.ActorSubject, request.StoreID = identity.OrganizationID, identity.ActorSubject, identity.StoreID
	return request, nil
}

func normalizeDeleteStoreRequest(request DeleteStoreRequest) (DeleteStoreRequest, error) {
	identity, err := normalizeMutationIdentity(request.OrganizationID, request.ActorSubject, request.StoreID, request.ExpectedVersion)
	if err != nil {
		return DeleteStoreRequest{}, err
	}
	request.OrganizationID, request.ActorSubject, request.StoreID = identity.OrganizationID, identity.ActorSubject, identity.StoreID
	if request.OperationKey, err = canonicalUUID(request.OperationKey); err != nil {
		return DeleteStoreRequest{}, err
	}
	return request, nil
}

type mutationIdentity struct{ OrganizationID, ActorSubject, StoreID string }

func normalizeMutationIdentity(organizationID, actor, storeID string, expectedVersion int64) (mutationIdentity, error) {
	var err error
	if organizationID, err = validateOpaqueIdentity("organization ID", organizationID, MaxOrganizationIDBytes); err != nil {
		return mutationIdentity{}, err
	}
	if actor, err = validateOpaqueIdentity("actor subject", actor, MaxSubjectBytes); err != nil {
		return mutationIdentity{}, err
	}
	if storeID, err = canonicalUUID(storeID); err != nil || expectedVersion <= 0 {
		return mutationIdentity{}, errors.New("invalid versioned store request")
	}
	return mutationIdentity{organizationID, actor, storeID}, nil
}

func deterministicMutationKey(organizationID, storeID, action string, expectedVersion int64) string {
	name := organizationID + "\n" + storeID + "\n" + action + "\n" + strconv.FormatInt(expectedVersion, 10)
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(name)).String()
}

func (s *Service) utcNow() time.Time { return s.now().UTC() }
func (s *Service) monotonicNow(previous time.Time) time.Time {
	now := s.utcNow()
	if now.Before(previous) {
		return previous
	}
	return now
}

func isNilDependency(value any) bool {
	if value == nil {
		return true
	}
	v := reflect.ValueOf(value)
	return (v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface || v.Kind() == reflect.Map || v.Kind() == reflect.Func || v.Kind() == reflect.Slice) && v.IsNil()
}
