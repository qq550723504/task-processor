package shein

import (
	"context"
	"strings"
	"unicode"

	common "task-processor/internal/publishing/common"
	sheincontent "task-processor/internal/shein/content"
	sheinctx "task-processor/internal/shein/context"
	"task-processor/internal/shein/submitprep"
)

func sanitizeSheinListingCopy(copy *listingCopy, runtimeCtx context.Context, ctx *sheinctx.TaskContext) bool {
	if copy == nil {
		return false
	}
	service := submitprep.NewSensitiveWordServiceForContext(runtimeCtx)
	changed := false
	changed = sanitizeStringField(service, ctx, &copy.Title) || changed
	changed = sanitizeStringField(service, ctx, &copy.Description) || changed
	changed = sanitizeStringField(service, ctx, &copy.SKCTitleBase) || changed
	if copy.TitleDiagnostics != nil {
		copy.TitleDiagnostics.SKCBaseTitle = copy.SKCTitleBase
	}
	return changed
}

func SanitizeDraftPayloadSensitiveContent(pkg *Package, runtimeCtx context.Context, ctx *sheinctx.TaskContext) bool {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	service := submitprep.NewSensitiveWordServiceForContext(runtimeCtx)
	changed := false

	changed = sanitizeCommonAttributes(service, ctx, pkg.ProductAttributes) || changed
	changed = sanitizeLocalizedTexts(service, ctx, pkg.DraftPayload.MultiLanguageNameList) || changed
	changed = sanitizeLocalizedTexts(service, ctx, pkg.DraftPayload.MultiLanguageDescList) || changed
	changed = sanitizeCommonAttributes(service, ctx, pkg.DraftPayload.ProductAttributeList) || changed
	changed = sanitizeResolvedAttributes(service, ctx, pkg.ResolvedAttributes) || changed

	for i := range pkg.DraftPayload.SKCList {
		skc := &pkg.DraftPayload.SKCList[i]
		changed = sanitizeStringField(service, ctx, &skc.SkcName) || changed
		changed = sanitizeLocalizedTexts(service, ctx, skc.MultiLanguageNameList) || changed
	}

	return changed
}

func sanitizeResolvedAttributes(service *sheincontent.SensitiveWordService, ctx *sheinctx.TaskContext, attrs []ResolvedAttribute) bool {
	changed := false
	for i := range attrs {
		if attrs[i].AttributeValueID == nil && shouldSanitizeDraftAttributeValue(attrs[i].Value) {
			changed = sanitizeStringField(service, ctx, &attrs[i].Value) || changed
		}
		if shouldSanitizeDraftAttributeValue(attrs[i].AttributeExtraValue) {
			changed = sanitizeStringField(service, ctx, &attrs[i].AttributeExtraValue) || changed
		}
	}
	return changed
}

func sanitizeLocalizedTexts(service *sheincontent.SensitiveWordService, ctx *sheinctx.TaskContext, items []LocalizedText) bool {
	changed := false
	for i := range items {
		changed = sanitizeStringField(service, ctx, &items[i].Name) || changed
	}
	return changed
}

func sanitizeCommonAttributes(service *sheincontent.SensitiveWordService, ctx *sheinctx.TaskContext, attrs []common.Attribute) bool {
	changed := false
	for i := range attrs {
		if !shouldSanitizeDraftAttributeValue(attrs[i].Value) {
			continue
		}
		changed = sanitizeStringField(service, ctx, &attrs[i].Value) || changed
	}
	return changed
}

func sanitizeStringField(service *sheincontent.SensitiveWordService, ctx *sheinctx.TaskContext, value *string) bool {
	if service == nil || value == nil {
		return false
	}
	original := *value
	cleaned := service.SanitizeDisplayTextWithContext(ctx, original)
	if cleaned == original {
		return false
	}
	*value = cleaned
	return true
}

func shouldSanitizeDraftAttributeValue(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if len(value) >= 3 && strings.ContainsAny(value, " \t\r\n") {
		return true
	}
	hasLetter := false
	for _, r := range value {
		if unicode.IsLetter(r) {
			hasLetter = true
			continue
		}
		if unicode.IsDigit(r) || unicode.IsSpace(r) {
			continue
		}
		switch r {
		case '.', ',', '-', '_', '/', '\\', '+', '%', 'x', 'X':
			continue
		default:
			return hasLetter
		}
	}
	return false
}
