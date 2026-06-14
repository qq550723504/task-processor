package submission

import "strings"

func RefreshActionMatches(currentAction, requestedAction string) bool {
	currentAction = strings.TrimSpace(currentAction)
	if currentAction == "" {
		currentAction = strings.TrimSpace(requestedAction)
	}
	return currentAction == strings.TrimSpace(requestedAction)
}

func RefreshRequestMatches(currentRequestID, requestedRequestID string) bool {
	currentRequestID = strings.TrimSpace(currentRequestID)
	return currentRequestID != "" && currentRequestID == strings.TrimSpace(requestedRequestID)
}
