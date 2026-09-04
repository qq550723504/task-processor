package storecenter

import (
	"errors"
	"testing"
	"time"
)

func TestActivateStoreServiceRequiresFreshConnectionAndStartsThirtyDays(t *testing.T) {
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	state := StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusPendingActivation}

	got, err := ActivateStoreService(state, ConnectionStatusConnected, now)
	if err != nil {
		t.Fatalf("ActivateStoreService() error = %v", err)
	}
	if got.RecordStatus != RecordStatusActive || got.ServiceStatus != ServiceStatusActive {
		t.Fatalf("activated state = %+v", got)
	}
	if got.StartedAt == nil || !got.StartedAt.Equal(now) || got.ExpiresAt == nil || !got.ExpiresAt.Equal(now.Add(30*24*time.Hour)) {
		t.Fatalf("activated period = %+v, want [%s, %s)", got, now, now.Add(30*24*time.Hour))
	}
}

func TestActivateStoreServiceRejectsInvalidPreconditionsWithoutChangingState(t *testing.T) {
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		name       string
		state      StoreServiceState
		connection ConnectionStatus
		want       error
	}{
		{name: "disconnected", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusPendingActivation}, connection: ConnectionStatusDisconnected, want: ErrConnectionNotFresh},
		{name: "already active", state: activeServiceState(now), connection: ConnectionStatusConnected, want: ErrServiceAlreadyActive},
		{name: "suspended", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusSuspended}, connection: ConnectionStatusConnected, want: ErrServiceSuspended},
		{name: "record provisioning", state: StoreServiceState{RecordStatus: RecordStatusProvisioning}, connection: ConnectionStatusConnected, want: ErrInvalidServiceTransition},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := tc.state
			if _, err := ActivateStoreService(tc.state, tc.connection, now); !errors.Is(err, tc.want) {
				t.Fatalf("ActivateStoreService() error = %v, want %v", err, tc.want)
			}
			if !sameServiceState(tc.state, before) {
				t.Fatalf("input state changed from %+v to %+v", before, tc.state)
			}
		})
	}
}

func TestRenewStoreServiceExtendsFromCurrentExpiryAndHonorsMaximum(t *testing.T) {
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	state := activeServiceState(now)
	got, err := RenewStoreService(state, 2, 12, now)
	if err != nil {
		t.Fatalf("RenewStoreService() error = %v", err)
	}
	wantExpiry := now.Add(30 * 24 * time.Hour).Add(2 * 30 * 24 * time.Hour)
	if got.ServiceStatus != ServiceStatusActive || got.ExpiresAt == nil || !got.ExpiresAt.Equal(wantExpiry) {
		t.Fatalf("renewed state = %+v, want expiry %s", got, wantExpiry)
	}
	if _, err := RenewStoreService(state, 13, 12, now); !errors.Is(err, ErrServiceQuantityExceeded) {
		t.Fatalf("over-limit renew error = %v, want %v", err, ErrServiceQuantityExceeded)
	}
	if _, err := RenewStoreService(state, 1, 12, now.Add(31*24*time.Hour)); !errors.Is(err, ErrServiceExpired) {
		t.Fatalf("expired renew error = %v, want %v", err, ErrServiceExpired)
	}
}

func TestReactivateStoreServiceStartsNewPeriodOnlyFromExpired(t *testing.T) {
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	state := StoreServiceState{
		RecordStatus:  RecordStatusActive,
		ServiceStatus: ServiceStatusExpired,
		StartedAt:     timePointer(now.Add(-60 * 24 * time.Hour)),
		ExpiresAt:     timePointer(now.Add(-30 * 24 * time.Hour)),
	}
	got, err := ReactivateStoreService(state, 2, 12, now)
	if err != nil {
		t.Fatalf("ReactivateStoreService() error = %v", err)
	}
	if got.ServiceStatus != ServiceStatusActive || got.StartedAt == nil || !got.StartedAt.Equal(now) || got.ExpiresAt == nil || !got.ExpiresAt.Equal(now.Add(60*24*time.Hour)) {
		t.Fatalf("reactivated state = %+v", got)
	}
	if _, err := ReactivateStoreService(activeServiceState(now), 1, 12, now); !errors.Is(err, ErrServiceNotExpired) {
		t.Fatalf("active reactivation error = %v, want %v", err, ErrServiceNotExpired)
	}
}

func activeServiceState(now time.Time) StoreServiceState {
	return StoreServiceState{
		RecordStatus:  RecordStatusActive,
		ServiceStatus: ServiceStatusActive,
		StartedAt:     timePointer(now),
		ExpiresAt:     timePointer(now.Add(30 * 24 * time.Hour)),
	}
}

func sameServiceState(left, right StoreServiceState) bool {
	return left.RecordStatus == right.RecordStatus && left.ServiceStatus == right.ServiceStatus && sameTimePointer(left.StartedAt, right.StartedAt) && sameTimePointer(left.ExpiresAt, right.ExpiresAt)
}

func sameTimePointer(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == right
	}
	return left.Equal(*right)
}

func timePointer(value time.Time) *time.Time { return &value }
