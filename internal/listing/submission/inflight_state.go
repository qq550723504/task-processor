package submission

func ShouldClearInFlight(currentAction, currentRequestID, action, requestID string) bool {
	return currentAction == action && currentRequestID == requestID
}
