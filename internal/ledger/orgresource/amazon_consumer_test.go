package orgresource

import (
	"strings"
	"testing"
)

func TestAmazonConsumerAdmitsOnlyOnePrepaidDataRow(t *testing.T) {
	i := ConsumerChargeIntent{Identity: ConsumerChargeIdentity{OrganizationID: "org", Consumer: "amazon_data_v1", OperationID: "row"}, ActorID: "actor", MemberID: "member", Funding: FundingMember, ResourceType: ResourceDataRow, Quantity: 1, Fingerprint: strings.Repeat("a", 64), BusinessScope: "amazon:us:B000123456"}
	if !ValidConsumerChargeIntent(i) {
		t.Fatal("Amazon single DATA_ROW rejected")
	}
	for _, qty := range []int64{0, 2, -1} {
		i.Quantity = qty
		if ValidConsumerChargeIntent(i) {
			t.Fatal("invalid quantity accepted")
		}
	}
	i.Quantity = 1
	i.ResourceType = ResourceStoreRenewalPeriod
	if ValidConsumerChargeIntent(i) {
		t.Fatal("wrong resource accepted")
	}
}
