package submission

import (
	"context"
	"errors"
	"testing"
)

func TestRecoveryRouteServiceRecoverLocal(t *testing.T) {
	t.Parallel()

	type state struct{ local bool }
	type result struct{ value string }

	var calls []string
	service := NewRecoveryRouteService(RecoveryRouteServiceConfig[state, result]{
		UseLocal: func(s *state) bool {
			calls = append(calls, "route")
			return s.local
		},
		RecoverLocal: func(context.Context, *state) (*result, error) {
			calls = append(calls, "local")
			return &result{value: "local"}, nil
		},
		RecoverRemote: func(context.Context, *state) (*result, error) {
			calls = append(calls, "remote")
			return &result{value: "remote"}, nil
		},
	})

	got, err := service.Recover(context.Background(), &state{local: true})
	if err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if got == nil || got.value != "local" {
		t.Fatalf("result = %+v", got)
	}
	want := []string{"route", "local"}
	for i := range want {
		if calls[i] != want[i] {
			t.Fatalf("calls[%d] = %q, want %q; full=%+v", i, calls[i], want[i], calls)
		}
	}
}

func TestRecoveryRouteServiceRecoverRemote(t *testing.T) {
	t.Parallel()

	type state struct{ local bool }
	type result struct{ value string }

	service := NewRecoveryRouteService(RecoveryRouteServiceConfig[state, result]{
		UseLocal: func(s *state) bool { return s.local },
		RecoverLocal: func(context.Context, *state) (*result, error) {
			t.Fatal("RecoverLocal should not be called")
			return nil, nil
		},
		RecoverRemote: func(context.Context, *state) (*result, error) {
			return &result{value: "remote"}, nil
		},
	})

	got, err := service.Recover(context.Background(), &state{local: false})
	if err != nil {
		t.Fatalf("Recover() error = %v", err)
	}
	if got == nil || got.value != "remote" {
		t.Fatalf("result = %+v", got)
	}
}

func TestRecoveryRouteServiceRecoverReturnsIncompleteError(t *testing.T) {
	t.Parallel()

	type state struct{}
	service := NewRecoveryRouteService(RecoveryRouteServiceConfig[state, string]{})
	if _, err := service.Recover(context.Background(), &state{}); err == nil {
		t.Fatal("expected incomplete service error")
	}
}

func TestRecoveryRouteServiceRecoverReturnsNilStateError(t *testing.T) {
	t.Parallel()

	type state struct{}
	service := NewRecoveryRouteService(RecoveryRouteServiceConfig[state, string]{
		UseLocal:      func(*state) bool { return true },
		RecoverLocal:  func(context.Context, *state) (*string, error) { value := "local"; return &value, nil },
		RecoverRemote: func(context.Context, *state) (*string, error) { value := "remote"; return &value, nil },
	})
	if _, err := service.Recover(context.Background(), nil); err == nil {
		t.Fatal("expected nil state error")
	}
}

func TestRecoveryRouteServiceRecoverPropagatesHandlerError(t *testing.T) {
	t.Parallel()

	type state struct{ local bool }

	expectedErr := errors.New("remote failed")
	service := NewRecoveryRouteService(RecoveryRouteServiceConfig[state, string]{
		UseLocal: func(*state) bool { return false },
		RecoverLocal: func(context.Context, *state) (*string, error) {
			value := "local"
			return &value, nil
		},
		RecoverRemote: func(context.Context, *state) (*string, error) {
			return nil, expectedErr
		},
	})

	if _, err := service.Recover(context.Background(), &state{}); !errors.Is(err, expectedErr) {
		t.Fatalf("Recover() error = %v, want %v", err, expectedErr)
	}
}
