package collection

// PublicationBatch is an internal producer port. A stable job/delivery ID groups
// exact immutable references; it grants no ability to choose a different owner.
type PublicationBatch struct{ ID, Name, Kind, OperationID string }

func NewPublicationBatch(scope Scope, operation, kind, name string) (PublicationBatch, error) {
	if scope.Validate() != nil || !ValidID(operation) || !validName(name) || (kind != "amazon_data" && kind != "custom_dataset") {
		return PublicationBatch{}, ErrInvalid
	}
	return PublicationBatch{ID: StableID(scope.OrganizationID, scope.ActorID, "data-batch", kind, operation), Name: name, Kind: kind, OperationID: operation}, nil
}
