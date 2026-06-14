package studio

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrSessionNotFound = errors.New("studio session not found")

type SessionDetail[Session any, Design any] struct {
	Session *Session
	Designs []Design
}

type EnsureSessionRequest[Selection any] struct {
	UserID    string
	Selection *Selection
}

type SessionRepository[Session any, Selection any, Design any] interface {
	FindLatestSessionBySelectionKey(ctx context.Context, selectionKey string) (*Session, error)
	CreateSession(ctx context.Context, session *Session) error
	GetSession(ctx context.Context, sessionID string) (*Session, error)
	ListSessionDesigns(ctx context.Context, sessionID string) ([]Design, error)
}

type SessionService[Session any, Selection any, Design any] struct {
	repo              SessionRepository[Session, Selection, Design]
	validateSelection func(*Selection) error
	buildSelectionKey func(*Selection) string
	newSession        func(id string, userID string, selectionKey string, selection *Selection) *Session
	sessionID         func(*Session) string
	requestUserID     func(context.Context) string
	newSessionID      func() string
}

type SessionServiceConfig[Session any, Selection any, Design any] struct {
	Repo              SessionRepository[Session, Selection, Design]
	ValidateSelection func(*Selection) error
	BuildSelectionKey func(*Selection) string
	NewSession        func(id string, userID string, selectionKey string, selection *Selection) *Session
	SessionID         func(*Session) string
	RequestUserID     func(context.Context) string
	NewSessionID      func() string
}

func NewSessionService[Session any, Selection any, Design any](config SessionServiceConfig[Session, Selection, Design]) *SessionService[Session, Selection, Design] {
	return &SessionService[Session, Selection, Design]{
		repo:              config.Repo,
		validateSelection: config.ValidateSelection,
		buildSelectionKey: config.BuildSelectionKey,
		newSession:        config.NewSession,
		sessionID:         config.SessionID,
		requestUserID:     config.RequestUserID,
		newSessionID:      config.NewSessionID,
	}
}

func (s *SessionService[Session, Selection, Design]) EnsureSession(ctx context.Context, req *EnsureSessionRequest[Selection]) (*SessionDetail[Session, Design], error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	if req == nil || req.Selection == nil {
		return nil, fmt.Errorf("selection is required")
	}
	if s.validateSelection != nil {
		if err := s.validateSelection(req.Selection); err != nil {
			return nil, err
		}
	}

	userID := strings.TrimSpace(req.UserID)
	if userID == "" && s.requestUserID != nil {
		userID = s.requestUserID(ctx)
	}

	selectionKey := ""
	if s.buildSelectionKey != nil {
		selectionKey = s.buildSelectionKey(req.Selection)
	}
	existing, err := s.repo.FindLatestSessionBySelectionKey(ctx, selectionKey)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return s.loadSessionDetail(ctx, existing)
	}
	if s.newSession == nil || s.newSessionID == nil {
		return nil, fmt.Errorf("studio session creation is not configured")
	}
	session := s.newSession(s.newSessionID(), userID, selectionKey, req.Selection)
	if err := s.repo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return &SessionDetail[Session, Design]{
		Session: session,
		Designs: []Design{},
	}, nil
}

func (s *SessionService[Session, Selection, Design]) GetSession(ctx context.Context, sessionID string) (*SessionDetail[Session, Design], error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("studio session repository is not configured")
	}
	session, err := s.repo.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if session == nil {
		return nil, ErrSessionNotFound
	}
	return s.loadSessionDetail(ctx, session)
}

func (s *SessionService[Session, Selection, Design]) loadSessionDetail(ctx context.Context, session *Session) (*SessionDetail[Session, Design], error) {
	if s.sessionID == nil {
		return nil, fmt.Errorf("studio session id mapping is not configured")
	}
	designs, err := s.repo.ListSessionDesigns(ctx, s.sessionID(session))
	if err != nil {
		return nil, err
	}
	return &SessionDetail[Session, Design]{
		Session: session,
		Designs: designs,
	}, nil
}
