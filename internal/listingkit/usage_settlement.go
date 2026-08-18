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
