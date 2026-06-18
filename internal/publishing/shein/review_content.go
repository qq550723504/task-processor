package shein

import (
	"context"
	"strings"
)

type ReviewContentOptimizer interface {
	OptimizeReviewContent(ctx context.Context, title, description, features string, imageURLs []string) (string, string, error)
}

// OptimizePackageReviewContent enhances the reviewed SHEIN copy before the
// workbench enters final confirmation. Submit-time preparation stays
// deterministic and reuses the reviewed content produced here.
func OptimizePackageReviewContent(ctx context.Context, pkg *Package, optimizer ReviewContentOptimizer) error {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}

	sourceTitle := strings.TrimSpace(firstNonEmpty(
		pkg.ProductNameEn,
		pkg.ProductNameMulti,
		pkg.SpuName,
		packageFirstDraftTitle(pkg),
	))
	sourceDescription := strings.TrimSpace(firstNonEmpty(
		pkg.Description,
		packageFirstDraftDescription(pkg),
	))
	if sourceTitle == "" && sourceDescription == "" {
		return nil
	}

	title := normalizeSheinContentText(sourceTitle)
	description := normalizeSheinContentText(sourceDescription)
	if title == "" {
		title = sourceTitle
	}
	if description == "" {
		description = sourceDescription
	}

	if optimizer != nil {
		optimizedTitle, optimizedDescription, err := optimizer.OptimizeReviewContent(
			ctx,
			title,
			description,
			buildPackageReviewContentFeatures(pkg),
			collectPackageReviewContentImageURLs(pkg),
		)
		if err == nil {
			title = optimizedTitle
			description = optimizedDescription
		}
	}

	title = strengthenSubmitTitle(title, sourceTitle, sourceDescription)
	description = strengthenSubmitDescription(description, sourceDescription)
	applyPackageReviewContent(ctx, pkg, title, description)
	return nil
}

func collectPackageReviewContentImageURLs(pkg *Package) []string {
	if pkg == nil {
		return nil
	}
	if pkg.Images != nil {
		if imageURL := strings.TrimSpace(pkg.Images.MainImage); imageURL != "" {
			return []string{imageURL}
		}
		for _, imageURL := range pkg.Images.Gallery {
			if imageURL = strings.TrimSpace(imageURL); imageURL != "" {
				return []string{imageURL}
			}
		}
		if imageURL := strings.TrimSpace(pkg.Images.WhiteBgImage); imageURL != "" {
			return []string{imageURL}
		}
	}
	if pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil {
		if imageURL := strings.TrimSpace(pkg.DraftPayload.ImageInfo.MainImage); imageURL != "" {
			return []string{imageURL}
		}
		for _, imageURL := range pkg.DraftPayload.ImageInfo.Gallery {
			if imageURL = strings.TrimSpace(imageURL); imageURL != "" {
				return []string{imageURL}
			}
		}
	}
	return nil
}

func buildPackageReviewContentFeatures(pkg *Package) string {
	if pkg == nil {
		return ""
	}
	parts := make([]string, 0, 12)
	if len(pkg.CategoryPath) > 0 {
		parts = append(parts, "Category path: "+strings.Join(pkg.CategoryPath, " > "))
	}
	for _, point := range pkg.SellingPoints {
		if point = strings.TrimSpace(point); point != "" {
			parts = append(parts, "Selling point: "+point)
		}
		if len(parts) >= 8 {
			break
		}
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if saleName := strings.TrimSpace(firstNonEmpty(skc.SaleName, skc.SkcName, firstLocalizedText(skc.MultiLanguageNameList))); saleName != "" {
				parts = append(parts, "Variant: "+saleName)
			}
			if len(parts) >= 10 {
				break
			}
		}
	}
	for _, attr := range pkg.ResolvedAttributes {
		if name := strings.TrimSpace(attr.Name); name != "" && strings.TrimSpace(attr.Value) != "" {
			parts = append(parts, "Attribute: "+name+"="+strings.TrimSpace(attr.Value))
		}
		if len(parts) >= 12 {
			break
		}
	}
	return strings.Join(parts, "\n")
}

func applyPackageReviewContent(ctx context.Context, pkg *Package, title, description string) {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return
	}
	title = truncateSubmitTitle(strings.TrimSpace(title), sheinSubmitTitleMaxLength)
	description = truncateSubmitDescription(strings.TrimSpace(description), sheinSubmitDescriptionMaxLength)
	if title == "" && description == "" {
		return
	}

	if title != "" {
		pkg.ProductNameEn = title
		pkg.ProductNameMulti = title
	}
	if description != "" {
		pkg.Description = description
	}

	language := packageDraftLanguage(pkg)
	if pkg.DraftPayload != nil {
		if title != "" {
			pkg.DraftPayload.MultiLanguageNameList = localizedEnglishText(language, title)
		}
		if description != "" {
			pkg.DraftPayload.MultiLanguageDescList = localizedEnglishText(language, description)
		}
		applyPackageReviewSKCContent(pkg.DraftPayload.SKCList, title)
	}
	applyPackageReviewPackageSKCContent(pkg.SkcList, title)
	SanitizeDraftPayloadSensitiveContent(pkg, ctx, nil)
	SetPreviewPayload(pkg, BuildPreviewProduct(pkg))
	NormalizePackageSemanticFields(pkg)
}

func applyPackageReviewSKCContent(items []SKCRequestDraft, title string) {
	for index := range items {
		name := buildSubmitSKCTitle(title, strings.TrimSpace(firstNonEmpty(items[index].SaleName, items[index].SkcName)))
		items[index].SkcName = name
		items[index].MultiLanguageNameList = []LocalizedText{{
			Language: "en",
			Name:     name,
		}}
	}
}

func applyPackageReviewPackageSKCContent(items []SKCPackage, title string) {
	for index := range items {
		items[index].SkcName = buildSubmitSKCTitle(title, strings.TrimSpace(firstNonEmpty(items[index].SaleName, items[index].SkcName)))
	}
}

func firstLocalizedText(items []LocalizedText) string {
	for _, item := range items {
		if text := strings.TrimSpace(item.Name); text != "" {
			return text
		}
	}
	return ""
}

func packageFirstDraftTitle(pkg *Package) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return ""
	}
	return firstLocalizedText(pkg.DraftPayload.MultiLanguageNameList)
}

func packageFirstDraftDescription(pkg *Package) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return ""
	}
	return firstLocalizedText(pkg.DraftPayload.MultiLanguageDescList)
}

func packageDraftLanguage(pkg *Package) string {
	pkg = NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return "en"
	}
	for _, item := range pkg.DraftPayload.MultiLanguageNameList {
		if language := strings.TrimSpace(item.Language); language != "" {
			return language
		}
	}
	for _, item := range pkg.DraftPayload.MultiLanguageDescList {
		if language := strings.TrimSpace(item.Language); language != "" {
			return language
		}
	}
	if language := strings.TrimSpace(pkg.Metadata["language"]); language != "" {
		return language
	}
	return "en"
}
