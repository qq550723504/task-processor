package submission

import "fmt"

func EnsureEventID(id, action string, nowUnixNanoSource interface{ UnixNano() int64 }) string {
	if id != "" {
		return id
	}
	return fmt.Sprintf("%s-%d", action, nowUnixNanoSource.UnixNano())
}

func PrependRecentEvents[T any](events []T, event T, limit int) []T {
	events = append([]T{event}, events...)
	if limit > 0 && len(events) > limit {
		events = events[:limit]
	}
	return events
}
