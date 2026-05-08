package generation

import "strings"

type ConditionalState struct {
	DeltaToken  string
	ETag        string
	NotModified bool
	NoChanges   bool
}

func BuildConditionalState(deltaToken string, notModified bool, noChanges bool) *ConditionalState {
	token := strings.TrimSpace(deltaToken)
	if token == "" && !notModified && !noChanges {
		return nil
	}
	return &ConditionalState{
		DeltaToken:  token,
		ETag:        ConditionalETag(token),
		NotModified: notModified,
		NoChanges:   noChanges,
	}
}

func ConditionalETag(deltaToken string) string {
	token := strings.TrimSpace(deltaToken)
	if token == "" {
		return ""
	}
	return `"` + token + `"`
}

func IsReadNotModified(queryDeltaToken, queryIfMatch, currentToken string) bool {
	currentToken = strings.TrimSpace(currentToken)
	if currentToken == "" {
		return false
	}
	if token := strings.TrimSpace(queryDeltaToken); token != "" && token == currentToken {
		return true
	}
	if token := strings.TrimSpace(queryIfMatch); token != "" && token == currentToken {
		return true
	}
	return false
}
