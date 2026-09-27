package agent

import (
	"testing"
	"time"
)

func TestBudgetRejectsBeforeDispatch(t *testing.T) {
	limits := Limits{Steps: 4, ModelCalls: 2, Tokens: 100, CostMicros: 50, Currency: "USD", Runtime: time.Minute}
	for _, test := range []struct {
		name  string
		used  Usage
		quote Quote
		want  StopReason
	}{
		{"steps", Usage{Steps: 4}, Quote{Tokens: 1, CostMicros: 1, Currency: "USD", Known: true}, StopSteps},
		{"calls", Usage{ModelCalls: 2}, Quote{Tokens: 1, CostMicros: 1, Currency: "USD", Known: true}, StopModelCalls},
		{"tokens", Usage{Tokens: 99}, Quote{Tokens: 2, CostMicros: 1, Currency: "USD", Known: true}, StopTokens},
		{"cost", Usage{CostMicros: 49}, Quote{Tokens: 1, CostMicros: 2, Currency: "USD", Known: true}, StopCost},
		{"unknown", Usage{}, Quote{Tokens: 1, CostMicros: 1, Currency: "USD"}, StopUsageUnknown},
		{"currency", Usage{}, Quote{Tokens: 1, CostMicros: 1, Currency: "EUR", Known: true}, StopUsageUnknown},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := test.used
			if got := test.used.Reserve(limits, test.quote); got != test.want {
				t.Fatalf("got %q want %q", got, test.want)
			}
			if test.used != before {
				t.Fatal("rejected dispatch consumed budget")
			}
		})
	}
}

func TestUnknownUsageRetainsReservation(t *testing.T) {
	limits := Limits{Steps: 4, ModelCalls: 2, Tokens: 100, CostMicros: 50, Currency: "USD", Runtime: time.Minute}
	quote := Quote{Tokens: 20, CostMicros: 10, Currency: "USD", Known: true}
	used := Usage{}
	if stop := used.Reserve(limits, quote); stop != "" {
		t.Fatal(stop)
	}
	before := used
	if stop := used.Settle(quote, ObservedUsage{}); stop != StopUsageUnknown {
		t.Fatal(stop)
	}
	if used != before {
		t.Fatal("unknown consumption must not refund reservation")
	}
	if stop := used.Settle(quote, ObservedUsage{Tokens: 7, CostMicros: 3, Currency: "USD", Known: true}); stop != "" {
		t.Fatal(stop)
	}
	if used.Tokens != 7 || used.CostMicros != 3 || used.Steps != 1 || used.ModelCalls != 1 {
		t.Fatalf("wrong settlement: %+v", used)
	}
}

func TestExactBindingRequiresPlatformAndVersion(t *testing.T) {
	binding := Binding{ContextKind: "acquisition", ContextID: "op-1", ProductKey: "product-1", CatalogVersion: "1", PublicationID: "publication-1", TargetPlatform: "shein"}
	if !binding.Valid() {
		t.Fatal("valid binding rejected")
	}
	for _, version := range []string{"", "0", "01", "-1", "9223372036854775808"} {
		bad := binding
		bad.CatalogVersion = version
		if bad.Valid() {
			t.Fatalf("version %q accepted", version)
		}
	}
	binding.TargetPlatform = ""
	if binding.Valid() {
		t.Fatal("missing target platform accepted")
	}
}
