// Package inviteflow owns invitation consent and effect receipts. It does not
// own users or memberships, which remain ZITADEL facts.
package inviteflow

import (
	"context"
	"errors"
	"task-processor/internal/authidentity"
	"time"
)

var (
	ErrInvalid     = errors.New("invalid invitation")
	ErrPermission  = errors.New("invitation permission denied")
	ErrConflict    = errors.New("invitation conflict")
	ErrNotFound    = errors.New("invitation not found")
	ErrUnavailable = errors.New("invitation unavailable")
)

const (
	Pending   = "pending"
	Accepting = "accepting"
	Accepted  = "accepted"
	Declined  = "declined"
	Cancelled = "cancelled"
	Expired   = "expired"
)

type Invitation struct {
	ID                string     `json:"id"`
	ProjectID         string     `json:"-"`
	OrganizationID    string     `json:"organizationId"`
	OrganizationName  string     `json:"organizationName"`
	CreatorID         string     `json:"creatorId"`
	Contact           string     `json:"contact"`
	Role              string     `json:"role"`
	Fingerprint       string     `json:"-"`
	State             string     `json:"state"`
	Revision          int64      `json:"revision"`
	CreatedAt         time.Time  `json:"createdAt"`
	ExpiresAt         time.Time  `json:"expiresAt"`
	RecipientID       string     `json:"recipientId"`
	DispatchID        string     `json:"-"`
	AuthorizationID   string     `json:"authorizationId"`
	DeliveryState     string     `json:"deliveryState"`
	DeliveryAttempt   string     `json:"-"`
	DeliveryAttempts  int        `json:"deliveryAttempts"`
	DeliveryUpdatedAt *time.Time `json:"deliveryUpdatedAt"`
}
type Change struct {
	State, RecipientID, DispatchID, AuthorizationID string
	Now                                             time.Time
}
type Page struct {
	Items          []Invitation
	Total, Pending int
}
type Store interface {
	Create(context.Context, Invitation) (Invitation, bool, error)
	Read(context.Context, string) (Invitation, error)
	List(context.Context, string, int, int) (Page, error)
	Change(context.Context, string, int64, Change) (Invitation, error)
	ClaimDelivery(context.Context, string, string, time.Time) (Invitation, error)
	FinishDelivery(context.Context, string, string, string, time.Time) (Invitation, error)
}
type Grant struct {
	Found     bool
	ID, State string
	Roles     []string
}
type Dependencies struct {
	Store       Store
	Manager     func(context.Context, string) (authidentity.AuthenticatedIdentity, error)
	Reader      func(context.Context) (authidentity.AuthenticatedIdentity, error)
	RoleAllowed func(string) bool
	ReadSelf    func(context.Context) (authidentity.SelfProfile, error)
	ReadGrant   func(context.Context, string, string) (Grant, error)
	Authorize   func(string, []string, string) bool
	WriteGrant  func(context.Context, Invitation) error
	Notify      func(context.Context, Invitation) error
	Now         func() time.Time
}
type Service struct {
	Dependencies
	ProjectID string
}

func New(project string, deps Dependencies) *Service {
	return &Service{Dependencies: deps, ProjectID: project}
}

func Transition(inv Invitation, change Change) (Invitation, error) {
	if inv.Revision < 1 || change.Now.IsZero() {
		return Invitation{}, ErrInvalid
	}
	switch change.State {
	case Accepting:
		if inv.State != Pending || !change.Now.Before(inv.ExpiresAt) || !authidentity.IsBoundedIdentifier(change.RecipientID) || !authidentity.IsBoundedIdentifier(change.DispatchID) {
			return Invitation{}, ErrConflict
		}
		inv.RecipientID = change.RecipientID
		inv.DispatchID = change.DispatchID
	case Accepted:
		if inv.State != Accepting || change.RecipientID != inv.RecipientID || !authidentity.IsBoundedIdentifier(change.AuthorizationID) {
			return Invitation{}, ErrConflict
		}
		inv.AuthorizationID = change.AuthorizationID
	case Declined, Cancelled:
		if inv.State != Pending || !change.Now.Before(inv.ExpiresAt) {
			return Invitation{}, ErrConflict
		}
		if change.State == Declined {
			if !authidentity.IsBoundedIdentifier(change.RecipientID) {
				return Invitation{}, ErrInvalid
			}
			inv.RecipientID = change.RecipientID
		}
	case Expired:
		if inv.State != Pending || change.Now.Before(inv.ExpiresAt) {
			return Invitation{}, ErrConflict
		}
	default:
		return Invitation{}, ErrInvalid
	}
	inv.State = change.State
	inv.Revision++
	return inv, nil
}
