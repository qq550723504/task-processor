package storecenter

import (
	"context"
	"time"
)

const (
	ObservationPurposeProducts = "store_products_read"
	ObservationPurposeOrders   = "store_orders_read"
)

type ObservationSubject struct {
	OrganizationID, ActorID, MemberID, Purpose string
	Sync                                       bool
}
type ObservationAuthorization struct {
	Access  StoreMemberAccess
	Allowed bool
}
type ObservationAuthorizer interface {
	AuthorizeObservation(context.Context, ObservationSubject) (ObservationAuthorization, error)
}
type ObservationReader interface {
	ReadObservation(context.Context, ObservationSubject, string, ObservationAuthorizer, time.Time) (ProductExecutionMaterial, error)
}

// Independent readonly admission never substitutes a publish/rules purpose.
func (r *MemberScopedStoreRepository) ReadObservation(ctx context.Context, subject ObservationSubject, storeID string, a ObservationAuthorizer, now time.Time) (ProductExecutionMaterial, error) {
	if isNilDependency(a) || (subject.Purpose != ObservationPurposeProducts && subject.Purpose != ObservationPurposeOrders) {
		return ProductExecutionMaterial{}, ErrNotFound
	}
	return r.readOfficialMaterial(ctx, StoreMemberAccess{OrganizationID: subject.OrganizationID, ActorID: subject.ActorID, MemberID: subject.MemberID}, storeID, now, func(ctx context.Context) (ProductExecutionAuthorization, error) {
		approved, e := a.AuthorizeObservation(ctx, subject)
		return ProductExecutionAuthorization{Access: approved.Access, Allowed: approved.Allowed}, e
	})
}
