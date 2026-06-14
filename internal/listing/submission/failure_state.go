package submission

import "strings"

// ResolveFailureState resolves the request ID and phase used when recording a
// submission failure. Explicit values win; otherwise current in-flight state and
// the supplied default phase are used.
func ResolveFailureState(requestedID, phase, currentID, currentPhase, defaultPhase string) (string, string) {
	requestID := strings.TrimSpace(requestedID)
	resolvedPhase := strings.TrimSpace(phase)
	defaultPhase = strings.TrimSpace(defaultPhase)
	if resolvedPhase == "" {
		resolvedPhase = defaultPhase
	}
	currentID = strings.TrimSpace(currentID)
	currentPhase = strings.TrimSpace(currentPhase)
	if requestID == "" {
		requestID = currentID
	}
	if resolvedPhase == defaultPhase && currentPhase != "" {
		resolvedPhase = currentPhase
	}
	return requestID, resolvedPhase
}
