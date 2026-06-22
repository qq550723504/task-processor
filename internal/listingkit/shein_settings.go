package listingkit

import (
	"context"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	"task-processor/internal/tenantbridge"
)

func (s *service) listSheinStoreOptions(ctx context.Context) []SheinStoreOption {
	storeCatalog := resolveSheinStoreCatalog(s)
	if s == nil || storeCatalog == nil {
		return nil
	}
	tenantID, ok := tenantIDInt64FromContext(ctx)
	if !ok {
		return nil
	}
	options, err := storeCatalog.ListStoreOptions(ctx, tenantID)
	if err != nil || len(options) == 0 {
		return nil
	}
	return append([]SheinStoreOption(nil), options...)
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
