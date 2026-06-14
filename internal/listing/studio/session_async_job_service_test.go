package studio

import (
	"context"
	"errors"
	"testing"
)

type asyncSessionStub struct {
	ID              string
	Status          string
	GenerationJobID string
	GenerationError string
}

type asyncSessionRepoStub struct {
	session *asyncSessionStub
}

func (r *asyncSessionRepoStub) GetSession(context.Context, string) (*asyncSessionStub, error) {
	if r.session == nil {
		return nil, nil
	}
	cloned := *r.session
	return &cloned, nil
}

func (r *asyncSessionRepoStub) UpdateSession(_ context.Context, session *asyncSessionStub) error {
	if session == nil {
		return nil
	}
	cloned := *session
	r.session = &cloned
	return nil
}

func TestSessionAsyncJobSyncServiceSyncsJobState(t *testing.T) {
	repo := &asyncSessionRepoStub{session: &asyncSessionStub{ID: "session-1"}}
	service := NewSessionAsyncJobSyncService(SessionAsyncJobSyncServiceConfig[asyncSessionStub, string]{
		Repo:         repo,
		StatusForJob: func(jobStatus string) string { return "mapped:" + jobStatus },
		SetStatus: func(session *asyncSessionStub, status string) {
			session.Status = status
		},
		SetGenerationJob: func(session *asyncSessionStub, jobID string) {
			session.GenerationJobID = jobID
		},
		SetGenerationError: func(session *asyncSessionStub, errMessage string) {
			session.GenerationError = errMessage
		},
	})

	err := service.SyncAsyncJob(context.Background(), SessionAsyncJobSyncRequest{
		SessionID:    "session-1",
		JobStatus:    "running",
		JobID:        " job-1 ",
		ErrorMessage: " err ",
	})
	if err != nil {
		t.Fatalf("SyncAsyncJob() error = %v", err)
	}
	if repo.session.Status != "mapped:running" || repo.session.GenerationJobID != "job-1" || repo.session.GenerationError != "err" {
		t.Fatalf("repo.session = %+v, want mapped async job fields", repo.session)
	}
}

func TestSessionAsyncJobSyncServiceReturnsNotFound(t *testing.T) {
	service := NewSessionAsyncJobSyncService(SessionAsyncJobSyncServiceConfig[asyncSessionStub, string]{
		Repo: &asyncSessionRepoStub{},
	})

	err := service.SyncAsyncJob(context.Background(), SessionAsyncJobSyncRequest{SessionID: "missing"})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("SyncAsyncJob() error = %v, want ErrSessionNotFound", err)
	}
}

func TestSessionAsyncJobSyncServiceIgnoresBlankSessionID(t *testing.T) {
	service := NewSessionAsyncJobSyncService(SessionAsyncJobSyncServiceConfig[asyncSessionStub, string]{
		Repo: &asyncSessionRepoStub{},
	})

	if err := service.SyncAsyncJob(context.Background(), SessionAsyncJobSyncRequest{}); err != nil {
		t.Fatalf("SyncAsyncJob() error = %v, want nil", err)
	}
}
