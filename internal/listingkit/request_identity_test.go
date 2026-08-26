package listingkit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"task-processor/internal/shared/aiidentity"
)

func TestRequestIdentityContextPreservesNeutralIdentityMetadata(t *testing.T) {
	ctx := aiidentity.WithIdentity(context.Background(), aiidentity.Identity{
		BusinessTaskID: "task-1",
		TraceID:        "trace-1",
	})

	ctx = WithRequestIdentity(ctx, RequestIdentity{TenantID: " tenant-a ", UserID: " user-a "})

	require.Equal(t, RequestIdentity{TenantID: "tenant-a", UserID: "user-a"}, RequestIdentityFromContext(ctx))
	require.Equal(t, aiidentity.Identity{
		TenantID:       "tenant-a",
		UserID:         "user-a",
		BusinessTaskID: "task-1",
		TraceID:        "trace-1",
	}, aiidentity.FromContext(ctx))
}
