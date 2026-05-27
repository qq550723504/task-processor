package listingkit

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

func (s *service) RegenerateSheinDataImage(ctx context.Context, taskID string, req *RegenerateSheinDataImageRequest) (*RegenerateSheinDataImageResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("invalid request: request is required")
	}
	oldURL := strings.TrimSpace(req.ImageURL)
	if oldURL == "" {
		return nil, fmt.Errorf("invalid request: image_url is required")
	}
	fixPrompt := strings.TrimSpace(req.Prompt)
	if fixPrompt == "" {
		return nil, fmt.Errorf("invalid request: prompt is required")
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil || task.Result.Shein == nil {
		return nil, ErrTaskResultUnavailable
	}
	if s.studioImageGenerator == nil {
		return nil, fmt.Errorf("studio image generator is not configured")
	}

	productReq, role := buildSheinDataImageRegenerationRequest(task, req)
	sourceURL := strings.TrimSpace(productReq.SourceDesignURL)
	if sourceURL == "" {
		sourceURL = oldURL
		productReq.SourceDesignURL = sourceURL
	}
	promptText := buildStudioProductImagePrompt(productReq, role, 1, 1)
	newURL, err := s.generateOneStudioProductImage(ctx, productReq, sourceURL, promptText)
	if err != nil {
		return nil, err
	}

	replaced := replaceSheinDataImageURL(task, oldURL, newURL)
	if replaced == 0 {
		return nil, fmt.Errorf("invalid request: image_url was not found in this SHEIN task")
	}
	if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
		return nil, err
	}

	preview, err := s.GetTaskPreview(ctx, task.ID, "shein")
	if err != nil {
		return nil, err
	}
	return &RegenerateSheinDataImageResponse{
		Preview: preview,
		Image: StudioGeneratedImage{
			ID:            uuid.NewString(),
			ImageURL:      newURL,
			RevisedPrompt: fixPrompt,
			Role:          role.Key,
			RoleLabel:     role.Label,
		},
		ReplacedURL: oldURL,
	}, nil
}

func buildSheinDataImageRegenerationRequest(task *Task, req *RegenerateSheinDataImageRequest) (*StudioProductImageRequest, studioProductImageRole) {
	options := taskOptions(task)
	sds := optionsSDS(options)
	studio := optionsSheinStudio(options)
	role := inferStudioProductImageRole(req.Role, req.Label)

	productReq := &StudioProductImageRequest{
		Prompt:          firstNonEmptyString(taskText(task), "marketplace product image"),
		ProductName:     sdsProductName(sds),
		CategoryPath:    sdsCategoryPath(sds),
		StyleName:       firstNonEmptyString(sheinStudioOptionStyleName(studio), strings.TrimSpace(req.Label)),
		SourceDesignURL: firstURLString(studioSourceDesignURLs(studio), taskImageURLs(task)),
		CustomPrompt: strings.TrimSpace(`Regenerate only this problematic marketplace product image.
Use the provided problem image as a visual reference for what must be fixed, not as a final output to copy blindly.
Keep the approved artwork and product identity consistent with the existing SHEIN data images.
User issue and required fix: ` + strings.TrimSpace(req.Prompt)),
		ImagePrompts: []StudioProductImagePrompt{{
			Role:   role.Key,
			Prompt: strings.TrimSpace(req.Prompt),
		}},
		Count: 1,
	}
	productReq.ProductReferenceImageURLs = collectRegenerationReferenceURLs(req.ImageURL, studio, sds)
	return productReq, role
}

func collectRegenerationReferenceURLs(problemURL string, studio *SheinStudioOptions, sds *SDSSyncOptions) []string {
	urls := make([]string, 0, 8)
	add := func(values ...string) {
		for _, value := range values {
			trimmed := strings.TrimSpace(value)
			if trimmed == "" {
				continue
			}
			urls = append(urls, trimmed)
		}
	}
	add(problemURL)
	if studio != nil {
		add(studio.ProductImageURLs...)
		add(studio.SizeReferenceImageURLs...)
		for _, set := range studio.VariantProductImages {
			add(set.ImageURLs...)
		}
	}
	if sds != nil {
		add(sds.MockupImageURLs...)
		for _, variant := range sds.Variants {
			add(variant.MockupImageURL)
			add(variant.MockupImageURLs...)
		}
	}
	return uniqueStringsLimit(urls, 8)
}

