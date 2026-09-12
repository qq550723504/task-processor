package sourcing

import (
	"context"
	"errors"
	"time"
)

const (
	AcquisitionTimeout             = 20 * time.Second
	AcquisitionLease               = 30 * time.Second
	MaxAcquisitionOperations       = 256
	MaxActiveAcquisitionOperations = 32
	MaxAcquisitionCommandBytes     = 2 << 20
	AcquisitionProducerKind        = "public_acquisition"
	AcquisitionAcquiring           = "acquiring"
	AcquisitionPrepared            = "prepared"
	AcquisitionPublishing          = "publishing"
	AcquisitionPublished           = "published"
	AcquisitionFailed              = "failed"
)

var (
	ErrAcquisitionUnknown     = errors.New("acquisition outcome unknown")
	ErrAcquisitionConflict    = errors.New("acquisition idempotency conflict")
	ErrAcquisitionCapacity    = errors.New("acquisition resource limit reached")
	ErrAcquisitionNotFound    = errors.New("acquisition operation not found")
	ErrAcquisitionUnavailable = errors.New("acquisition unavailable")
	ErrAcquisitionFailed      = errors.New("acquisition failed")
	ErrAcquisitionFence       = errors.New("acquisition claim superseded")
)

type AcquisitionOperation struct {
	Scope       PublicationScope
	Key         string
	ID          string
	Source      AcquisitionSource
	Fingerprint string
	State       string
	Fence       int64
	LeaseUntil  time.Time
	Command     *PublicationCommand
	CommandHash string
	FailureCode string
}

// AcquisitionOperationStore retains original commands, not Product facts.
// A successful Start/Claim boolean grants exactly one caller the next action.
type AcquisitionOperationStore interface {
	Start(context.Context, AcquisitionOperation) (AcquisitionOperation, bool, error)
	ByKey(context.Context, PublicationScope, string) (AcquisitionOperation, error)
	ByID(context.Context, PublicationScope, string) (AcquisitionOperation, error)
	Prepare(context.Context, AcquisitionOperation, PublicationCommand) (AcquisitionOperation, error)
	Claim(context.Context, AcquisitionOperation) (AcquisitionOperation, bool, error)
	Finish(context.Context, AcquisitionOperation, string, string) error
}

type PublicAcquirer interface {
	Acquire(context.Context, AcquisitionSource) (AcquisitionEvidence, error)
}

type AcquisitionPublisher interface {
	Publish(context.Context, PublicationCommand) (PublicationReceipt, error)
	Verify(context.Context, PublicationCommand) (PublicationReceipt, error)
	Read(context.Context, string) (PersistedPublication, error)
}

type AcquisitionResult struct {
	Operation   AcquisitionOperation
	Replayed    bool
	Publication *PersistedPublication
}
