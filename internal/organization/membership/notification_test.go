package membership

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/authz"
)

type noticeOperationStore struct{ *memoryReceipts }

func (noticeOperationStore) ListNoticeOperations(context.Context, OperationScope, string, int) ([]Operation, string, error) {
	return []Operation{}, "", nil
}

func TestNoticeFinalAuthorizationRetainsDependencyFailure(t *testing.T) {
	authorizer, err := authz.NewListingKitAuthorizer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	service := testService(&directoryStub{}, authorizer, "project")
	calls := 0
	commands := NewCommands(service, noticeOperationStore{&memoryReceipts{}}, &writerStub{}, func(ctx context.Context) (context.Context, error) {
		calls++
		if calls == 2 {
			return nil, ErrUnavailable
		}
		return ctx, nil
	})
	_, _, err = commands.NotificationFacts(scopedContext("listingkit_admin"), "", 20)
	if calls != 2 || !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrPermission) {
		t.Fatalf("late authorization failure hidden as denial: calls=%d err=%v", calls, err)
	}
}
