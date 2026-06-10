package listingkit

import (
	"context"
	"fmt"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func withSheinSubmitTaskIdentity(ctx context.Context, task *Task) (context.Context, error) {
	if task == nil {
		return nil, fmt.Errorf("shein submit task is required")
	}
	tenantID := strings.TrimSpace(task.TenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("shein submit tenant id is unavailable")
	}

	identity := openaiclient.IdentityFromContext(ctx)
	identity.TenantID = tenantID
	if strings.TrimSpace(identity.UserID) == "" {
		identity.UserID = strings.TrimSpace(task.UserID)
	}

	ctx = WithTenantID(ctx, tenantID)
	return openaiclient.WithIdentity(ctx, identity), nil
}
