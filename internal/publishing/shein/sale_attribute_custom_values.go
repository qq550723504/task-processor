package shein

import (
	"context"
	"fmt"
	"strings"
	"sync"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

type customSaleAttributePermissionDeniedKey struct {
	categoryID  int
	attributeID int
}

var customSaleAttributePermissionDeniedCache sync.Map

func resolveCustomSaleAttributeValues(
	attr sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValues []string,
	scope string,
	api AttributeAPI,
	categoryID int,
	spuName string,
	storeID string,
	store ResolutionCacheStore,
	preserveSourceValues bool,
) (map[string]ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string) {
	if api == nil || categoryID <= 0 || len(sourceValues) == 0 || attr.AttributeID <= 0 {
		return nil, nil, nil
	}

	assignments := make(map[string]ResolvedSaleAttribute, len(sourceValues))
	relations := make([]sheinattribute.CustomAttributeRelation, 0, len(sourceValues))
	notes := make([]string, 0, len(sourceValues))
	if isCustomSaleAttributePermissionDenied(store, storeID, categoryID, attr.AttributeID) {
		notes = append(notes, fmt.Sprintf(
			"SHEIN 自定义销售属性值已跳过自定义尝试: 模板属性 %q 在类目 %d 已确认没有自定义属性值权限",
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
			categoryID,
		))
		return nil, nil, dedupeStrings(notes)
	}

	for _, sourceValue := range uniqueNormalizedValues(sourceValues) {
		attributeValue := sanitizeSheinAttributeText(sourceValue)
		if preserveSourceValues {
			attributeValue = strings.TrimSpace(sourceValue)
		}
		if attributeValue == "" || len(attributeValue) > 100 || (!preserveSourceValues && !isValidSheinAttributeText(attributeValue)) {
			notes = append(notes, fmt.Sprintf(
				"SHEIN 自定义销售属性值不可用: 源维度 %q 的值 %q 清洗后不符合 SHEIN 约束",
				sourceDimension,
				sourceValue,
			))
			continue
		}

		validateResp, err := api.ValidateCustomAttributeValue(attr.AttributeID, attributeValue, categoryID, strings.TrimSpace(spuName))
		if err != nil {
			if isCustomSaleAttributePermissionDeniedError(err) {
				rememberCustomSaleAttributePermissionDenied(store, storeID, categoryID, attr.AttributeID)
				notes = append(notes, fmt.Sprintf(
					"SHEIN 自定义销售属性值校验失败: 模板属性 %q 在类目 %d 没有自定义属性值权限，后续已跳过自定义尝试: %v",
					firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
					categoryID,
					err,
				))
				break
			}
			notes = append(notes, fmt.Sprintf(
				"SHEIN 自定义销售属性值校验失败: 模板属性 %q 的值 %q 校验报错: %v",
				firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
				sourceValue,
				err,
			))
			continue
		}
		if validateResp == nil || validateResp.Data.AttributeID == 0 || validateResp.Data.PreAttributeValueID == 0 {
			notes = append(notes, fmt.Sprintf(
				"SHEIN 自定义销售属性值不可创建: 模板属性 %q 未接受源值 %q",
				firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
				sourceValue,
			))
			continue
		}

		nameMultis := buildCustomAttributeValueNameMultis(validateResp.Data.AttributeValueNameMultis, attributeValue)
		if len(nameMultis) == 0 {
			notes = append(notes, fmt.Sprintf(
				"SHEIN 自定义销售属性值不可创建: 模板属性 %q 的值 %q 未返回可用的多语言名称",
				firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
				sourceValue,
			))
			continue
		}

		addResp, err := api.AddCustomAttributeValue(&sheinattribute.AddCustomAttributeValueRequest{
			CategoryID: categoryID,
			PreAttributeValueList: []sheinattribute.PreAttributeValue{{
				AttributeID:              attr.AttributeID,
				PreAttributeValueID:      int64(validateResp.Data.PreAttributeValueID),
				AttributeValue:           attributeValue,
				AttributeValueNameMultis: nameMultis,
			}},
		})
		if err != nil {
			notes = append(notes, fmt.Sprintf(
				"SHEIN 自定义销售属性值创建失败: 模板属性 %q 的值 %q 创建报错: %v",
				firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
				sourceValue,
				err,
			))
			continue
		}
		if addResp == nil || len(addResp.Info.Data.CustomAttributeRelation) == 0 {
			notes = append(notes, fmt.Sprintf(
				"SHEIN 自定义销售属性值创建失败: 模板属性 %q 的值 %q 未返回 attribute_value_id",
				firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
				sourceValue,
			))
			continue
		}

		relation := addResp.Info.Data.CustomAttributeRelation[0]
		valueID := int(relation.AttributeValueID)
		assignments[normalizeText(sourceValue)] = ResolvedSaleAttribute{
			Scope:            scope,
			Name:             firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
			Value:            sourceValue,
			AttributeID:      attr.AttributeID,
			AttributeValueID: &valueID,
			MatchedBy:        "custom_attribute_value",
		}
		relations = append(relations, addResp.Info.Data.CustomAttributeRelation...)
		notes = append(notes, fmt.Sprintf(
			"SHEIN 销售属性值使用自定义值承接: 模板属性 %q 的值 %q 已创建为自定义候选",
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
			sourceValue,
		))
	}

	if len(assignments) == 0 && len(relations) == 0 && len(notes) == 0 {
		return nil, nil, nil
	}
	return assignments, dedupeCustomAttributeRelations(relations), dedupeStrings(notes)
}

func isCustomSaleAttributePermissionDenied(store ResolutionCacheStore, storeID string, categoryID, attributeID int) bool {
	if categoryID <= 0 || attributeID <= 0 {
		return false
	}
	key := customSaleAttributePermissionDeniedKey{
		categoryID:  categoryID,
		attributeID: attributeID,
	}
	if _, ok := customSaleAttributePermissionDeniedCache.Load(key); ok {
		return true
	}
	if store == nil || strings.TrimSpace(storeID) == "" {
		return false
	}
	entry, err := store.GetResolutionCache(context.Background(), ResolutionCacheKindSaleAttributeCustomDenied, storeID, customSaleAttributePermissionDeniedCacheKey(categoryID, attributeID))
	if err != nil || entry == nil {
		return false
	}
	customSaleAttributePermissionDeniedCache.Store(key, struct{}{})
	return true
}

func rememberCustomSaleAttributePermissionDenied(store ResolutionCacheStore, storeID string, categoryID, attributeID int) {
	if categoryID <= 0 || attributeID <= 0 {
		return
	}
	key := customSaleAttributePermissionDeniedKey{
		categoryID:  categoryID,
		attributeID: attributeID,
	}
	customSaleAttributePermissionDeniedCache.Store(key, struct{}{})
	if store == nil || strings.TrimSpace(storeID) == "" {
		return
	}
	_ = store.SaveResolutionCache(context.Background(), &SheinResolutionCacheEntry{
		StoreID:        storeID,
		CacheKind:      ResolutionCacheKindSaleAttributeCustomDenied,
		CacheKey:       customSaleAttributePermissionDeniedCacheKey(categoryID, attributeID),
		ShortKey:       shortResolutionCacheKey(customSaleAttributePermissionDeniedCacheKey(categoryID, attributeID)),
		Source:         "runtime_permission_denied",
		Manual:         false,
		SourceIdentity: fmt.Sprintf("category=%d,attribute=%d", categoryID, attributeID),
		ResolutionJSON: `{"status":"custom_denied"}`,
	})
}

func isCustomSaleAttributePermissionDeniedError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "没有自定义属性值权限")
}

