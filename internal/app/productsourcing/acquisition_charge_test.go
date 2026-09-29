package productsourcing

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"gorm.io/gorm"
	acquisitionstore "task-processor/internal/integration/persistence/product/acquisition"
	"task-processor/internal/ledger/orgresource"
	"task-processor/internal/product/sourcing"
	"testing"
)

// Unit orchestration fixtures use an explicit charge boundary. Real balance
// conservation is exercised in orgresource's SQLite and PostgreSQL tests.
type acquisitionTestChargeCoordinator struct {
	beforeError error
	beforeCalls int
}

func (c *acquisitionTestChargeCoordinator) BeforeDispatch(context.Context, sourcing.AcquisitionOperation) error {
	c.beforeCalls++
	return c.beforeError
}
func (*acquisitionTestChargeCoordinator) Reconcile(context.Context, sourcing.AcquisitionOperation) error {
	return nil
}

type acquisitionTestResourceCharges struct{ db *gorm.DB }

func newTestResourceChargePort(_ *testing.T, db *gorm.DB) orgresource.ConsumerChargePort {
	return acquisitionTestResourceCharges{db: db}
}
func (c acquisitionTestResourceCharges) Reserve(ctx context.Context, identity orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	repo, err := acquisitionstore.NewRepository(ctx, c.db)
	if err != nil {
		return orgresource.ConsumerChargeReceipt{}, err
	}
	intent, err := repo.ReadChargeIntent(ctx, identity.OrganizationID, identity.OperationID)
	if err != nil {
		return orgresource.ConsumerChargeReceipt{}, err
	}
	proof, err := repo.ReadChargeProof(ctx, identity.OrganizationID, identity.OperationID)
	if err != nil {
		return orgresource.ConsumerChargeReceipt{}, err
	}
	reservation := proof.ReservationID
	if reservation == "" {
		encoded, _ := json.Marshal(identity)
		reservation = uuid.NewSHA1(uuid.NameSpaceOID, encoded).String()
	}
	return orgresource.ConsumerChargeReceipt{Intent: productChargeIntent(intent), ReservationID: reservation, State: orgresource.ReservationReserved}, nil
}
func (c acquisitionTestResourceCharges) Reconcile(ctx context.Context, identity orgresource.ConsumerChargeIdentity) (orgresource.ConsumerChargeReceipt, error) {
	repo, err := acquisitionstore.NewRepository(ctx, c.db)
	if err != nil {
		return orgresource.ConsumerChargeReceipt{}, err
	}
	proof, err := repo.ReadChargeProof(ctx, identity.OrganizationID, identity.OperationID)
	if err != nil {
		return orgresource.ConsumerChargeReceipt{}, err
	}
	state := orgresource.ReservationReserved
	if proof.State == "succeeded" {
		state = orgresource.ReservationCommitted
	} else if proof.State == "failed_fenced" {
		state = orgresource.ReservationReleased
	} else {
		return orgresource.ConsumerChargeReceipt{}, orgresource.ErrConsumerChargeUnknown
	}
	return orgresource.ConsumerChargeReceipt{Intent: productChargeIntent(proof.Intent), ReservationID: proof.ReservationID, State: state, OwnerEvidenceID: proof.EvidenceID}, nil
}
