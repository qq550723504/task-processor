package orgresource

import (
	"context"
	"time"
)

// These are read projections of existing committed administration facts.
// Quantity is a transfer amount or the configured monthly cap, never a balance.
type MemberAuditEvent struct {
	OrganizationID, OperationID, ActorID, MemberID, Action string
	ResourceType                                           ResourceType
	Quantity, Version, ID                                  int64
	CreatedAt                                              time.Time
}

type MemberAuditPosition struct {
	CreatedAt time.Time
	ID        int64
}

type MemberAuditPage struct {
	Items []MemberAuditEvent
	Next  *MemberAuditPosition
}

type MemberAuditReader interface {
	ListMemberAudit(context.Context, string, int, string, string, *MemberAuditPosition) (MemberAuditPage, error)
}
