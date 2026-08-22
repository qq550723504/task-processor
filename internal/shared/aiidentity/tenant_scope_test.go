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
		{name: "persisted tenant is normalized", ctx: scoped, persistedTenantID: " tenant-a ", want: true},
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
