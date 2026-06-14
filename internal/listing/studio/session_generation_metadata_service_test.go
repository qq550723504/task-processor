package studio

import (
	"context"
	"errors"
	"testing"
)

type metadataSessionStub struct {
	ID              string
	Status          string
	GenerationJobID string
	GenerationJobs  []string
	GenerationError string
}

type metadataSessionRepoStub struct {
	session *metadataSessionStub
}

func (r *metadataSessionRepoStub) GetSession(context.Context, string) (*metadataSessionStub, error) {
	if r.session == nil {
		return nil, nil
	}
	cloned := *r.session
	cloned.GenerationJobs = append([]string(nil), r.session.GenerationJobs...)
	return &cloned, nil
}

func (r *metadataSessionRepoStub) UpdateSession(_ context.Context, session *metadataSessionStub) error {
	if session == nil {
		return nil
	}
	cloned := *session
	cloned.GenerationJobs = append([]string(nil), session.GenerationJobs...)
	r.session = &cloned
	return nil
}

func TestSessionGenerationMetadataServicePatch(t *testing.T) {
	repo := &metadataSessionRepoStub{session: &metadataSessionStub{ID: "session-1"}}
	service := NewSessionGenerationMetadataService(SessionGenerationMetadataServiceConfig[metadataSessionStub, string, string]{
		Repo: repo,
		SetStatus: func(session *metadataSessionStub, status string) {
			session.Status = status
		},
		SetGenerationJobID: func(session *metadataSessionStub, jobID string) {
			session.GenerationJobID = jobID
		},
		SetGenerationJobs: func(session *metadataSessionStub, jobs []string) {
			session.GenerationJobs = append([]string(nil), jobs...)
		},
		SetGenerationError: func(session *metadataSessionStub, errMessage string) {
			session.GenerationError = errMessage
		},
	})

	status := "generated"
	jobID := "job-1"
	errMessage := "done"
	session, err := service.Patch(context.Background(), SessionGenerationMetadataPatchRequest[string, string]{
		SessionID:       "session-1",
		Status:          &status,
		GenerationJobID: &jobID,
		GenerationJobs:  []string{"job-1", "job-2"},
		GenerationError: &errMessage,
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if session.Status != "generated" || session.GenerationJobID != "job-1" || len(session.GenerationJobs) != 2 || session.GenerationError != "done" {
		t.Fatalf("session = %+v, want patched metadata", session)
	}
}

func TestSessionGenerationMetadataServicePatchReturnsNotFound(t *testing.T) {
	service := NewSessionGenerationMetadataService(SessionGenerationMetadataServiceConfig[metadataSessionStub, string, string]{
		Repo: &metadataSessionRepoStub{},
	})

	_, err := service.Patch(context.Background(), SessionGenerationMetadataPatchRequest[string, string]{SessionID: "missing"})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Patch() error = %v, want ErrSessionNotFound", err)
	}
}
