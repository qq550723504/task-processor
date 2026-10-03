package productsourcing

import (
	"context"
	"errors"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/sourcing"
)

type AcquisitionCharges interface {
	BeforeDispatch(context.Context, sourcing.AcquisitionOperation) error
	Reconcile(context.Context, sourcing.AcquisitionOperation) error
}

// Browser Capture is a free, client-captured channel. The common publication
// orchestration still calls Reconcile after reading a durable receipt, but this
// channel must never create or inspect a Resource reservation.
type freeAcquisitionCharges struct{}

func (freeAcquisitionCharges) BeforeDispatch(context.Context, sourcing.AcquisitionOperation) error {
	return nil
}
func (freeAcquisitionCharges) Reconcile(context.Context, sourcing.AcquisitionOperation) error {
	return nil
}

type acquisitionChargeCoordinator struct {
	operations  sourcing.AcquisitionChargeStore
	charges     orgresource.ConsumerChargePort
	permissions *authz.ListingKitAuthorizer
	live        sourcing.LiveOrganizationAccess
}

func newAcquisitionChargeCoordinator(operations sourcing.AcquisitionChargeStore, charges orgresource.ConsumerChargePort, permissions *authz.ListingKitAuthorizer, live sourcing.LiveOrganizationAccess) (AcquisitionCharges, error) {
	if operations == nil || charges == nil || permissions == nil || live == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	return &acquisitionChargeCoordinator{operations: operations, charges: charges, permissions: permissions, live: live}, nil
}
func (c *acquisitionChargeCoordinator) BeforeDispatch(ctx context.Context, op sourcing.AcquisitionOperation) error {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.UserID != op.Scope.ActorID || identity.TenantID != op.Scope.OrganizationID || identity.EffectiveOrganizationID != op.Scope.OrganizationID || !authidentity.IsBoundedIdentifier(identity.EffectiveMemberID) {
		return sourcing.ErrPublicationForbidden
	}
	funding := sourcing.AcquisitionFundingMember
	roles, err := c.live.ResolveLiveRoles(ctx, identity.EffectiveOrganizationID, identity.UserID)
	if err != nil {
		return err
	}
	admin := c.permissions.IsTenantAdmin(identity.UserID, roles)
	if admin {
		funding = sourcing.AcquisitionFundingEnterprise
	}
	intent := sourcing.AcquisitionChargeIntent{Scope: op.Scope, OperationID: op.ID, MemberID: identity.EffectiveMemberID, Funding: funding, Fingerprint: op.Fingerprint, Source: op.Source}
	original, readErr := c.operations.ReadChargeIntent(ctx, op.Scope.OrganizationID, op.ID)
	if readErr == nil {
		if original.Scope != op.Scope || original.OperationID != op.ID || original.MemberID != identity.EffectiveMemberID || original.Fingerprint != op.Fingerprint || original.Source != op.Source {
			return sourcing.ErrAcquisitionConflict
		}
		if original.Funding == sourcing.AcquisitionFundingEnterprise && !admin {
			return sourcing.ErrPublicationForbidden
		}
		intent = original
	} else if !errors.Is(readErr, sourcing.ErrAcquisitionUnknown) {
		return readErr
	}
	if err := c.operations.RecordChargeIntent(ctx, op, intent); err != nil {
		return err
	}
	receipt, err := c.charges.Reserve(ctx, productChargeIdentity(intent))
	if err != nil {
		return err
	}
	if receipt.Intent != productChargeIntent(intent) || receipt.State != orgresource.ReservationReserved {
		return sourcing.ErrAcquisitionUnknown
	}
	return c.operations.BindChargeReservation(ctx, op, intent, receipt.ReservationID)
}
func (c *acquisitionChargeCoordinator) Reconcile(ctx context.Context, op sourcing.AcquisitionOperation) error {
	_, err := c.charges.Reconcile(ctx, orgresource.ConsumerChargeIdentity{OrganizationID: op.Scope.OrganizationID, Consumer: orgresource.ConsumerProductAcquisition, OperationID: op.ID})
	if errors.Is(err, orgresource.ErrConsumerChargeUnknown) {
		return sourcing.ErrAcquisitionUnknown
	}
	return err
}

// This read-only adapter is registered in the Resource owner at composition.
// It exposes immutable Product intent and same-transaction terminal evidence,
// without passing a Resource transaction into the Product database.
type AcquisitionChargeOwner struct {
	operations sourcing.AcquisitionChargeStore
}

func NewAcquisitionChargeOwner(operations sourcing.AcquisitionChargeStore) (*AcquisitionChargeOwner, error) {
	if operations == nil {
		return nil, sourcing.ErrAcquisitionUnavailable
	}
	return &AcquisitionChargeOwner{operations: operations}, nil
}
func (o *AcquisitionChargeOwner) ReadChargeIntent(ctx context.Context, identity orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeIntent, error) {
	if identity.Consumer != orgresource.ConsumerProductAcquisition {
		return orgresource.ConsumerChargeIntent{}, orgresource.ErrOwnerScopeMismatch
	}
	intent, err := o.operations.ReadChargeIntent(ctx, identity.OrganizationID, identity.OperationID)
	if err != nil {
		return orgresource.ConsumerChargeIntent{}, err
	}
	return productChargeIntent(intent), nil
}
func (o *AcquisitionChargeOwner) ReadChargeProof(ctx context.Context, receipt orgresource.ConsumerChargeReceipt) (orgresource.ConsumerChargeProof, error) {
	identity := receipt.Intent.Identity
	if identity.Consumer != orgresource.ConsumerProductAcquisition {
		return orgresource.ConsumerChargeProof{}, orgresource.ErrOwnerScopeMismatch
	}
	proof, err := o.operations.ReadChargeProof(ctx, identity.OrganizationID, identity.OperationID)
	if err != nil {
		return orgresource.ConsumerChargeProof{}, err
	}
	// A missing original reservation binding cannot prove release, even when
	// another caller reports a terminal operation or a lease has expired.
	if proof.ReservationID == "" {
		return orgresource.ConsumerChargeProof{Intent: receipt.Intent, ReservationID: receipt.ReservationID, State: orgresource.ConsumerEffectUnknown}, nil
	}
	return orgresource.ConsumerChargeProof{Intent: productChargeIntent(proof.Intent), ReservationID: proof.ReservationID, State: orgresource.ConsumerEffectState(proof.State), EvidenceID: proof.EvidenceID}, nil
}
func productChargeIdentity(intent sourcing.AcquisitionChargeIntent) orgresource.ConsumerChargeIdentity {
	return orgresource.ConsumerChargeIdentity{OrganizationID: intent.Scope.OrganizationID, Consumer: orgresource.ConsumerProductAcquisition, OperationID: intent.OperationID}
}
func productChargeIntent(intent sourcing.AcquisitionChargeIntent) orgresource.ConsumerChargeIntent {
	return orgresource.ConsumerChargeIntent{Identity: productChargeIdentity(intent), ActorID: intent.Scope.ActorID, MemberID: intent.MemberID, Funding: orgresource.ResourceFunding(intent.Funding), ResourceType: orgresource.ResourceDataRow, Quantity: 1, Fingerprint: intent.Fingerprint, BusinessScope: intent.Source.URL}
}
