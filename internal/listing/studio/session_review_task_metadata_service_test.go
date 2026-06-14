package studio

import (
	"context"
	"errors"
	"testing"
)

type reviewTaskSessionStub struct {
	ID                string
	ApprovedDesignIDs []string
	CreatedTasks      []string
}

type reviewTaskSessionRepoStub struct {
	session *reviewTaskSessionStub
}

func (r *reviewTaskSessionRepoStub) GetSession(context.Context, string) (*reviewTaskSessionStub, error) {
	if r.session == nil {
		return nil, nil
	}
	cloned := *r.session
	cloned.ApprovedDesignIDs = append([]string(nil), r.session.ApprovedDesignIDs...)
	cloned.CreatedTasks = append([]string(nil), r.session.CreatedTasks...)
	return &cloned, nil
}

func (r *reviewTaskSessionRepoStub) UpdateSession(_ context.Context, session *reviewTaskSessionStub) error {
	if session == nil {
		return nil
	}
	cloned := *session
	cloned.ApprovedDesignIDs = append([]string(nil), session.ApprovedDesignIDs...)
	cloned.CreatedTasks = append([]string(nil), session.CreatedTasks...)
	r.session = &cloned
	return nil
}

func TestSessionReviewTaskMetadataServicePatch(t *testing.T) {
	repo := &reviewTaskSessionRepoStub{session: &reviewTaskSessionStub{ID: "session-1"}}
	service := NewSessionReviewTaskMetadataService(SessionReviewTaskMetadataServiceConfig[reviewTaskSessionStub, string]{
		Repo: repo,
		SetApprovedDesignIDs: func(session *reviewTaskSessionStub, ids []string) {
			session.ApprovedDesignIDs = append([]string(nil), ids...)
		},
		SetCreatedTasks: func(session *reviewTaskSessionStub, tasks []string) {
			session.CreatedTasks = append([]string(nil), tasks...)
		},
	})

	session, err := service.Patch(context.Background(), SessionReviewTaskMetadataPatchRequest[string]{
		SessionID:         "session-1",
		ApprovedDesignIDs: []string{"design-1"},
		CreatedTasks:      []string{"task-1"},
	})
	if err != nil {
		t.Fatalf("Patch() error = %v", err)
	}
	if len(session.ApprovedDesignIDs) != 1 || session.ApprovedDesignIDs[0] != "design-1" {
		t.Fatalf("session.ApprovedDesignIDs = %+v, want design-1", session.ApprovedDesignIDs)
	}
	if len(session.CreatedTasks) != 1 || session.CreatedTasks[0] != "task-1" {
		t.Fatalf("session.CreatedTasks = %+v, want task-1", session.CreatedTasks)
	}
}

func TestSessionReviewTaskMetadataServicePatchReturnsNotFound(t *testing.T) {
	service := NewSessionReviewTaskMetadataService(SessionReviewTaskMetadataServiceConfig[reviewTaskSessionStub, string]{
		Repo: &reviewTaskSessionRepoStub{},
	})

	_, err := service.Patch(context.Background(), SessionReviewTaskMetadataPatchRequest[string]{SessionID: "missing"})
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("Patch() error = %v, want ErrSessionNotFound", err)
	}
}
