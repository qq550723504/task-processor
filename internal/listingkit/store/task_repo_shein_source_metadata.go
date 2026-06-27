package store

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"task-processor/internal/listingkit"
)

func (r *taskRepository) ListSheinSourceSDSMetadata(ctx context.Context, query *listingkit.SheinSourceSDSMetadataQuery) ([]listingkit.SheinSourceSDSMetadataRecord, error) {
	targets := normalizedSheinSourceSDSTargets(query)
	if query == nil || query.StoreID <= 0 || len(targets) == 0 {
		return []listingkit.SheinSourceSDSMetadataRecord{}, nil
	}
	targetFamilies := sheinSourceSDSFamilyPrefixes(targets)

	db := applySheinSourceSDSMetadataAccessScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), ctx)
	db = applySheinSourceSDSMetadataStoreScope(db, query.StoreID)
	db = applySheinSourceSDSMetadataTargetScope(db, targets)

	var tasks []listingkit.Task
	if err := db.Select("id", "tenant_id", "user_id", "request", "created_at").
		Order("created_at DESC").
		Limit(5000).
		Find(&tasks).Error; err != nil {
		return nil, err
	}
	items := collectSheinSourceSDSMetadata(tasks, query.StoreID, targets, targetFamilies)
	missingTargets := missingSheinSourceSDSTargets(targets, items)
	if len(missingTargets) == 0 && len(targetFamilies) == 0 {
		return items, nil
	}

	fallbackDB := applySheinSourceSDSMetadataStoreScope(r.db.WithContext(ctx).Model(&listingkit.Task{}), query.StoreID)
	if len(targetFamilies) == 0 {
		fallbackDB = applySheinSourceSDSMetadataTargetScope(fallbackDB, missingTargets)
	}
	var fallbackTasks []listingkit.Task
	if err := fallbackDB.Select("id", "tenant_id", "user_id", "request", "created_at").
		Order("created_at DESC").
		Limit(5000).
		Find(&fallbackTasks).Error; err != nil {
		return nil, err
	}
	return mergeSheinSourceSDSMetadata(
		items,
		collectSheinSourceSDSMetadata(fallbackTasks, query.StoreID, targets, targetFamilies),
		targets,
	), nil
}

func applySheinSourceSDSMetadataAccessScope(db *gorm.DB, ctx context.Context) *gorm.DB {
	if !listingkit.RequestHasPlatformAdminAccess(ctx) {
		userID := strings.TrimSpace(listingkit.RequestUserIDFromContext(ctx))
		if userID != "" {
			return applyTaskUserScope(db, userID)
		}
	}
	return applyTenantScope(db, ctx, "tenant_id")
}

func applySheinSourceSDSMetadataStoreScope(db *gorm.DB, storeID int64) *gorm.DB {
	switch db.Dialector.Name() {
	case "postgres":
		return db.Where("request IS NOT NULL").
			Where("(request::jsonb ->> 'shein_store_id') = ?", strconv.FormatInt(storeID, 10)).
			Where("(request::jsonb -> 'options' -> 'sds') IS NOT NULL")
	case "sqlite":
		return db.Where("request IS NOT NULL").
			Where("CAST(json_extract(request, '$.shein_store_id') AS INTEGER) = ?", storeID).
			Where("json_extract(request, '$.options.sds') IS NOT NULL")
	default:
		return db.Where("request IS NOT NULL")
	}
}

func applySheinSourceSDSMetadataTargetScope(db *gorm.DB, targets map[string]string) *gorm.DB {
	codes := sortedSheinSourceSDSTargetCodes(targets)
	if len(codes) == 0 {
		return db
	}
	switch db.Dialector.Name() {
	case "postgres":
		return db.Where(
			`(
				UPPER(BTRIM(COALESCE(request::jsonb -> 'options' -> 'sds' ->> 'variant_sku', ''))) IN ?
				OR UPPER(BTRIM(COALESCE(request::jsonb -> 'options' -> 'sds' ->> 'product_sku', ''))) IN ?
				OR EXISTS (
					SELECT 1
					FROM jsonb_array_elements(COALESCE(request::jsonb -> 'options' -> 'sds' -> 'variants', '[]'::jsonb)) AS variant
					WHERE UPPER(BTRIM(COALESCE(variant ->> 'variant_sku', ''))) IN ?
				)
			)`,
			codes,
			codes,
			codes,
		)
	case "sqlite":
		return db.Where(
			`(
				UPPER(TRIM(COALESCE(json_extract(request, '$.options.sds.variant_sku'), ''))) IN ?
				OR UPPER(TRIM(COALESCE(json_extract(request, '$.options.sds.product_sku'), ''))) IN ?
				OR EXISTS (
					SELECT 1
					FROM json_each(COALESCE(json_extract(request, '$.options.sds.variants'), '[]')) AS variant
					WHERE UPPER(TRIM(COALESCE(json_extract(variant.value, '$.variant_sku'), ''))) IN ?
				)
			)`,
			codes,
			codes,
			codes,
		)
	default:
		return db
	}
}

