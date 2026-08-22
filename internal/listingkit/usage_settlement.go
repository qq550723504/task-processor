package listingkit

import (
	"context"
	"strings"
	"time"
)

const (
	generationUsageModuleCode = "studio"
	generationUsageMetric     = "studio_design_jobs_succeeded"
	generationUsageSourceType = "listingkit_generation"
)

type generationUsageFact struct {
	TenantID       string
	TaskID         string
	ModuleCode     string
	Metric         string
	Quantity       int64
	SourceType     string
	SourceID       string
	IdempotencyKey string
	OccurredAt     time.Time
}

type GenerationUsageReservation struct {
	EventID          string
	AlreadyCommitted bool
}

type GenerationUsageSettlement interface {
	ReserveGeneration(context.Context, string, string, time.Time) (GenerationUsageReservation, error)
	CommitGeneration(context.Context, string, string) error
	ReleaseGeneration(context.Context, string, string, string) error
}

// GenerationUsageAdmission controls new ledger reservations. It deliberately
// does not gate commit or release, so a disabled or narrowed rollout can still
// drain settlement work created by an earlier cohort configuration.
type GenerationUsageAdmission interface {
	AllowsGenerationUsage(tenantID string) bool
}

// StudioProductImageUsage meters direct product-image generation invoked by
// Studio batch task creation through the same subscription boundary as the
// product-image APIs.
type StudioProductImageUsage interface {
	AuthorizeProductImageUsage(context.Context, string, int) error
	RecordProductImageUsage(context.Context, string, int) error
}

// StudioProductImageUsageIdempotent is the crash-recoverable legacy counter
// path used when the durable usage ledger is disabled. The operation key is
// tied to the generated task identity so retries cannot double-charge.
type StudioProductImageUsageIdempotent interface {
	RecordProductImageUsageOnce(context.Context, string, int, string) error
}

// StudioProductImageUsageReservation is the durable quota path for batch
// product-image generation. Implementations reserve by a deterministic
// candidate identity before provider work, then commit or release that same
// reservation after the task outcome is known.
type StudioProductImageUsageReservation interface {
	ReserveProductImageUsage(context.Context, string, string, int) error
	CommitProductImageUsage(context.Context, string, string) error
	ReleaseProductImageUsage(context.Context, string, string, string) error
}

// StudioProductImageUsageReservationLifecycle reserves a route already
// selected and persisted by the batch-task lifecycle. Unlike admission, this
// path must not re-evaluate a later rollout change.
type StudioProductImageUsageReservationLifecycle interface {
	ReserveProductImageUsageForLifecycle(context.Context, string, string, int) error
}

// StudioProductImageUsageReservationLookup identifies reservations created by
// older batch links that predate their persisted accounting route.
type StudioProductImageUsageReservationLookup interface {
	HasProductImageUsageReservation(context.Context, string, string) (bool, error)
}

// StudioProductImageUsageReservationAvailability lets an adapter keep the
// legacy authorize/record path when its durable ledger is disabled. The
// reservation methods may still exist for a configured deployment, but they
// must not be selected when the backing ledger is unavailable.
type StudioProductImageUsageReservationAvailability interface {
	StudioProductImageUsageReservationEnabled() bool
}

func generationUsageIdentity(taskID, tenantID string, occurredAt time.Time) generationUsageFact {
	taskID = strings.TrimSpace(taskID)
	tenantID = strings.TrimSpace(tenantID)
	return generationUsageFact{
		TenantID:       tenantID,
		TaskID:         taskID,
		ModuleCode:     generationUsageModuleCode,
		Metric:         generationUsageMetric,
		Quantity:       1,
		SourceType:     generationUsageSourceType,
		SourceID:       taskID,
		IdempotencyKey: "listingkit:generation:" + taskID,
		OccurredAt:     occurredAt,
	}
}
