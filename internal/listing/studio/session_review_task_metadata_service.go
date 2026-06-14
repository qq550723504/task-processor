package studio

import (
	"context"
	"fmt"
)

type SessionReviewTaskMetadataPatchRequest[Task any] struct {
	SessionID         string
	ApprovedDesignIDs []string
	CreatedTasks      []Task
}

type SessionReviewTaskMetadataService[Session any, Task any] struct {
	repo                 SessionMutationRepository[Session]
	setApprovedDesignIDs func(*Session, []string)
	setCreatedTasks      func(*Session, []Task)
}

type SessionReviewTaskMetadataServiceConfig[Session any, Task any] struct {
	Repo                 SessionMutationRepository[Session]
	SetApprovedDesignIDs func(*Session, []string)
	SetCreatedTasks      func(*Session, []Task)
}

func NewSessionReviewTaskMetadataService[Session any, Task any](config SessionReviewTaskMetadataServiceConfig[Session, Task]) *SessionReviewTaskMetadataService[Session, Task] {
	return &SessionReviewTaskMetadataService[Session, Task]{
		repo:                 config.Repo,
		setApprovedDesignIDs: config.SetApprovedDesignIDs,
		setCreatedTasks:      config.SetCreatedTasks,
	}
}

func (s *SessionReviewTaskMetadataService[Session, Task]) Patch(ctx context.Context, req SessionReviewTaskMetadataPatchRequest[Task]) (*Session, error) {
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
	if req.ApprovedDesignIDs != nil && s.setApprovedDesignIDs != nil {
		s.setApprovedDesignIDs(session, req.ApprovedDesignIDs)
	}
	if req.CreatedTasks != nil && s.setCreatedTasks != nil {
		s.setCreatedTasks(session, req.CreatedTasks)
	}
	if err := s.repo.UpdateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}