func normalizedSheinSourceSDSTargets(query *listingkit.SheinSourceSDSMetadataQuery) map[string]string {
	targets := map[string]string{}
	if query == nil {
		return targets
	}
	for _, code := range query.SourceCodes {
		normalized := normalizeSheinSourceSDSCode(code)
		if normalized == "" {
			continue
		}
		targets[normalized] = normalized
	}
	return targets
}

func sheinSourceSDSFamilyPrefixes(targets map[string]string) map[string]string {
	families := map[string]string{}
	for _, code := range targets {
		if family := sheinSourceSDSFamilyPrefix(code); family != "" {
			families[family] = family
		}
	}
	return families
}

func sheinSourceSDSFamilyPrefix(value string) string {
	code := normalizeSheinSourceSDSCode(value)
	if len(code) <= 4 {
		return ""
	}
	for _, r := range code[len(code)-4:] {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return code[:len(code)-4]
}

func collectSheinSourceSDSMetadata(
	tasks []listingkit.Task,
	storeID int64,
	targets map[string]string,
	targetFamilies map[string]string,
) []listingkit.SheinSourceSDSMetadataRecord {
	found := map[string]listingkit.SheinSourceSDSMetadataRecord{}
	for i := range tasks {
		req := tasks[i].Request
		if req == nil || req.SheinStoreID != storeID || req.Options == nil || req.Options.SDS == nil {
			continue
		}
		records := sheinSourceSDSMetadataRecords(req.Options.SDS)
		matched := false
		for _, record := range records {
			for _, key := range []string{record.VariantSKU, record.ProductSKU} {
				if sheinSourceSDSMetadataMatchesTarget(key, targets, targetFamilies) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			continue
		}
		for _, record := range records {
			sourceCode := sheinSourceSDSMetadataSourceCode(record)
			if sourceCode == "" {
				continue
			}
			if _, exists := found[sourceCode]; exists {
				continue
			}
			record.SourceCode = sourceCode
			found[sourceCode] = record
		}
		if sheinSourceSDSMetadataCoversTargets(found, targets) {
			break
		}
	}

	out := make([]listingkit.SheinSourceSDSMetadataRecord, 0, len(found))
	for _, sourceCode := range sortedSheinSourceSDSMetadataCodes(found) {
		if record, ok := found[sourceCode]; ok {
			out = append(out, record)
		}
	}
	return out
}

func sheinSourceSDSMetadataMatchesTarget(
	value string,
	targets map[string]string,
	targetFamilies map[string]string,
) bool {
	code := normalizeSheinSourceSDSCode(value)
	if code == "" {
		return false
	}
	if _, ok := targets[code]; ok {
		return true
	}
	for family := range targetFamilies {
		if strings.HasPrefix(code, family) {
			return true
		}
	}
	return false
}

func sheinSourceSDSMetadataSourceCode(record listingkit.SheinSourceSDSMetadataRecord) string {
	for _, value := range []string{record.VariantSKU, record.SourceCode, record.ProductSKU} {
		if code := normalizeSheinSourceSDSCode(value); code != "" {
			return code
		}
	}
	return ""
}

func sheinSourceSDSMetadataCoversTargets(
	found map[string]listingkit.SheinSourceSDSMetadataRecord,
	targets map[string]string,
) bool {
	for target := range targets {
		if _, ok := found[target]; !ok {
			return false
		}
	}
	return true
}

func missingSheinSourceSDSTargets(targets map[string]string, items []listingkit.SheinSourceSDSMetadataRecord) map[string]string {
	if len(targets) == 0 {
		return map[string]string{}
	}
	missing := make(map[string]string, len(targets))
	for key, value := range targets {
		missing[key] = value
	}
	for _, item := range items {
		delete(missing, normalizeSheinSourceSDSCode(item.SourceCode))
	}
	return missing
}

func mergeSheinSourceSDSMetadata(primary, fallback []listingkit.SheinSourceSDSMetadataRecord, targets map[string]string) []listingkit.SheinSourceSDSMetadataRecord {
	records := make(map[string]listingkit.SheinSourceSDSMetadataRecord, len(primary)+len(fallback))
	for _, item := range fallback {
		if code := normalizeSheinSourceSDSCode(item.SourceCode); code != "" {
			records[code] = item
		}
	}
	for _, item := range primary {
		if code := normalizeSheinSourceSDSCode(item.SourceCode); code != "" {
			records[code] = item
		}
	}
	out := make([]listingkit.SheinSourceSDSMetadataRecord, 0, len(records))
	for _, sourceCode := range sortedSheinSourceSDSMetadataCodes(records) {
		if record, ok := records[sourceCode]; ok {
			out = append(out, record)
		}
	}
	return out
}

func sheinSourceSDSMetadataRecords(sds *listingkit.SDSSyncOptions) []listingkit.SheinSourceSDSMetadataRecord {
	if sds == nil {
		return nil
	}
	title := firstNonEmptyString(sds.ProductName, sds.ProductEnglishName)
	productSKU := strings.TrimSpace(sds.ProductSKU)
	productImageURL := firstSheinSourceSDSImageURL(sds.MockupImageURLs, sds.BlankDesignURL, sds.TemplateImageURL)
	records := make([]listingkit.SheinSourceSDSMetadataRecord, 0, len(sds.Variants)+1)
	if strings.TrimSpace(sds.VariantSKU) != "" || productSKU != "" {
		records = append(records, listingkit.SheinSourceSDSMetadataRecord{
			Title:        title,
			ProductSKU:   productSKU,
			VariantSKU:   strings.TrimSpace(sds.VariantSKU),
			Price:        sds.VariantPrice,
			VariantLabel: sheinSourceSDSVariantLabelWithFallback(sds.VariantSKU, sds.VariantColor, sds.VariantSize),
			ImageURL:     productImageURL,
		})
	}
	for _, variant := range sds.Variants {
		if strings.TrimSpace(variant.VariantSKU) == "" {
			continue
		}
		records = append(records, listingkit.SheinSourceSDSMetadataRecord{
			Title:        title,
			ProductSKU:   productSKU,
			VariantSKU:   strings.TrimSpace(variant.VariantSKU),
			Price:        variant.Price,
			VariantLabel: sheinSourceSDSVariantLabel(variant.Color, variant.Size, ""),
			ImageURL: firstSheinSourceSDSImageURL(
				[]string{variant.MockupImageURL},
				variant.MockupImageURLs,
				[]string{productImageURL, variant.BlankDesignURL, variant.TemplateImageURL},
			),
		})
	}
	return records
}

func firstSheinSourceSDSImageURL(groups ...any) string {
	for _, group := range groups {
		switch value := group.(type) {
		case string:
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		case []string:
			for _, item := range value {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					return trimmed
				}
			}
		}
	}
	return ""
}

func sheinSourceSDSVariantLabel(parts ...string) string {
	labels := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			labels = append(labels, trimmed)
		}
	}
	return strings.Join(labels, " / ")
}

func sheinSourceSDSVariantLabelWithFallback(fallback string, parts ...string) string {
	if label := sheinSourceSDSVariantLabel(parts...); label != "" {
		return label
	}
	return strings.TrimSpace(fallback)
}

func sortedSheinSourceSDSTargetCodes(targets map[string]string) []string {
	out := make([]string, 0, len(targets))
	for _, sourceCode := range targets {
		out = append(out, sourceCode)
	}
	sort.Strings(out)
	return out
}

func sortedSheinSourceSDSMetadataCodes(records map[string]listingkit.SheinSourceSDSMetadataRecord) []string {
	out := make([]string, 0, len(records))
	for sourceCode := range records {
		out = append(out, sourceCode)
	}
	sort.Strings(out)
	return out
}

func normalizeSheinSourceSDSCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
