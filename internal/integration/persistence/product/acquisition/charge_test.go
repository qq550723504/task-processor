package acquisition

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"task-processor/internal/product/sourcing"
)

func TestAcquisitionChargeBindingGuardsOriginalMemberAndLatePublisher(t *testing.T) {
	db := isolatedDatabase(t)
	require.NoError(t, InstallSchema(db))
	repository, err := NewRepository(context.Background(), db)
	require.NoError(t, err)
	op, claimed, err := repository.Start(context.Background(), requestedOperation(t, "org-a", "user-a", uuid.NewString()))
	require.NoError(t, err)
	require.True(t, claimed)
	intent := sourcing.AcquisitionChargeIntent{Scope: op.Scope, OperationID: op.ID, MemberID: "original-membership", Funding: sourcing.AcquisitionFundingMember, Fingerprint: op.Fingerprint, Source: op.Source}
	require.NoError(t, repository.RecordChargeIntent(context.Background(), op, intent))
	changed := intent
	changed.MemberID = "rejoined-membership"
	require.ErrorIs(t, repository.RecordChargeIntent(context.Background(), op, changed), sourcing.ErrAcquisitionConflict)
	reservation := uuid.NewString()
	require.NoError(t, repository.BindChargeReservation(context.Background(), op, intent, reservation))
	require.ErrorIs(t, repository.BindChargeReservation(context.Background(), op, intent, uuid.NewString()), sourcing.ErrAcquisitionConflict)
	command := originalCommand(t, op)
	op, err = repository.Prepare(context.Background(), op, command)
	require.NoError(t, err)
	op, claimed, err = repository.Claim(context.Background(), op)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, repository.Finish(context.Background(), op, sourcing.AcquisitionFailed, "PUBLICATION_CONFLICT"))
	proof, err := repository.ReadChargeProof(context.Background(), op.Scope.OrganizationID, op.ID)
	require.NoError(t, err)
	require.Equal(t, "failed_fenced", proof.State)
	require.Equal(t, reservation, proof.ReservationID)
	require.Equal(t, intent, proof.Intent)
	// Failure and its publisher fence are durable in the same owner transaction.
	publication := sourcing.AtomicPublication{OrganizationID: op.Scope.OrganizationID, ActorID: op.Scope.ActorID, PublicationID: command.PublicationID, InputHash: op.CommandHash, ProductKey: command.ProductKey, Envelope: command.Envelope, Producer: command.Producer}
	require.ErrorIs(t, db.Transaction(func(tx *gorm.DB) error {
		guard := NewPublicationChargeGuard(tx)
		return guard.Lock(context.Background(), publication)
	}), sourcing.ErrAcquisitionFence)
}
