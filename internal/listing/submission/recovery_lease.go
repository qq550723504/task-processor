package submission

import (
	"strings"
	"time"
)

type RecoveryLeaseState struct {
	CurrentAction     string
	CurrentRequestID  string
	CurrentPhase      string
	InFlightStartedAt *time.Time
	LeaseExpiresAt    *time.Time
}

func IsActiveAttempt(state RecoveryLeaseState, action string, now time.Time, ttl time.Duration) bool {
	if state.CurrentAction != action || state.CurrentRequestID == "" || state.CurrentPhase == "" || state.InFlightStartedAt == nil {
		return false
	}
	if state.LeaseExpiresAt != nil {
		return !now.After(*state.LeaseExpiresAt)
	}
	return now.Sub(*state.InFlightStartedAt) <= ttl
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

func NeedsRequestScopedRemoteRecovery(currentRequestID, currentPhase, requestedRequestID, remoteSubmitPhase string, responsePersisted bool) bool {
	currentRequestID = strings.TrimSpace(currentRequestID)
	requestedRequestID = strings.TrimSpace(requestedRequestID)
	if currentRequestID == "" || currentRequestID != requestedRequestID {
		return false
	}
	return strings.TrimSpace(currentPhase) != strings.TrimSpace(remoteSubmitPhase) || responsePersisted
}
