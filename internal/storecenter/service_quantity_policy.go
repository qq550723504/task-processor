package storecenter

import (
	"context"
	"strings"
)

const Phase1MaxStoreServicePeriods int64 = 12

// Phase1ServiceQuantityPolicy owns the approved server-side quantity limits.
// Callers cannot widen these bounds through HTTP or composition settings.
type Phase1ServiceQuantityPolicy struct{}

func (Phase1ServiceQuantityPolicy) MaxQuantity(ctx context.Context, organizationID string, command ServiceCommand) (int64, error) {
	if ctx == nil || strings.TrimSpace(organizationID) == "" {
		return 0, ErrServiceQuantityInvalid
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	switch command {
	case ServiceCommandActivate:
		return 1, nil
	case ServiceCommandRenew, ServiceCommandReactivate:
		return Phase1MaxStoreServicePeriods, nil
	default:
		return 0, ErrServiceQuantityInvalid
	}
}

var _ ServiceQuantityPolicy = Phase1ServiceQuantityPolicy{}
