package api

import (
	"context"
	"encoding/json"
	"time"

	"task-processor/internal/listingkit"
)

type startStudioAsyncJobRequest struct {
	Path      string          `json:"path"`
	Body      json.RawMessage `json:"body"`
	SessionID string          `json:"session_id,omitempty"`
}

type studioAsyncJob struct {
	ID             string                          `json:"job_id"`
	Path           string                          `json:"path"`
	Status         listingkit.StudioAsyncJobStatus `json:"status"`
	Result         any                             `json:"result,omitempty"`
	Error          string                          `json:"error,omitempty"`
	UpstreamStatus int                             `json:"upstream_status,omitempty"`
	CreatedAt      time.Time                       `json:"created_at"`
	UpdatedAt      time.Time                       `json:"updated_at"`
	FinishedAt     *time.Time                      `json:"finished_at,omitempty"`
}

type studioAsyncJobStore struct {
	repo listingkit.StudioAsyncJobRepository
}

type studioAsyncJobStoreService interface {
	create(ctx context.Context, path string) (studioAsyncJob, error)
	get(ctx context.Context, id string) (studioAsyncJob, bool)
	succeed(ctx context.Context, id string, result any)
	fail(ctx context.Context, id string, err error, status int)
}
