package studio

import (
	"context"
	"errors"
	"testing"
)

type generalSessionRecordStub struct {
	ID     string
	Status string
	Prompt string
}

type generalSessionPatchStub struct {
	Status *string
	Prompt *string
}

type generalSessionRepoStub struct {
	byID        map[string]*generalSessionRecordStub
	updated     *generalSessionRecordStub
	updateErr   error
	getCalls    int
	updateCalls int
}

func (r *generalSessionRepoStub) GetSession(_ context.Context, sessionID string) (*generalSessionRecordStub, error) {
	r.getCalls++
	session := r.byID[sessionID]
	if session == nil {
		return nil, nil
	}
	cloned := *session
	return &cloned, nil
}

func (r *generalSessionRepoStub) UpdateSession(_ context.Context, session *generalSessionRecordStub) error {
	r.updateCalls++
	if r.updateErr != nil {
		return r.updateErr
	}
	cloned := *session
	r.updated = &cloned
	return nil
}

func TestSessionGeneralMetadataServicePatchAppliesAndPersists(t *testing.T) {
	t.Parallel()

	status := "ready"
	prompt := "new prompt"
	repo := &generalSessionRepoStub{
		byID: map[string]*generalSessionRecordStub{
			"session-1": {ID: "session-1", Status: "draft", Prompt: "old"},
		},
	}
	service := NewSessionGeneralMetadataService(SessionGeneralMetadataServiceConfig[generalSessionRecordStub, generalSessionPatchStub]{
		Repo: repo,
		ApplyPatch: func(session *generalSessionRecordStub, patch *generalSessionPatchStub) {
			if patch.Status != nil {
				session.Status = *patch.Status
			}
			if patch.Prompt != nil {
				session.Prompt = *patch.Prompt
			}
		},
	})

	session, err := service.Patch(context.Background(), SessionGeneralMetadataPatchRequest[generalSessionPatchStub]{
		SessionID: "session-1",
		Patch:     &generalSessionPatchStub{Status: &status, Prompt: &prompt},
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if session == nil || session.Status != status || session.Prompt != prompt {
		t.Fatalf("Patch() session = %+v, want patched values", session)
	}
	if repo.updated == nil || repo.updated.Status != status || repo.updateCalls != 1 {
		t.Fatalf("updated = %+v updateCalls=%d, want persisted patched session", repo.updated, repo.updateCalls)
	}
}

func TestSessionGeneralMetadataServicePatchReturnsNotFound(t *testing.T) {
	t.Parallel()

	service := NewSessionGeneralMetadataService(SessionGeneralMetadataServiceConfig[generalSessionRecordStub, generalSessionPatchStub]{
		Repo: &generalSessionRepoStub{},
	})

	_, err := service.Patch(context.Background(), SessionGeneralMetadataPatchRequest[generalSessionPatchStub]{SessionID: "missing"})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Patch() error = %v, want ErrSessionNotFound", err)
	}
}