func inferStudioProductImageRole(roleKey string, label string) studioProductImageRole {
	needle := strings.ToLower(strings.TrimSpace(firstNonEmptyString(roleKey, label)))
	for _, role := range defaultStudioProductImageRoles {
		if strings.EqualFold(strings.TrimSpace(roleKey), role.Key) {
			return role
		}
		if strings.Contains(needle, strings.ToLower(role.Key)) || strings.Contains(needle, strings.ToLower(role.Label)) {
			return role
		}
	}
	return defaultStudioProductImageRoles[0]
}

func replaceSheinDataImageURL(task *Task, oldURL string, newURL string) int {
	if task == nil || task.Result == nil {
		return 0
	}
	count := 0
	replaceString := func(value *string) {
		if value == nil {
			return
		}
		if sameImageURL(*value, oldURL) {
			*value = newURL
			count++
		}
	}
	replaceSlice := func(values []string) []string {
		for idx := range values {
			replaceString(&values[idx])
		}
		return values
	}

	if task.Request != nil && task.Request.Options != nil && task.Request.Options.SheinStudio != nil {
		studio := task.Request.Options.SheinStudio
		studio.ProductImageURLs = replaceSlice(studio.ProductImageURLs)
		studio.SourceDesignURLs = replaceSlice(studio.SourceDesignURLs)
		studio.SizeReferenceImageURLs = replaceSlice(studio.SizeReferenceImageURLs)
		for idx := range studio.VariantProductImages {
			studio.VariantProductImages[idx].ImageURLs = replaceSlice(studio.VariantProductImages[idx].ImageURLs)
		}
	}

	if pkg := task.Result.Shein; pkg != nil {
		pkg = sheinpub.NormalizePackageSemanticFields(pkg)
		count += replaceCommonImageSetURL(pkg.Images, oldURL, newURL)
		count += replacePublishImageBundleURL(pkg.ImageBundle, oldURL, newURL)
		if pkg.DraftPayload != nil {
			count += replaceSheinImageDraftURL(pkg.DraftPayload.ImageInfo, oldURL, newURL)
		}
		for idx := range pkg.SkcList {
			replaceString(&pkg.SkcList[idx].MainImageURL)
			for skuIdx := range pkg.SkcList[idx].SKUs {
				replaceString(&pkg.SkcList[idx].SKUs[skuIdx].Image)
			}
		}
		if pkg.DraftPayload != nil {
			for idx := range pkg.DraftPayload.SKCList {
				count += replaceSheinImageDraftURL(pkg.DraftPayload.SKCList[idx].ImageInfo, oldURL, newURL)
				for skuIdx := range pkg.DraftPayload.SKCList[idx].SKUList {
					replaceString(&pkg.DraftPayload.SKCList[idx].SKUList[skuIdx].MainImage)
				}
			}
		}
		count += replacePreviewProductImageURL(pkg.PreviewPayload, oldURL, newURL)
	}
	return count
}

func replaceCommonImageSetURL(images *common.ImageSet, oldURL string, newURL string) int {
	if images == nil {
		return 0
	}
	count := 0
	replace := func(value *string) {
		if sameImageURL(*value, oldURL) {
			*value = newURL
			count++
		}
	}
	replace(&images.MainImage)
	replace(&images.WhiteBgImage)
	for idx := range images.Gallery {
		replace(&images.Gallery[idx])
	}
	for idx := range images.SourceImages {
		replace(&images.SourceImages[idx])
	}
	return count
}

