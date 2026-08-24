package listingadmin

import (
	"context"
	"errors"
	"testing"
)

func TestRequireOwnerUserIDRejectsMissingAndWhitespaceOwners(t *testing.T) {
	for _, explicit := range []string{"", " ", "\t\n"} {
		if got, err := requireOwnerUserID(context.Background(), explicit); !errors.Is(err, ErrOwnerUserIDRequired) || got != "" {
			t.Fatalf("requireOwnerUserID(%q) = %q, %v; want empty owner error", explicit, got, err)
		}
	}
}

func TestRequireOwnerUserIDUsesVerifiedContextIdentityOverPayload(t *testing.T) {
	ctx := withRequestUserID(context.Background(), " verified-sub ")
	got, err := requireOwnerUserID(ctx, "payload-subject")
	if err != nil || got != "verified-sub" {
		t.Fatalf("requireOwnerUserID() = %q, %v; want verified-sub", got, err)
	}
}

func TestRequireOwnerUserIDAcceptsExplicitTrustedInternalOwner(t *testing.T) {
	got, err := requireOwnerUserID(context.Background(), " internal-sub ")
	if err != nil || got != "internal-sub" {
		t.Fatalf("requireOwnerUserID() = %q, %v; want internal-sub", got, err)
	}
}

func TestWithOwnerUserIDSuppliesInternalOwner(t *testing.T) {
	got, err := requireOwnerUserID(WithOwnerUserID(context.Background(), "job-sub"), "")
	if err != nil || got != "job-sub" {
		t.Fatalf("requireOwnerUserID(WithOwnerUserID()) = %q, %v; want job-sub", got, err)
	}
}
