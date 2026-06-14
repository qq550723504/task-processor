package studio

import (
	"context"
	"fmt"
	"strings"
)

type SessionAsyncJobSyncRequest struct {
	SessionID    string
	JobStatus    string
	JobID        string
	ErrorMessage string
}

type SessionMutationRepository[Session any] interface {
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	UpdateSession(ctx context.Context, session *Session) error
}

type SessionAsyncJobSyncService[Session any, SessionStatus any] struct {
	repo               SessionMutationRepository[Session]
	statusForJob       func(string) SessionStatus
	setStatus          func(*Session, SessionStatus)
	setGenerationJob   func(*Session, string)
	setGenerationError func(*Session, string)
}

type SessionAsyncJobSyncServiceConfig[Session any, SessionStatus any] struct {
	Repo               SessionMutationRepository[Session]
	StatusForJob       func(string) SessionStatus
	SetStatus          func(*Session, SessionStatus)
	SetGenerationJob   func(*Session, string)
	SetGenerationError func(*Session, string)
}

func NewSessionAsyncJobSyncService[Session any, SessionStatus any](config SessionAsyncJobSyncServiceConfig[Session, SessionStatus]) *SessionAsyncJobSyncService[Session, SessionStatus] {
	return &SessionAsyncJobSyncService[Session, SessionStatus]{
		repo:               config.Repo,
		statusForJob:       config.StatusForJob,
		setStatus:          config.SetStatus,
		setGenerationJob:   config.SetGenerationJob,
		setGenerationError: config.SetGenerationError,
	}
}

func (s *SessionAsyncJobSyncService[Session, SessionStatus]) SyncAsyncJob(ctx context.Context, req SessionAsyncJobSyncRequest) error {
	if strings.TrimSpace(req.SessionID) == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, req.SessionID)
	if err != nil {
		return err
	}
	if session == nil {
		return ErrSessionNotFound
	}
	if s.statusForJob != nil && s.setStatus != nil {
		s.setStatus(session, s.statusForJob(req.JobStatus))
	}
	if s.setGenerationJob != nil {
		s.setGenerationJob(session, strings.TrimSpace(req.JobID))
	}
	if s.setGenerationError != nil {
		s.setGenerationError(session, strings.TrimSpace(req.ErrorMessage))
	}
	return s.repo.UpdateSession(ctx, session)
}
