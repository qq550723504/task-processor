package orgresource

import (
	"context"
	"time"
)

type EventReader interface {
	ListEvents(context.Context, EventQuery) (EventPage, error)
}

type EventQuery struct {
	OrganizationID string
	ResourceType   ResourceType
	From, Until    *time.Time
	Cursor         string
	Limit          int
}

type EventView struct {
	EventID        string       `json:"event_id"`
	OperationID    string       `json:"operation_id"`
	ResourceType   ResourceType `json:"resource_type"`
	Quantity       string       `json:"quantity"`
	AvailableDelta string       `json:"available_delta"`
	AllocatedDelta string       `json:"allocated_delta"`
	ReservedDelta  string       `json:"reserved_delta"`
	ConsumedDelta  string       `json:"consumed_delta"`
	AvailableAfter string       `json:"available_after"`
	AllocatedAfter string       `json:"allocated_after"`
	ReservedAfter  string       `json:"reserved_after"`
	ConsumedAfter  string       `json:"consumed_after"`
	Reason         string       `json:"reason"`
	SourceType     string       `json:"source_type"`
	SourceIdentity string       `json:"source_identity"`
	OccurredAt     time.Time    `json:"occurred_at"`
}

type EventPage struct {
	OrganizationID string      `json:"organization_id"`
	Items          []EventView `json:"items"`
	NextCursor     *string     `json:"next_cursor"`
}
