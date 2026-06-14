package studio

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type sessionSelectionStub struct {
	VariantID int64
}

type sessionRecordStub struct {
	ID           string
	UserID       string
	SelectionKey string
	Selection    sessionSelectionStub
}

type sessionRepoStub struct {
	findByKey map[string]*sessionRecordStub
	byID      map[string]*sessionRecordStub
	designs   map[string][]string
}

func (r *sessionRepoStub) FindLatestSessionBySelectionKey(_ context.Context, selectionKey string) (*sessionRecordStub, error) {
	session := r.findByKey[selectionKey]
	if session == nil {
		return nil, nil
	}
	cloned := *session
	return &cloned, nil
}

func (r *sessionRepoStub) CreateSession(_ context.Context, session *sessionRecordStub) error {
	cloned := *session
	if r.byID == nil {
		r.byID = map[string]*sessionRecordStub{}
	}
	if r.findByKey == nil {
		r.findByKey = map[string]*sessionRecordStub{}
	}
	r.byID[session.ID] = &cloned
	r.findByKey[session.SelectionKey] = &cloned
	return nil
}

func (r *sessionRepoStub) GetSession(_ context.Context, sessionID string) (*sessionRecordStub, error) {
	session := r.byID[sessionID]
	if session == nil {
		return nil, nil
	}
	cloned := *session
	return &cloned, nil
}

func (r *sessionRepoStub) ListSessionDesigns(_ context.Context, sessionID string) ([]string, error) {
	return append([]string(nil), r.designs[sessionID]...), nil
}

func newSessionServiceForTest(repo *sessionRepoStub) *SessionService[sessionRecordStub, sessionSelectionStub, string] {
	return NewSessionService(SessionServiceConfig[sessionRecordStub, sessionSelectionStub, string]{
		Repo: repo,
		ValidateSelection: func(selection *sessionSelectionStub) error {
			if selection == nil || selection.VariantID <= 0 {
				return fmt.Errorf("selection is required")
			}
			return nil
		},
		BuildSelectionKey: func(selection *sessionSelectionStub) string {
			return fmt.Sprintf("variant:%d", selection.VariantID)
		},
		NewSession: func(id string, userID string, selectionKey string, selection *sessionSelectionStub) *sessionRecordStub {
			return &sessionRecordStub{
				ID:           id,
				UserID:       userID,
				SelectionKey: selectionKey,
				Selection:    *selection,
			}
		},
		SessionID:     func(session *sessionRecordStub) string { return session.ID },
		RequestUserID: func(context.Context) string { return "ctx-user" },
		NewSessionID:  func() string { return "session-1" },
	})
}

func TestSessionServiceEnsureCreatesNewSession(t *testing.T) {
	service := newSessionServiceForTest(&sessionRepoStub{})

	result, err := service.EnsureSession(context.Background(), &EnsureSessionRequest[sessionSelectionStub]{
		Selection: &sessionSelectionStub{VariantID: 101},
	})
	if err != nil {
		t.Fatalf("EnsureSession() error = %v", err)
	}
	if result.Session == nil || result.Session.ID != "session-1" || result.Session.UserID != "ctx-user" {
		t.Fatalf("result.Session = %+v, want created session", result.Session)
	}
	if len(result.Designs) != 0 {
		t.Fatalf("result.Designs = %+v, want empty designs", result.Designs)
	}
}

func TestSessionServiceEnsureLoadsExistingSessionDetail(t *testing.T) {
	service := newSessionServiceForTest(&sessionRepoStub{
		findByKey: map[string]*sessionRecordStub{
			"variant:101": {ID: "session-1", SelectionKey: "variant:101", Selection: sessionSelectionStub{VariantID: 101}},
		},
		designs: map[string][]string{
			"session-1": {"design-1"},
		},
	})

	result, err := service.EnsureSession(context.Background(), &EnsureSessionRequest[sessionSelectionStub]{
		Selection: &sessionSelectionStub{VariantID: 101},
	})
	if err != nil {
		t.Fatalf("EnsureSession() error = %v", err)
	}
	if result.Session == nil || result.Session.ID != "session-1" || len(result.Designs) != 1 {
		t.Fatalf("result = %+v, want existing session detail", result)
	}
}

func TestSessionServiceGetReturnsNotFound(t *testing.T) {
	service := newSessionServiceForTest(&sessionRepoStub{})

	_, err := service.GetSession(context.Background(), "missing")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("GetSession() error = %v, want ErrSessionNotFound", err)
	}
}
