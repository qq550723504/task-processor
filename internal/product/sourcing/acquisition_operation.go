package sourcing

import (
	"context"
	"errors"
	"time"
)

const (
	AcquisitionTimeout = 20 * time.Second
	AcquisitionLease   = 30 * time.Second
	// MaxAcquisitionOperations bounds every operation row an organization may create over
	// its lifetime. Rows are never deleted (no key GC) and terminal rows keep counting, so
	// this is a hard ceiling, not a soft threshold.
	//
	// 2048 is sized from observed storage, not from a scale assumption: a published command
	// measures ~18 KiB (browser-capture envelope after normalization), so a heavy
	// organization holds roughly 36 MiB. At a batch size of 10 that is 204 batches.
	//
	// This is explicitly NOT a storage bound. MaxAcquisitionCommandBytes admits commands up
	// to 2 MiB, so an organization that submits near-limit commands could reach 2048 * 2 MiB
	// = 4 GiB. The previous 256-row ceiling admitted 512 MiB by the same arithmetic, so this
	// change raises the admitted worst case by 8x; bounding that would require byte-based
	// accounting, which is deliberately not introduced here.
	//
	// Raising this only moves the ceiling; it does not remove it. Removing it requires
	// reclaiming terminal rows that can never be replayed (see the batch local-agent design).
	MaxAcquisitionOperations       = 2048
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
	Scope         PublicationScope
	Key           string
	ID            string
	Source        AcquisitionSource
	Fingerprint   string
	CaptureSHA256 string
	State         string
	Fence         int64
	LeaseUntil    time.Time
	Command       *PublicationCommand
	CommandHash   string
	FailureCode   string
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

// CapacityReadOperationStore optionally reports whether an organization still
// has acquisition capacity, so a caller can reject before doing expensive
// external work. It is an optimization, not a correctness gate: the atomic
// capacity check inside Start/StartPrepared remains authoritative.
type CapacityReadOperationStore interface {
	// CapacityAdmitted reports whether the organization is below both the
	// retained-operation and active-operation ceilings.
	CapacityAdmitted(context.Context, PublicationScope) (bool, error)
}

// PreparedAcquisitionOperationStore atomically admits an operation with its
// durable publication command. Browser capture uses this contract because its
// receiver can recover the command after a process or response failure.
type PreparedAcquisitionOperationStore interface {
	AcquisitionOperationStore
	StartPrepared(context.Context, AcquisitionOperation, PublicationCommand) (AcquisitionOperation, bool, error)
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
