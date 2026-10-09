package orgresource

import (
	"context"
	"testing"
)

type lookupRepo struct {
	ConsumerChargeRepository
	receipt ConsumerChargeReceipt
	reads   int
}

func (r *lookupRepo) Read(context.Context, ConsumerChargeIdentity) (ConsumerChargeReceipt, error) {
	r.reads++
	return r.receipt, nil
}

type lookupOwner struct{ ConsumerChargeOwner }

func TestOriginalConsumerLookupDoesNotReserveOrSettle(t *testing.T) {
	id := ConsumerChargeIdentity{OrganizationID: "org", Consumer: ConsumerAmazonData, OperationID: "original"}
	r := &lookupRepo{receipt: ConsumerChargeReceipt{Intent: ConsumerChargeIntent{Identity: id, ActorID: "actor", MemberID: "member", Funding: FundingMember, ResourceType: ResourceDataRow, Quantity: 1, Fingerprint: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", BusinessScope: "amazon:us:B000123456"}, ReservationID: "original-reservation", State: ReservationReserved}}
	s, err := NewConsumerChargeService(r, map[ResourceConsumer]ConsumerChargeOwner{ConsumerAmazonData: lookupOwner{}})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := s.Lookup(context.Background(), id)
	if err != nil || receipt.ReservationID != "original-reservation" || r.reads != 1 {
		t.Fatalf("%#v %v", receipt, err)
	}
}
