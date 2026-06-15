package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"task-processor/internal/listingkit"
)

func newStudioAsyncJobStore(repo listingkit.StudioAsyncJobRepository) (*studioAsyncJobStore, error) {
	if repo == nil {
		repo = listingkit.NewMemStudioAsyncJobRepository()
	}
	return &studioAsyncJobStore{repo: repo}, nil
}

func (s *studioAsyncJobStore) create(ctx context.Context, path string) (studioAsyncJob, error) {
	now := time.Now().UTC()
	record := &listingkit.StudioAsyncJobRecord{
		ID:        newStudioAsyncJobID(),
		Path:      path,
		Status:    listingkit.StudioAsyncJobStatusRunning,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.repo.CreateStudioAsyncJob(ctx, record); err != nil {
		return studioAsyncJob{}, err
	}
	return mapStudioAsyncJobRecord(record)
}

func (s *studioAsyncJobStore) get(ctx context.Context, id string) (studioAsyncJob, bool) {
	record, err := s.repo.GetStudioAsyncJob(ctx, id)
	if err != nil || record == nil {
		return studioAsyncJob{}, false
	}
	job, err := mapStudioAsyncJobRecord(record)
	if err != nil {
		return studioAsyncJob{}, false
	}
	return job, true
}

func (s *studioAsyncJobStore) succeed(ctx context.Context, id string, result any) {
	_ = s.update(ctx, id, listingkit.StudioAsyncJobStatusSucceeded, result, "", http.StatusOK)
}

func (s *studioAsyncJobStore) fail(ctx context.Context, id string, err error, status int) {
	message := "async job failed"
	if err != nil {
		message = err.Error()
	}
	_ = s.update(ctx, id, listingkit.StudioAsyncJobStatusFailed, nil, message, status)
}

func (s *studioAsyncJobStore) update(ctx context.Context, id string, status listingkit.StudioAsyncJobStatus, result any, message string, upstreamStatus int) error {
	record, err := s.repo.GetStudioAsyncJob(ctx, id)
	if err != nil || record == nil {
		return err
	}
	now := time.Now().UTC()
	record.Status = status
	record.Error = message
	record.UpstreamStatus = upstreamStatus
	record.UpdatedAt = now
	record.FinishedAt = &now
	if err := record.EncodeResult(result); err != nil {
		return err
	}
	return s.repo.UpdateStudioAsyncJob(ctx, record)
}

func mapStudioAsyncJobRecord(record *listingkit.StudioAsyncJobRecord) (studioAsyncJob, error) {
	if record == nil {
		return studioAsyncJob{}, nil
	}
	result, err := record.DecodeResult()
	if err != nil {
		return studioAsyncJob{}, err
	}
	return studioAsyncJob{
		ID:             record.ID,
		Path:           record.Path,
		Status:         record.Status,
		Result:         result,
		Error:          record.Error,
		UpstreamStatus: record.UpstreamStatus,
		CreatedAt:      record.CreatedAt,
		UpdatedAt:      record.UpdatedAt,
		FinishedAt:     record.FinishedAt,
	}, nil
}

func newStudioAsyncJobID() string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		return strings.ReplaceAll(time.Now().UTC().Format(time.RFC3339Nano), ":", "")
	}
	return hex.EncodeToString(data[:])
}
