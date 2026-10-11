package ecoservices

import (
	"context"
	"errors"
	"testing"
)

type qualificationRepository struct {
	Repository
	writes, replays int
}

func (r *qualificationRepository) Apply(context.Context, Command, int) (Result, error) {
	r.writes++
	return Result{}, nil
}
func (r *qualificationRepository) ReadMutationResult(context.Context, Command) (Result, bool, error) {
	r.replays++
	return Result{}, false, nil
}

func TestQualificationRejectsAllOtherCommandsBeforeOwnerReplayOrWrite(t *testing.T) {
	repo := &qualificationRepository{}
	s, err := NewQualificationService(repo)
	if err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"listing_create", "listing_update", "listing_publish", "request_create", "quote", "confirm_quote", "start", "deliver", "accept", "reject", "cancel", "refund_propose", "refund_confirm", "refund_review", "refund_review_reject", "future_command"} {
		if _, err := s.Mutate(context.Background(), Command{Kind: kind}); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("closed command %s was admitted: %v", kind, err)
		}
	}
	if repo.writes != 0 || repo.replays != 0 {
		t.Fatal("closed command reached owner")
	}
}
