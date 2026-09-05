package storecenter

import (
	"context"
	"errors"
	"testing"
)

func TestPhase1ServiceQuantityPolicyUsesApprovedCommandLimits(t *testing.T) {
	policy := Phase1ServiceQuantityPolicy{}
	tests := []struct {
		command ServiceCommand
		want    int64
	}{
		{command: ServiceCommandActivate, want: 1},
		{command: ServiceCommandRenew, want: 12},
		{command: ServiceCommandReactivate, want: 12},
	}
	for _, test := range tests {
		got, err := policy.MaxQuantity(context.Background(), "org-a", test.command)
		if err != nil || got != test.want {
			t.Fatalf("MaxQuantity(%q) = %d, %v; want %d, nil", test.command, got, err, test.want)
		}
	}
}

func TestPhase1ServiceQuantityPolicyFailsClosedOnInvalidInput(t *testing.T) {
	policy := Phase1ServiceQuantityPolicy{}
	if _, err := policy.MaxQuantity(context.Background(), "", ServiceCommandRenew); !errors.Is(err, ErrServiceQuantityInvalid) {
		t.Fatalf("empty organization error = %v, want ErrServiceQuantityInvalid", err)
	}
	if _, err := policy.MaxQuantity(context.Background(), "org-a", ServiceCommand("unknown")); !errors.Is(err, ErrServiceQuantityInvalid) {
		t.Fatalf("unknown command error = %v, want ErrServiceQuantityInvalid", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := policy.MaxQuantity(cancelled, "org-a", ServiceCommandRenew); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled context error = %v, want context.Canceled", err)
	}
}
