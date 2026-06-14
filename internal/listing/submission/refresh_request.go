package submission

import "strings"

func ResolveRefreshRequestID(requestID string) string {
	return strings.TrimSpace(requestID)
}