func resetCustomSaleAttributePermissionDeniedCache() {
	customSaleAttributePermissionDeniedCache = sync.Map{}
}

func customSaleAttributePermissionDeniedCacheKey(categoryID, attributeID int) string {
	return fmt.Sprintf("category:%d:attribute:%d", categoryID, attributeID)
}

func buildCustomAttributeValueNameMultis(source []struct {
	Language                string `json:"language"`
	AttributeValueNameMulti string `json:"attribute_value_name_multi"`
	WarningType             int    `json:"warning_type"`
}, fallbackValue string) []sheinattribute.AttributeValueNameMulti {
	if len(source) == 0 {
		return []sheinattribute.AttributeValueNameMulti{{
			Language:           "en",
			AttributeValueName: fallbackValue,
		}}
	}
	result := make([]sheinattribute.AttributeValueNameMulti, 0, len(source))
	for _, item := range source {
		language := strings.TrimSpace(item.Language)
		name := strings.TrimSpace(strings.ReplaceAll(item.AttributeValueNameMulti, "锛?", ","))
		if language == "" {
			continue
		}
		if name == "" {
			name = fallbackValue
		}
		result = append(result, sheinattribute.AttributeValueNameMulti{
			Language:           language,
			AttributeValueName: name,
			WarningType:        item.WarningType,
		})
	}
	if len(result) == 0 {
		return []sheinattribute.AttributeValueNameMulti{{
			Language:           "en",
			AttributeValueName: fallbackValue,
		}}
	}
	return result
}

func dedupeCustomAttributeRelations(relations []sheinattribute.CustomAttributeRelation) []sheinattribute.CustomAttributeRelation {
	if len(relations) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(relations))
	result := make([]sheinattribute.CustomAttributeRelation, 0, len(relations))
	for _, relation := range relations {
		key := fmt.Sprintf("%d:%d", relation.PreAttributeValueID, relation.AttributeValueID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, relation)
	}
	return result
}
