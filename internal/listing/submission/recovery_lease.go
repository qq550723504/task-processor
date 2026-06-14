package submission

import "time"

type RecoveryLeaseState struct {
	CurrentAction     string
	CurrentRequestID  string
	CurrentPhase      string
	InFlightStartedAt *time.Time
	LeaseExpiresAt    *time.Time
}

func NeedsRemoteRecovery(state RecoveryLeaseState, action string, now time.Time, ttl time.Duration, recoverablePhases map[string]struct{}) bool {
	if state.CurrentAction != action || state.CurrentRequestID == "" {
		return false
	}
	if _, ok := recoverablePhases[state.CurrentPhase]; !ok {
		return false
	}
	if state.LeaseExpiresAt != nil {
		return now.After(*state.LeaseExpiresAt)
	}
	if state.InFlightStartedAt == nil {
		return true
	}
	return now.Sub(*state.InFlightStartedAt) > ttl
}
