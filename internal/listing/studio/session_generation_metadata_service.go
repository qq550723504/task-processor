package studio

import (
	"context"
	"fmt"
)

type SessionGenerationMetadataPatchRequest[Status any, Job any] struct {
	SessionID       string
	Status          *Status
	GenerationJobID *string
	GenerationJobs  []Job
	GenerationError *string
}

type SessionGenerationMetadataService[Session any, Status any, Job any] struct {
	repo               SessionMutationRepository[Session]
	setStatus          func(*Session, Status)
	setGenerationJobID func(*Session, string)
	setGenerationJobs  func(*Session, []Job)
	setGenerationError func(*Session, string)
}

type SessionGenerationMetadataServiceConfig[Session any, Status any, Job any] struct {
	Repo               SessionMutationRepository[Session]
	SetStatus          func(*Session, Status)
	SetGenerationJobID func(*Session, string)
	SetGenerationJobs  func(*Session, []Job)
	SetGenerationError func(*Session, string)
}

func NewSessionGenerationMetadataService[Session any, Status any, Job any](config SessionGenerationMetadataServiceConfig[Session, Status, Job]) *SessionGenerationMetadataService[Session, Status, Job] {
	return &SessionGenerationMetadataService[Session, Status, Job]{
		repo:               config.Repo,
		setStatus:          config.SetStatus,
		setGenerationJobID: config.SetGenerationJobID,
		setGenerationJobs:  config.SetGenerationJobs,
		setGenerationError: config.SetGenerationError,
	}
}

func (s *SessionGenerationMetadataService[Session, Status, Job]) Patch(ctx context.Context, req SessionGenerationMetadataPatchRequest[Status, Job]) (*Session, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}
	if req.Status != nil && s.setStatus != nil {
		s.setStatus(session, *req.Status)
	}
	if req.GenerationJobID != nil && s.setGenerationJobID != nil {
		s.setGenerationJobID(session, *req.GenerationJobID)
	}
	if req.GenerationJobs != nil && s.setGenerationJobs != nil {
		s.setGenerationJobs(session, req.GenerationJobs)
	}
	if req.GenerationError != nil && s.setGenerationError != nil {
		s.setGenerationError(session, *req.GenerationError)
	}
	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}
