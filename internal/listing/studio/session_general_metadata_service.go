package studio

import (
	"context"
	"fmt"
)

type SessionGeneralMetadataPatchRequest[Patch any] struct {
	SessionID string
	Patch     *Patch
}

type SessionGeneralMetadataService[Session any, Patch any] struct {
	repo       SessionMutationRepository[Session]
	applyPatch func(*Session, *Patch)
}

type SessionGeneralMetadataServiceConfig[Session any, Patch any] struct {
	Repo       SessionMutationRepository[Session]
	ApplyPatch func(*Session, *Patch)
}

func NewSessionGeneralMetadataService[Session any, Patch any](config SessionGeneralMetadataServiceConfig[Session, Patch]) *SessionGeneralMetadataService[Session, Patch] {
	return &SessionGeneralMetadataService[Session, Patch]{
		repo:       config.Repo,
		applyPatch: config.ApplyPatch,
	}
}

func (s *SessionGeneralMetadataService[Session, Patch]) Patch(ctx context.Context, req SessionGeneralMetadataPatchRequest[Patch]) (*Session, error) {
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
	if s.applyPatch != nil {
		s.applyPatch(session, req.Patch)
	}
	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}
