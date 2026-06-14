package submission

import "time"

const InFlightTTL = 15 * time.Minute

type InFlightState struct {
	AttemptCount      int
	CurrentAction     string
	CurrentPhase      string
	CurrentRequestID  string
	InFlightStartedAt *time.Time
	LeaseExpiresAt    *time.Time
}

func BeginInFlightState(state InFlightState, action, requestID, phase string, startedAt time.Time, ttl time.Duration) InFlightState {
	leaseExpiresAt := startedAt.Add(ttl)
	return InFlightState{
		AttemptCount:      state.AttemptCount + 1,
		CurrentAction:     action,
		CurrentPhase:      phase,
		CurrentRequestID:  requestID,
		InFlightStartedAt: &startedAt,
		LeaseExpiresAt:    &leaseExpiresAt,
	}
}

func AdvanceInFlightState(state InFlightState, action, requestID, phase string, now time.Time, ttl time.Duration) InFlightState {
	leaseExpiresAt := now.Add(ttl)
	state.CurrentAction = action
	state.CurrentPhase = phase
	state.CurrentRequestID = requestID
	state.LeaseExpiresAt = &leaseExpiresAt
	return state
}

func ShouldClearInFlight(currentAction, currentRequestID, action, requestID string) bool {
	return currentAction == action && currentRequestID == requestID
}
