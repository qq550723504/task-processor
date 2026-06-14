package submission

import (
	"context"
	"errors"
	"testing"
)

func TestRemoteRefreshServiceRefreshSuccess(t *testing.T) {
	t.Parallel()

	type state struct{ value string }
	type result struct{ value string }

	var calls []string
	service := NewRemoteRefreshService(RemoteRefreshServiceConfig[state, string, string, result]{
		PersistPhase: func(context.Context, *state) error {
			calls = append(calls, "phase")
			return nil
		},
		BuildRequest: func(_ context.Context, s *state) (string, error) {
			calls = append(calls, "build")
			return s.value + "-request", nil
		},
		Execute: func(_ context.Context, req string) (string, error) {
			calls = append(calls, "execute:"+req)
			return "event", nil
		},
		RecordEvent: func(_ *state, event string) {
			calls = append(calls, "event:"+event)
		},
		FinishError: func(context.Context, *state, error) (*result, error) {
			t.Fatal("FinishError should not be called")
			return nil, nil
		},
		FinishOK: func(_ context.Context, s *state) (*result, error) {
			calls = append(calls, "success")
			return &result{value: s.value + "-ok"}, nil
		},
	})

	got, err := service.Refresh(context.Background(), &state{value: "remote"})
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if got == nil || got.value != "remote-ok" {
		t.Fatalf("result = %+v", got)
	}
	want := []string{"phase", "build", "execute:remote-request", "event:event", "success"}
	if len(calls) != len(want) {
		t.Fatalf("calls = %+v, want %+v", calls, want)
	}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q; full=%+v", i, calls[i], want[i], calls)
		}
	}
}

func TestRemoteRefreshServiceRefreshReturnsOriginalRemoteErrorAfterFinishError(t *testing.T) {
	t.Parallel()

	type state struct{}

	remoteErr := errors.New("remote failed")
	var finished bool
	service := NewRemoteRefreshService(RemoteRefreshServiceConfig[state, string, string, string]{
		BuildRequest: func(context.Context, *state) (string, error) { return "request", nil },
		Execute: func(context.Context, string) (string, error) {
			return "event", remoteErr
		},
		RecordEvent: func(*state, string) {},
		FinishError: func(context.Context, *state, error) (*string, error) {
			finished = true
			result := "fallback"
			return &result, nil
		},
		FinishOK: func(context.Context, *state) (*string, error) {
			t.Fatal("FinishOK should not be called")
			return nil, nil
		},
	})

	got, err := service.Refresh(context.Background(), &state{})
	if !errors.Is(err, remoteErr) {
		t.Fatalf("Refresh() error = %v, want %v", err, remoteErr)
	}
	if got == nil || *got != "fallback" {
		t.Fatalf("result = %+v", got)
	}
	if !finished {
		t.Fatal("expected FinishError to run")
	}
}

func TestRemoteRefreshServiceRefreshReturnsFinishErrorWhenFallbackFails(t *testing.T) {
	t.Parallel()

	type state struct{}

	remoteErr := errors.New("remote failed")
	finishErr := errors.New("persist failed")
	service := NewRemoteRefreshService(RemoteRefreshServiceConfig[state, string, string, string]{
		BuildRequest: func(context.Context, *state) (string, error) { return "request", nil },
		Execute: func(context.Context, string) (string, error) {
			return "", remoteErr
		},
		FinishError: func(context.Context, *state, error) (*string, error) {
			return nil, finishErr
		},
		FinishOK: func(context.Context, *state) (*string, error) {
			return nil, nil
		},
	})

	if _, err := service.Refresh(context.Background(), &state{}); !errors.Is(err, finishErr) {
		t.Fatalf("Refresh() error = %v, want %v", err, finishErr)
	}
}
