package ecoservices

import (
	"encoding/json"
	"testing"
	"time"
)

func TestQuoteFreezesServerAllocationPolicy(t *testing.T) {
	r := Request{ID: "request", BuyerOrganizationID: "buyer", ProviderOrganizationID: "provider", State: "REQUESTED", Version: 1}
	q := Quote{AmountMinor: 101, Scope: "scope", AcceptanceCriteria: "criteria", DeliveryDays: 3}
	_, err := TransitionRequest(&r, Command{Kind: "quote", Scope: Scope{OrganizationID: "provider"}, Version: 1, Quote: &q}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(r.Quote)
	var snapshot map[string]any
	_ = json.Unmarshal(raw, &snapshot)
	if snapshot["commissionBps"] != float64(1000) || snapshot["allocationBasis"] != "CHANNEL_SETTLEMENT_NET_FLOOR_V2" || snapshot["policyVersion"] != "ecoservices-v2-channel-net-10-platform-fee-manual-expiry" {
		t.Fatalf("quote lost original allocation policy: %s", raw)
	}
}
