package inviteflow

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/authidentity"
)

type noticeStore struct{ *testStore }

func (noticeStore) ListNoticeInvitations(context.Context, NoticeQuery) ([]Invitation, string, error) {
	panic("authorization must precede native facts")
}

func TestNoticeManagerPreservesUnavailableDependency(t *testing.T) {
	s, ctx, store, _ := fixture()
	s.Store = noticeStore{store}
	s.Manager = func(context.Context, string) (authidentity.AuthenticatedIdentity, error) {
		return authidentity.AuthenticatedIdentity{}, ErrUnavailable
	}
	_, _, err := s.NotificationFacts(ctx, false, "", 20)
	if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrPermission) {
		t.Fatalf("dependency failure hidden as permission denial: %v", err)
	}
}
