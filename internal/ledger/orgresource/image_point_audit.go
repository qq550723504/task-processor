package orgresource

import "time"

// ImagePointDebit is a read-only projection of the canonical resource event
// and its original reservation. It is not a new usage or balance fact.
type ImagePointDebit struct {
	OrganizationID, EventID, ActorID, MemberID, RunID, IntentID, PriceVersion string
	Points                                                                    int64
	CreatedAt                                                                 time.Time
}

type ImagePointAuditPosition struct {
	CreatedAt time.Time
	EventID   string
}

type ImagePointAuditPage struct {
	Items []ImagePointDebit
	Next  *ImagePointAuditPosition
}
