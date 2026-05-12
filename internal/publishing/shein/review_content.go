package shein

import (
	"context"
	"strings"

	openaiclient "task-processor/internal/infra/clients/openai"
	sheincontent "task-processor/internal/shein/content"
)

// OptimizePackageReviewContent enhances the reviewed SHEIN copy before the
// workbench enters final confirmation. Submit-time preparation stays
// deterministic and reuses the reviewed content produced here.
func OptimizePackageReviewContent(ctx context.Context, pkg *Package, aiClient openaiclient.ChatCompleter) error {
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

	cleaner := sheincontent.NewTextCleaner()
	title := cleaner.NormalizeText(sourceTitle)
	description := cleaner.NormalizeText(sourceDescription)
	if title == "" {
		title = sourceTitle
	}
	if description == "" {
		description = sourceDescription
	}

	if aiClient != nil {
		optimizedTitle, optimizedDescription, err := optimizeSubmitContentWithAI(
			ctx,
			aiClient,
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
	applyPackageReviewContent(pkg, title, description)
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
	if pkg.RequestDraft != nil && pkg.RequestDraft.ImageInfo != nil {
		if imageURL := strings.TrimSpace(pkg.RequestDraft.ImageInfo.MainImage); imageURL != "" {
			return []string{imageURL}
		}
		for _, imageURL := range pkg.RequestDraft.ImageInfo.Gallery {
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
	if pkg.RequestDraft != nil {
		for _, skc := range pkg.RequestDraft.SKCList {
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

func applyPackageReviewContent(pkg *Package, title, description string) {
	if pkg == nil {
		return
	}
	title = truncateSubmitTitle(strings.TrimSpace(title), 800)
	description = truncateSubmitDescription(strings.TrimSpace(description), 5000)
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
	if pkg.RequestDraft != nil {
		if title != "" {
			pkg.RequestDraft.MultiLanguageNameList = localizedEnglishText(language, title)
		}
		if description != "" {
			pkg.RequestDraft.MultiLanguageDescList = localizedEnglishText(language, description)
		}
		applyPackageReviewSKCContent(pkg.RequestDraft.SKCList, title)
	}
	applyPackageReviewPackageSKCContent(pkg.SkcList, title)
	pkg.PreviewProduct = BuildPreviewProduct(pkg)
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
	if pkg == nil || pkg.RequestDraft == nil {
		return ""
	}
	return firstLocalizedText(pkg.RequestDraft.MultiLanguageNameList)
}

func packageFirstDraftDescription(pkg *Package) string {
	if pkg == nil || pkg.RequestDraft == nil {
		return ""
	}
	return firstLocalizedText(pkg.RequestDraft.MultiLanguageDescList)
}

func packageDraftLanguage(pkg *Package) string {
	if pkg == nil || pkg.RequestDraft == nil {
		return "en"
	}
	for _, item := range pkg.RequestDraft.MultiLanguageNameList {
		if language := strings.TrimSpace(item.Language); language != "" {
			return language
		}
	}
	for _, item := range pkg.RequestDraft.MultiLanguageDescList {
		if language := strings.TrimSpace(item.Language); language != "" {
			return language
		}
	}
	if language := strings.TrimSpace(pkg.Metadata["language"]); language != "" {
		return language
	}
	return "en"
}
