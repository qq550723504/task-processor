package design

import (
	"strings"
	"unicode"
)

const defaultSafeDesignProductName = "Custom Design Product"

func sanitizeSensitiveExportName(name string, hits []SensitiveWordHit) string {
	tokens := splitExportNameTokens(name)
	if len(tokens) == 0 {
		return defaultSafeDesignProductName
	}

	filtered := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if exportNameTokenBlocked(token, hits) {
			continue
		}
		filtered = append(filtered, token)
	}

	sanitized := strings.TrimSpace(strings.Join(filtered, " "))
	if sanitized == "" {
		return defaultSafeDesignProductName
	}
	return sanitized
}

func exportNameTokenBlocked(token string, hits []SensitiveWordHit) bool {
	token = strings.ToLower(strings.TrimSpace(token))
	if token == "" {
		return false
	}
	for _, hit := range hits {
		if !sensitiveWordTargetsExportName(hit) {
			continue
		}
		word := strings.ToLower(strings.TrimSpace(hit.SensitiveWord))
		if word == "" {
			continue
		}
		if strings.Contains(token, word) {
			return true
		}
	}
	return false
}

func sensitiveWordTargetsExportName(hit SensitiveWordHit) bool {
	position := strings.ToLower(strings.TrimSpace(hit.PositionStrs))
	if position == "" {
		return true
	}
	return strings.Contains(position, "导出名称") || strings.Contains(position, "export")
}

func splitExportNameTokens(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	return strings.FieldsFunc(name, func(r rune) bool {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
		switch r {
		case '-', '_':
			return false
		default:
			return true
		}
	})
}

func buildSensitiveDesignProductUpdates(items []DesignProductListItem, hitsByItemID map[string][]SensitiveWordHit) []UpdateDesignProductRequest {
	if len(items) == 0 || len(hitsByItemID) == 0 {
		return nil
	}
	flat := flattenDesignProductListItems(items)
	updates := make([]UpdateDesignProductRequest, 0, len(hitsByItemID))
	seen := make(map[string]struct{}, len(hitsByItemID))
	for itemID, hits := range hitsByItemID {
		itemID = strings.TrimSpace(itemID)
		if itemID == "" {
			continue
		}
		item, ok := flat[itemID]
		if !ok {
			continue
		}
		name := sanitizeSensitiveExportName(item.ExportName, hits)
		if name == "" || name == strings.TrimSpace(item.ExportName) {
			continue
		}
		if _, exists := seen[itemID]; exists {
			continue
		}
		seen[itemID] = struct{}{}
		updates = append(updates, UpdateDesignProductRequest{
			ID:                string(item.ID),
			Name:              name,
			MaterialImageName: item.MaterialImageName,
			MaterialColor:     item.MaterialColor,
			Keyword:           item.Keyword,
			Attributes:        append([]any(nil), item.Attributes...),
			ParentAttribute:   item.ParentAttribute,
		})
	}
	if len(updates) == 0 {
		return nil
	}
	return updates
}

func flattenDesignProductListItems(items []DesignProductListItem) map[string]DesignProductListItem {
	if len(items) == 0 {
		return nil
	}
	result := make(map[string]DesignProductListItem, len(items))
	var walk func(DesignProductListItem)
	walk = func(item DesignProductListItem) {
		itemID := strings.TrimSpace(string(item.ID))
		if itemID != "" {
			result[itemID] = item
		}
		for _, child := range item.MaterialVariant {
			walk(child)
		}
	}
	for _, item := range items {
		walk(item)
	}
	return result
}
