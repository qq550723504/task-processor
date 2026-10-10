package collection

import "testing"

func TestPublicationBatchKeepsJobRowsTogetherAndSourcesHonest(t *testing.T) {
	scope := Scope{OrganizationID: "org", ActorID: "actor", MemberID: "member"}
	job := StableID("job")
	first, err := NewPublicationBatch(scope, job, "amazon_data", "Amazon · us")
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewPublicationBatch(scope, job, "amazon_data", "Amazon · us")
	if err != nil || first.ID != second.ID {
		t.Fatal("same job split into different batches")
	}
	other, _ := NewPublicationBatch(Scope{OrganizationID: "org", ActorID: "other", MemberID: "other-member"}, job, "amazon_data", "Amazon · us")
	if first.ID == other.ID {
		t.Fatal("actor scopes collapsed")
	}
	if _, err := NewPublicationBatch(scope, job, "own", "Amazon · us"); err == nil {
		t.Fatal("declared data can masquerade as own product")
	}
}
