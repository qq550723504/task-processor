package listingkit

import (
	"context"
	"fmt"
	"strings"
)

func withSheinSubmitTaskIdentity(ctx context.Context, task *Task) (context.Context, error) {
	if task == nil {
		return nil, fmt.Errorf("shein submit task is required")
	}
	tenantID := strings.TrimSpace(task.TenantID)
	if tenantID == "" {
		return nil, fmt.Errorf("shein submit tenant id is unavailable")
	}

	identity := RequestIdentityFromContext(ctx)
	identity.TenantID = tenantID
	if strings.TrimSpace(identity.UserID) == "" {
		identity.UserID = strings.TrimSpace(task.UserID)
	}

	ctx = WithTenantID(ctx, tenantID)
	ctx = withSheinTaskStoreAccess(ctx, task)
	return WithRequestIdentity(ctx, identity), nil
}

func withSheinTaskStoreAccess(ctx context.Context, task *Task) context.Context {
	if task == nil || task.SheinStoreResolutionSnapshot == nil || !task.SheinStoreResolutionSnapshot.TenantAdminAccess {
		return ctx
	}
	roles := append(RequestRolesFromContext(ctx), "listingkit_admin")
	return WithRequestRoles(ctx, roles)
}
