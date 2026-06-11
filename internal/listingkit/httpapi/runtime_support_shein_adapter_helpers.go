package httpapi

import (
	"context"
	"encoding/json"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
	"task-processor/internal/tenantbridge"
)

func tenantIDFromContext(ctx context.Context) int64 {
	identity := openaiclient.IdentityFromContext(ctx)
	if identity.TenantID == "" {
		return 0
	}
	tenantID, err := tenantbridge.ResolveLegacyTenantID(ctx, strings.TrimSpace(identity.TenantID))
	if err != nil || tenantID <= 0 {
		return 0
	}
	return tenantID
}

func tenantIDFromListingKitStoreInfo(storeInfo *listingkit.SheinStoreInfo) int64 {
	if storeInfo == nil {
		return 0
	}
	return storeInfo.TenantID
}

func tenantIDFromSheinClientStoreConfig(storeInfo *listingkit.SheinRuntimeStoreConfig) int64 {
	if storeInfo == nil {
		return 0
	}
	return storeInfo.TenantID
}

func toSheinClientStoreConfig(storeInfo *listingkit.SheinStoreInfo) *listingkit.SheinRuntimeStoreConfig {
	if storeInfo == nil {
		return nil
	}
	return &listingkit.SheinRuntimeStoreConfig{
		ID:       storeInfo.ID,
		TenantID: storeInfo.TenantID,
		StoreID:  strings.TrimSpace(storeInfo.StoreID),
		Name:     strings.TrimSpace(storeInfo.Name),
		Platform: strings.TrimSpace(storeInfo.Platform),
		Region:   strings.TrimSpace(storeInfo.Region),
		LoginURL: strings.TrimSpace(storeInfo.LoginURL),
		Proxy:    strings.TrimSpace(storeInfo.Proxy),
	}
}

func toSheinClientStoreConfigFromListingAdmin(store *listingadmin.Store) *listingkit.SheinRuntimeStoreConfig {
	if store == nil {
		return nil
	}
	return &listingkit.SheinRuntimeStoreConfig{
		ID:       store.ID,
		TenantID: store.TenantID,
		StoreID:  strings.TrimSpace(store.StoreID),
		Name:     strings.TrimSpace(store.Name),
		Platform: strings.TrimSpace(store.Platform),
		Region:   strings.TrimSpace(store.Region),
		LoginURL: strings.TrimSpace(store.LoginURL),
		Proxy:    strings.TrimSpace(store.Proxy),
	}
}

func normalizeSheinCookiePayload(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &wrapper); err == nil {
		if cookies, ok := wrapper["cookies"]; ok && len(cookies) > 0 && string(cookies) != "null" {
			return string(cookies), nil
		}
	}

	var list []json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &list); err == nil {
		return trimmed, nil
	}

	return trimmed, nil
}
