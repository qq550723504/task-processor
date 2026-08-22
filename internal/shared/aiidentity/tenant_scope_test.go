package aiidentity

import (
	"context"
	"testing"
)

func TestTenantIDFromContext(t *testing.T) {
	scoped := WithIdentity(context.Background(), Identity{TenantID: " tenant-a "})
	if got := TenantIDFromContext(scoped); got != "tenant-a" {
		t.Fatalf("TenantIDFromContext() = %q, want tenant-a", got)
	}
	if got := TenantIDFromContext(nil); got != "" {
		t.Fatalf("TenantIDFromContext(nil) = %q, want empty", got)
	}
}

func TestNormalizePersistedTenantID(t *testing.T) {
	tests := []struct {
		name     string
		tenantID string
		want     string
	}{
		{name: "canonical", tenantID: "tenant-a", want: "tenant-a"},
		{name: "ASCII space padding", tenantID: " tenant-a ", want: "tenant-a"},
		{name: "tab padding is noncanonical", tenantID: "\ttenant-a\t", want: "\ttenant-a\t"},
		{name: "Unicode space padding is noncanonical", tenantID: "\u00a0tenant-a\u00a0", want: "\u00a0tenant-a\u00a0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizePersistedTenantID(tt.tenantID); got != tt.want {
				t.Fatalf("NormalizePersistedTenantID(%q) = %q, want %q", tt.tenantID, got, tt.want)
			}
		})
	}
}

func TestTenantMatchesContext(t *testing.T) {
	scoped := WithIdentity(context.Background(), Identity{TenantID: " tenant-a "})

	tests := []struct {
		name              string
		ctx               context.Context
		persistedTenantID string
		want              bool
	}{
		{name: "matching verified tenant", ctx: scoped, persistedTenantID: "tenant-a", want: true},
		{name: "different verified tenant", ctx: scoped, persistedTenantID: "tenant-b", want: false},
		{name: "ASCII-space persisted tenant is normalized", ctx: scoped, persistedTenantID: " tenant-a ", want: true},
		{name: "tab-padded persisted tenant fails closed", ctx: scoped, persistedTenantID: "\ttenant-a\t", want: false},
		{name: "Unicode-space-padded persisted tenant fails closed", ctx: scoped, persistedTenantID: "\u00a0tenant-a\u00a0", want: false},
		{name: "empty verified tenant remains unscoped", ctx: context.Background(), persistedTenantID: "tenant-b", want: true},
		{name: "nil context remains unscoped", ctx: nil, persistedTenantID: "tenant-b", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TenantMatchesContext(tt.ctx, tt.persistedTenantID); got != tt.want {
				t.Fatalf("TenantMatchesContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTenantCanCreateTask(t *testing.T) {
	scoped := WithIdentity(context.Background(), Identity{TenantID: " tenant-a ", UserID: "user-a"})
	matching := PersistedExecutionEnvelope{
		ExecutionIdentityVersion: CurrentEnvelopeVersion,
		ExecutionTenantID:        " tenant-a ",
		ExecutionUserID:          "user-a",
		ExecutionSourcePlatform:  "amazon",
		ExecutionSourceTaskType:  "listing",
	}

	tests := []struct {
		name      string
		ctx       context.Context
		persisted PersistedExecutionEnvelope
		want      bool
	}{
		{name: "unscoped worker permits legacy row", ctx: context.Background(), want: true},
		{name: "matching scoped envelope", ctx: scoped, persisted: matching, want: true},
		{name: "cross tenant scoped envelope", ctx: scoped, persisted: func() PersistedExecutionEnvelope {
			value := matching
			value.ExecutionTenantID = "tenant-b"
			return value
		}(), want: false},
		{name: "absent scoped envelope", ctx: scoped, want: false},
		{name: "partial scoped envelope", ctx: scoped, persisted: PersistedExecutionEnvelope{ExecutionTenantID: "tenant-a"}, want: false},
		{name: "tab padded scoped envelope", ctx: scoped, persisted: func() PersistedExecutionEnvelope {
			value := matching
			value.ExecutionTenantID = "\ttenant-a\t"
			return value
		}(), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := TenantCanCreateTask(tt.ctx, tt.persisted); got != tt.want {
				t.Fatalf("TenantCanCreateTask() = %v, want %v", got, tt.want)
			}
		})
	}
}
