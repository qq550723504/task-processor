package storecenter

import (
	"errors"
	"testing"
	"time"
)

func TestValidateStoreServiceStateEnforcesRecordAndServiceInvariants(t *testing.T) {
	started := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC)
	expires := started.Add(30 * 24 * time.Hour)
	tests := []struct {
		name  string
		state StoreServiceState
		valid bool
	}{
		{name: "provisioning has no service", state: StoreServiceState{RecordStatus: RecordStatusProvisioning}, valid: true},
		{name: "deleting has no service", state: StoreServiceState{RecordStatus: RecordStatusDeleting}, valid: true},
		{name: "deleted has no service", state: StoreServiceState{RecordStatus: RecordStatusDeleted}, valid: true},
		{name: "active pending activation", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusPendingActivation}, valid: true},
		{name: "active service with ordered timestamps", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusActive, StartedAt: &started, ExpiresAt: &expires}, valid: true},
		{name: "expired service with ordered timestamps", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusExpired, StartedAt: &started, ExpiresAt: &expires}, valid: true},
		{name: "suspended without resolved history", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusSuspended}, valid: true},
		{name: "suspended with ordered history", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusSuspended, StartedAt: &started, ExpiresAt: &expires}, valid: true},
		{name: "missing record status", state: StoreServiceState{ServiceStatus: ServiceStatusPendingActivation}},
		{name: "active missing service status", state: StoreServiceState{RecordStatus: RecordStatusActive}},
		{name: "provisioning has service", state: StoreServiceState{RecordStatus: RecordStatusProvisioning, ServiceStatus: ServiceStatusPendingActivation}},
		{name: "pending activation has timestamps", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusPendingActivation, StartedAt: &started}},
		{name: "active missing timestamps", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusActive}},
		{name: "expiry is not after start", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusExpired, StartedAt: &expires, ExpiresAt: &started}},
		{name: "suspended has one timestamp", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusSuspended, StartedAt: &started}},
		{name: "unknown service status", state: StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatus("unknown")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStoreServiceState(tt.state)
			if (err == nil) != tt.valid {
				t.Fatalf("ValidateStoreServiceState() error = %v, valid = %t", err, tt.valid)
			}
			if !tt.valid && !errors.Is(err, ErrInvalidServiceState) {
				t.Fatalf("error = %v, want ErrInvalidServiceState", err)
			}
		})
	}
}

func TestValidateStoreServiceStateDoesNotMutateTimePointers(t *testing.T) {
	started := time.Date(2026, 9, 4, 1, 0, 0, 0, time.UTC)
	expires := started.Add(time.Hour)
	state := StoreServiceState{RecordStatus: RecordStatusActive, ServiceStatus: ServiceStatusActive, StartedAt: &started, ExpiresAt: &expires}
	if err := ValidateStoreServiceState(state); err != nil {
		t.Fatal(err)
	}
	if !state.StartedAt.Equal(started) || !state.ExpiresAt.Equal(expires) {
		t.Fatalf("state timestamps mutated: %#v", state)
	}
}
