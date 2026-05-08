package listingkit

import (
	"strings"

	"task-processor/internal/catalog/canonical"
)

func shouldUseStudioProductFallback(task *Task) bool {
	return task != nil &&
		task.Request != nil &&
		shouldSyncSDS(task.Request) &&
		len(task.Request.ImageURLs) > 0
}

func shouldUseStudioCatalogCanonical(task *Task) bool {
	if !shouldUseStudioProductFallback(task) || task.Request.Options == nil {
		return false
	}
	return shouldUseSDSCatalogSource(task.Request)
}

func buildStudioFallbackCanonicalProduct(task *Task) *canonical.Product {
	if task == nil || task.Request == nil {
		return nil
	}

	options := task.Request.Options
	var sds *SDSSyncOptions
	if options != nil {
		sds = options.SDS
	}

	title := strings.TrimSpace(task.Request.Text)
	if sds != nil {
		title = firstNonEmptyString(sds.ProductName, title, sds.ProductEnglishName)
	}
	if title == "" {
		title = "SDS studio product"
	}

	sources := []canonical.Source{
		{Type: canonical.SourceUserImage, Detail: productenrichSummaryImageSources(task.Request.ImageURLs)},
	}
	if strings.TrimSpace(task.Request.Text) != "" {
		sources = append(sources, canonical.Source{
			Type:   canonical.SourceUserText,
			Detail: productenrichSummaryUserText(task.Request.Text),
		})
	}
	sources = append(sources, canonical.Source{
		Type:   canonical.SourceDerived,
		Detail: "listingkit studio fallback canonical product from SDS detail",
	})

	trace := canonical.FieldTrace{
		Sources:     sources,
		Confidence:  0.86,
		IsInferred:  true,
		NeedsReview: false,
	}

	images := make([]canonical.Image, 0, len(task.Request.ImageURLs))
	for index, imageURL := range task.Request.ImageURLs {
		role := "gallery"
		if index == 0 {
			role = "primary"
		}
		images = append(images, canonical.Image{
			URL:   imageURL,
			Role:  role,
			Trace: trace,
		})
	}

	fieldTraces := map[string]canonical.FieldTrace{
		"title":       trace,
		"description": trace,
	}

	description := strings.TrimSpace(task.Request.Text)
	if sds != nil {
		description = firstNonEmptyString(
			sds.ProductPerformance,
			sds.MaterialDescription,
			description,
			sds.ProductName,
		)
	}
	if description == "" {
		description = "SDS-backed studio design product."
	}

	return &canonical.Product{
		Title:          title,
		Description:    description,
		CategoryPath:   studioCategoryPath(sds),
		Images:         images,
		Attributes:     studioAttributes(sds, trace),
		Specifications: studioSpecifications(sds),
		Variants:       studioVariants(sds, images, trace),
		FieldTraces:    fieldTraces,
		NeedsReview:    false,
		SellingPoints:  studioSellingPoints(sds),
	}
}

func productenrichSummaryUserText(text string) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if text == "" {
		return ""
	}
	if len(text) <= 96 {
		return `user input: "` + text + `"`
	}
	return `user input: "` + text[:93] + `..."`
}

func productenrichSummaryImageSources(urls []string) string {
	if len(urls) == 0 {
		return ""
	}
	first := strings.TrimSpace(urls[0])
	if len(urls) == 1 {
		return "user image: " + first
	}
	return "user images: " + first
}