func replacePublishImageBundleURL(bundle *common.PublishImageBundle, oldURL string, newURL string) int {
	if bundle == nil {
		return 0
	}
	count := 0
	replaceSlot := func(slot *common.BundleSlot) {
		if slot != nil && sameImageURL(slot.URL, oldURL) {
			slot.URL = newURL
			count++
		}
	}
	replaceSlot(bundle.Main)
	for idx := range bundle.Gallery {
		replaceSlot(&bundle.Gallery[idx])
	}
	for idx := range bundle.Auxiliary {
		replaceSlot(&bundle.Auxiliary[idx])
	}
	return count
}

func replaceSheinImageDraftURL(images *sheinpub.ImageDraft, oldURL string, newURL string) int {
	if images == nil {
		return 0
	}
	count := 0
	replace := func(value *string) {
		if sameImageURL(*value, oldURL) {
			*value = newURL
			count++
		}
	}
	replace(&images.MainImage)
	replace(&images.WhiteBg)
	for idx := range images.Gallery {
		replace(&images.Gallery[idx])
	}
	for idx := range images.Source {
		replace(&images.Source[idx])
	}
	return count
}

func replacePreviewProductImageURL(product *sheinproduct.Product, oldURL string, newURL string) int {
	if product == nil {
		return 0
	}
	count := replaceProductImageInfoURL(product.ImageInfo, oldURL, newURL)
	for skcIdx := range product.SKCList {
		count += replaceProductImageInfoURL(&product.SKCList[skcIdx].ImageInfo, oldURL, newURL)
		for skuIdx := range product.SKCList[skcIdx].SKUS {
			count += replaceProductImageInfoURL(product.SKCList[skcIdx].SKUS[skuIdx].ImageInfo, oldURL, newURL)
		}
	}
	return count
}

func replaceProductImageInfoURL(images *sheinproduct.ImageInfo, oldURL string, newURL string) int {
	if images == nil {
		return 0
	}
	count := 0
	for idx := range images.ImageInfoList {
		if sameImageURL(images.ImageInfoList[idx].ImageURL, oldURL) {
			images.ImageInfoList[idx].ImageURL = newURL
			count++
		}
	}
	return count
}

func sameImageURL(left string, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	if left == right {
		return true
	}
	leftURL, leftErr := url.Parse(left)
	rightURL, rightErr := url.Parse(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	leftPath := leftURL.EscapedPath()
	rightPath := rightURL.EscapedPath()
	if leftPath == "" || rightPath == "" || leftPath != rightPath {
		return false
	}
	return leftURL.RawQuery == rightURL.RawQuery
}

func taskOptions(task *Task) *GenerateOptions {
	if task == nil || task.Request == nil {
		return nil
	}
	return task.Request.Options
}

func optionsSDS(options *GenerateOptions) *SDSSyncOptions {
	if options == nil {
		return nil
	}
	return options.SDS
}

func optionsSheinStudio(options *GenerateOptions) *SheinStudioOptions {
	if options == nil {
		return nil
	}
	return options.SheinStudio
}

func taskText(task *Task) string {
	if task == nil || task.Request == nil {
		return ""
	}
	return task.Request.Text
}

func taskImageURLs(task *Task) []string {
	if task == nil || task.Request == nil {
		return nil
	}
	return task.Request.ImageURLs
}

func sdsProductName(sds *SDSSyncOptions) string {
	if sds == nil {
		return ""
	}
	return sds.ProductName
}

func sdsCategoryPath(sds *SDSSyncOptions) []string {
	if sds == nil {
		return nil
	}
	return append([]string(nil), sds.CategoryPath...)
}

func sheinStudioOptionStyleName(studio *SheinStudioOptions) string {
	if studio == nil {
		return ""
	}
	return studio.StyleName
}

func studioSourceDesignURLs(studio *SheinStudioOptions) []string {
	if studio == nil {
		return nil
	}
	return studio.SourceDesignURLs
}

func firstURLString(groups ...[]string) string {
	for _, group := range groups {
		for _, value := range group {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func uniqueStringsLimit(values []string, limit int) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}
