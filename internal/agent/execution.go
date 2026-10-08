package agent

// ExecutionIdentity is an internal original durable command subject, resolved
// freshly by the configured application authority. It is never a JWT identity.
type ExecutionIdentity struct{ OrganizationID, ActorID, MemberID string }

func (i ExecutionIdentity) Valid() bool {
	return ValidID(i.OrganizationID) && ValidID(i.ActorID) && ValidID(i.MemberID)
}
