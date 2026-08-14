package listingkit

import (
	"context"
	"fmt"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/tenantbridge"
)

func (s *service) listSheinStoreOptions(ctx context.Context) []SheinStoreOption {
	options, _ := s.listSheinStoreOptionsWithError(ctx)
	return options
}

func (s *service) listSheinStoreOptionsWithError(ctx context.Context) ([]SheinStoreOption, error) {
	storeCatalog := resolveSheinStoreCatalog(s)
	if s == nil || storeCatalog == nil {
		return nil, nil
	}
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil, nil
	}
	options, err := storeCatalog.ListStoreOptions(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list SHEIN store options: %w", err)
	}
	return append([]SheinStoreOption(nil), options...), nil
}

func tenantIDInt64FromContext(ctx context.Context) (int64, bool) {
	identity := RequestIdentityFromContext(ctx)
	tenantID := strings.TrimSpace(identity.TenantID)
	if tenantID == "" {
		return 0, false
	}
	value, err := tenantbridge.ResolveLegacyTenantID(ctx, tenantID)
	if err != nil || value <= 0 {
		return 0, false
	}
	return value, true
}

func tenantIDInt64FromTask(task *Task) int64 {
	if task == nil {
		return 0
	}
	value, err := tenantbridge.ResolveLegacyTenantID(context.Background(), strings.TrimSpace(task.TenantID))
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func sheinSubmitPayloadSettings(settings SheinSettings) sheinpub.SubmitPayloadSettings {
	return sheinpub.SubmitPayloadSettings{
		Site:          settings.Site,
		WarehouseCode: settings.WarehouseCode,
	}
}
