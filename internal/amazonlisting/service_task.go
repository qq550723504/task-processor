package amazonlisting

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/catalog/canonical"
)

func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	normalizeGenerateRequest(req)
	if err := validateRequest(req); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}
	task := &Task{
		ID:         uuid.New().String(),
		Request:    req,
		Status:     TaskStatusPending,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		RetryCount: 0,
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, fmt.Errorf("failed to create task: %w", err)
	}
	if s.taskSubmitter != nil {
		if err := s.taskSubmitter.Submit(task.ID); err != nil {
			_ = s.repo.MarkFailed(ctx, task.ID, fmt.Sprintf("failed to submit task: %v", err))
			return nil, fmt.Errorf("failed to submit task: %w", err)
		}
	}
	return task, nil
}

func (s *service) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	result := &TaskResult{
		TaskID:    task.ID,
		Status:    task.Status,
		Result:    task.Result,
		Error:     task.Error,
		CreatedAt: task.CreatedAt,
	}
	if task.Status == TaskStatusCompleted || task.Status == TaskStatusFailed || task.Status == TaskStatusRejected {
		result.CompletedAt = &task.UpdatedAt
	}
	return result, nil
}

func (s *service) ReviewTask(ctx context.Context, taskID string, req *ReviewTaskRequest) (*TaskResult, error) {
	if req == nil {
		return nil, fmt.Errorf("review request cannot be nil")
	}
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(strings.TrimSpace(req.Action)) {
	case "approve":
		if task.Result == nil {
			return nil, fmt.Errorf("task result is empty")
		}
		task.Result.Status = string(TaskStatusCompleted)
		if task.Result.Review == nil {
			task.Result.Review = &AmazonReviewReport{}
		}
		task.Result.Review.NeedsReview = false
		task.Result.Review.Reasons = nil
		if task.Result.Compliance == nil {
			task.Result.Compliance = &AmazonComplianceReport{}
		}
		task.Result.Compliance.Ready = true
		if err := s.repo.MarkCompleted(ctx, taskID, task.Result); err != nil {
			return nil, err
		}
	case "reject":
		if err := s.repo.MarkRejected(ctx, taskID, req.Reason); err != nil {
			return nil, err
		}
	case "retry":
		if err := s.repo.PrepareRetry(ctx, taskID); err != nil {
			return nil, err
		}
		if s.taskSubmitter != nil {
			if err := s.taskSubmitter.Submit(taskID); err != nil {
				_ = s.repo.MarkFailed(ctx, taskID, fmt.Sprintf("failed to resubmit task: %v", err))
				return nil, err
			}
		}
	case "apply_edits":
		if task.Result == nil {
			return nil, fmt.Errorf("task result is empty")
		}
		if len(req.Edits) == 0 {
			return nil, fmt.Errorf("edit request is empty")
		}
		ensureCanonicalProduct(task)
		if err := applyCanonicalEdits(task.Result.CanonicalProduct, req.Edits); err != nil {
			return nil, err
		}
		syncDraftFromCanonical(task.Result, task.Result.CanonicalProduct)
		if err := applyDraftEdits(task.Result, req.Edits); err != nil {
			return nil, err
		}
		task.Result.ReviewItems = refreshCanonicalReviewItems(removeResolvedReviewItems(task.Result.ReviewItems, req.Edits), task.Result.CanonicalProduct)
		if s.exportBuilder != nil {
			task.Result.Export = s.exportBuilder.Build(task.Request, task.Result)
		}
		report := s.validator.Validate(task.Request, task.Result)
		task.Result.Compliance = &AmazonComplianceReport{
			Ready:          report.Ready,
			BlockingIssues: append([]string(nil), report.BlockingIssues...),
			Warnings:       append([]string(nil), report.Warnings...),
		}
		task.Result.Review = &AmazonReviewReport{
			NeedsReview: report.NeedsReview,
			Reasons:     append([]string(nil), report.ReviewReasons...),
		}
		task.Result.Status = string(TaskStatusNeedsReview)
		if len(report.BlockingIssues) == 0 && !report.NeedsReview {
			task.Result.Status = string(TaskStatusCompleted)
			if err := s.repo.MarkCompleted(ctx, taskID, task.Result); err != nil {
				return nil, err
			}
		} else {
			reason := strings.Join(report.ReviewReasons, "; ")
			if len(report.BlockingIssues) > 0 {
				reason = strings.Join(report.BlockingIssues, "; ")
			}
			if err := s.repo.MarkNeedsReview(ctx, taskID, task.Result, reason); err != nil {
				return nil, err
			}
		}
	default:
		return nil, fmt.Errorf("unsupported review action: %s", req.Action)
	}

	return s.GetTaskResult(ctx, taskID)
}

func applyDraftEdits(draft *AmazonListingDraft, edits []DraftFieldEdit) error {
	if draft == nil {
		return fmt.Errorf("draft is nil")
	}
	for _, edit := range edits {
		field := strings.TrimSpace(edit.Field)
		if index, subfield, ok := parseIndexedField(field, "variants"); ok {
			if err := applyVariantDraftEdit(draft, index, subfield, edit); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(field, "attributes.") {
			key := strings.TrimSpace(strings.TrimPrefix(field, "attributes."))
			if key == "" {
				return fmt.Errorf("unsupported edit field: %s", field)
			}
			if draft.Attributes == nil {
				draft.Attributes = map[string]string{}
			}
			draft.Attributes[key] = strings.TrimSpace(edit.StringValue)
			continue
		}
		if strings.HasPrefix(field, "specifications.technical.") {
			key := strings.TrimSpace(strings.TrimPrefix(field, "specifications.technical."))
			if key == "" {
				return fmt.Errorf("unsupported edit field: %s", field)
			}
			if draft.Attributes == nil {
				draft.Attributes = map[string]string{}
			}
			draft.Attributes[key] = strings.TrimSpace(edit.StringValue)
			continue
		}
		switch field {
		case "title":
			draft.Title = strings.TrimSpace(edit.StringValue)
		case "brand":
			draft.Brand = strings.TrimSpace(edit.StringValue)
			if draft.Attributes == nil {
				draft.Attributes = map[string]string{}
			}
			if draft.Brand != "" {
				draft.Attributes["brand"] = draft.Brand
			}
		case "description":
			draft.Description = strings.TrimSpace(edit.StringValue)
		case "category_path":
			draft.CategoryPath = trimStringList(edit.StringList)
			if len(draft.CategoryPath) > 0 {
				draft.ProductType = draft.CategoryPath[len(draft.CategoryPath)-1]
			}
		case "bullet_points":
			draft.BulletPoints = trimStringList(edit.StringList)
		case "search_terms":
			draft.SearchTerms = trimStringList(edit.StringList)
		case "images.main_image":
			ensureDraftImages(draft)
			draft.Images.MainImage = strings.TrimSpace(edit.StringValue)
		case "images.white_bg_image":
			ensureDraftImages(draft)
			draft.Images.WhiteBgImage = strings.TrimSpace(edit.StringValue)
		case "images.gallery":
			ensureDraftImages(draft)
			draft.Images.GalleryImages = trimStringList(edit.StringList)
		case "pricing.currency":
			ensureDraftPricing(draft)
			draft.Pricing.Currency = strings.TrimSpace(edit.StringValue)
		case "pricing.suggested_price":
			if edit.NumberValue == nil {
				return fmt.Errorf("pricing.suggested_price requires number_value")
			}
			ensureDraftPricing(draft)
			draft.Pricing.SuggestedPrice = *edit.NumberValue
		case "pricing.min_price":
			if edit.NumberValue == nil {
				return fmt.Errorf("pricing.min_price requires number_value")
			}
			ensureDraftPricing(draft)
			draft.Pricing.MinPrice = *edit.NumberValue
		case "pricing.source_cost":
			if edit.NumberValue == nil {
				return fmt.Errorf("pricing.source_cost requires number_value")
			}
			ensureDraftPricing(draft)
			draft.Pricing.SourceCost = *edit.NumberValue
		case "dimensions.length":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.length requires number_value")
			}
			ensureDraftDimensions(draft)
			draft.Dimensions.Length = *edit.NumberValue
		case "dimensions.width":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.width requires number_value")
			}
			ensureDraftDimensions(draft)
			draft.Dimensions.Width = *edit.NumberValue
		case "dimensions.height":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.height requires number_value")
			}
			ensureDraftDimensions(draft)
			draft.Dimensions.Height = *edit.NumberValue
		case "dimensions.unit":
			ensureDraftDimensions(draft)
			draft.Dimensions.Unit = strings.TrimSpace(edit.StringValue)
		case "weight.value":
			if edit.NumberValue == nil {
				return fmt.Errorf("weight.value requires number_value")
			}
			ensureDraftWeight(draft)
			draft.Weight.Value = *edit.NumberValue
		case "weight.unit":
			ensureDraftWeight(draft)
			draft.Weight.Unit = strings.TrimSpace(edit.StringValue)
		case "package.quantity":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.quantity requires number_value")
			}
			ensureDraftPackage(draft)
			draft.Package.Quantity = int(*edit.NumberValue)
		case "package.dimensions.length":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.length requires number_value")
			}
			ensureDraftPackageDimensions(draft)
			draft.Package.Dimensions.Length = *edit.NumberValue
		case "package.dimensions.width":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.width requires number_value")
			}
			ensureDraftPackageDimensions(draft)
			draft.Package.Dimensions.Width = *edit.NumberValue
		case "package.dimensions.height":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.height requires number_value")
			}
			ensureDraftPackageDimensions(draft)
			draft.Package.Dimensions.Height = *edit.NumberValue
		case "package.dimensions.unit":
			ensureDraftPackageDimensions(draft)
			draft.Package.Dimensions.Unit = strings.TrimSpace(edit.StringValue)
		case "package.weight.value":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.weight.value requires number_value")
			}
			ensureDraftPackageWeight(draft)
			draft.Package.Weight.Value = *edit.NumberValue
		case "package.weight.unit":
			ensureDraftPackageWeight(draft)
			draft.Package.Weight.Unit = strings.TrimSpace(edit.StringValue)
		default:
			return fmt.Errorf("unsupported edit field: %s", field)
		}
	}
	return nil
}

func ensureCanonicalProduct(task *Task) {
	if task == nil || task.Result == nil || task.Result.CanonicalProduct != nil {
		return
	}
	task.Result.CanonicalProduct = canonicalProductFromDraft(task.Result)
}

func applyCanonicalEdits(product *canonical.Product, edits []DraftFieldEdit) error {
	if product == nil {
		return nil
	}
	if product.FieldTraces == nil {
		product.FieldTraces = map[string]canonical.FieldTrace{}
	}
	if product.Attributes == nil {
		product.Attributes = map[string]canonical.Attribute{}
	}
	for _, edit := range edits {
		field := strings.TrimSpace(edit.Field)
		if index, subfield, ok := parseIndexedField(field, "variants"); ok {
			if err := applyVariantCanonicalEdit(product, index, subfield, edit); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(field, "attributes.") {
			key := strings.TrimSpace(strings.TrimPrefix(field, "attributes."))
			if key == "" {
				return fmt.Errorf("unsupported edit field: %s", field)
			}
			product.Attributes[key] = canonical.Attribute{
				Value: strings.TrimSpace(edit.StringValue),
				Trace: manualFieldTrace(),
			}
			continue
		}
		if strings.HasPrefix(field, "specifications.technical.") {
			key := strings.TrimSpace(strings.TrimPrefix(field, "specifications.technical."))
			if key == "" {
				return fmt.Errorf("unsupported edit field: %s", field)
			}
			ensureCanonicalSpecifications(product)
			if product.Specifications.Technical == nil {
				product.Specifications.Technical = map[string]string{}
			}
			product.Specifications.Technical[key] = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
			continue
		}
		switch field {
		case "title":
			product.Title = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["title"] = manualFieldTrace()
		case "brand":
			product.Brand = strings.TrimSpace(edit.StringValue)
			if product.Brand != "" {
				product.Attributes["brand"] = canonical.Attribute{
					Value: product.Brand,
					Trace: manualFieldTrace(),
				}
			}
			product.FieldTraces["brand"] = manualFieldTrace()
		case "description":
			product.Description = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["description"] = manualFieldTrace()
		case "category_path":
			product.CategoryPath = trimStringList(edit.StringList)
			product.FieldTraces["category_path"] = manualFieldTrace()
		case "bullet_points":
			product.SellingPoints = trimStringList(edit.StringList)
			product.FieldTraces["selling_points"] = manualFieldTrace()
		case "search_terms":
			product.SEOKeywords = trimStringList(edit.StringList)
			product.FieldTraces["seo_keywords"] = manualFieldTrace()
		case "dimensions.length":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.length requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalDimensions(product.Specifications)
			product.Specifications.Dimensions.Length = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "dimensions.width":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.width requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalDimensions(product.Specifications)
			product.Specifications.Dimensions.Width = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "dimensions.height":
			if edit.NumberValue == nil {
				return fmt.Errorf("dimensions.height requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalDimensions(product.Specifications)
			product.Specifications.Dimensions.Height = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "dimensions.unit":
			ensureCanonicalSpecifications(product)
			ensureCanonicalDimensions(product.Specifications)
			product.Specifications.Dimensions.Unit = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "weight.value":
			if edit.NumberValue == nil {
				return fmt.Errorf("weight.value requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalWeight(product.Specifications)
			product.Specifications.Weight.Value = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "weight.unit":
			ensureCanonicalSpecifications(product)
			ensureCanonicalWeight(product.Specifications)
			product.Specifications.Weight.Unit = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.quantity":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.quantity requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackage(product.Specifications)
			product.Specifications.Package.Quantity = int(*edit.NumberValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.dimensions.length":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.length requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageDimensions(product.Specifications)
			product.Specifications.Package.Dimensions.Length = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.dimensions.width":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.width requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageDimensions(product.Specifications)
			product.Specifications.Package.Dimensions.Width = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.dimensions.height":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.dimensions.height requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageDimensions(product.Specifications)
			product.Specifications.Package.Dimensions.Height = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.dimensions.unit":
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageDimensions(product.Specifications)
			product.Specifications.Package.Dimensions.Unit = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.weight.value":
			if edit.NumberValue == nil {
				return fmt.Errorf("package.weight.value requires number_value")
			}
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageWeight(product.Specifications)
			product.Specifications.Package.Weight.Value = *edit.NumberValue
			product.FieldTraces["specifications"] = manualFieldTrace()
		case "package.weight.unit":
			ensureCanonicalSpecifications(product)
			ensureCanonicalPackageWeight(product.Specifications)
			product.Specifications.Package.Weight.Unit = strings.TrimSpace(edit.StringValue)
			product.FieldTraces["specifications"] = manualFieldTrace()
		}
	}
	product.NeedsReview = canonicalProductNeedsReview(product)
	return nil
}

func syncDraftFromCanonical(draft *AmazonListingDraft, product *canonical.Product) {
	if draft == nil || product == nil {
		return
	}
	draft.CanonicalProduct = product
	draft.Title = product.Title
	draft.Brand = product.Brand
	draft.Description = product.Description
	draft.CategoryPath = append([]string(nil), product.CategoryPath...)
	draft.BulletPoints = append([]string(nil), product.SellingPoints...)
	draft.SearchTerms = append([]string(nil), product.SEOKeywords...)
	if draft.Attributes == nil {
		draft.Attributes = map[string]string{}
	}
	for key := range draft.Attributes {
		if key == "brand" {
			continue
		}
		delete(draft.Attributes, key)
	}
	for key, attr := range product.Attributes {
		draft.Attributes[key] = attr.Value
	}
	if draft.Brand != "" {
		draft.Attributes["brand"] = draft.Brand
	}
	if len(product.CategoryPath) > 0 {
		draft.ProductType = product.CategoryPath[len(product.CategoryPath)-1]
	} else if product.Title != "" {
		draft.ProductType = product.Title
	}
	if product.Specifications != nil {
		if product.Specifications.Dimensions != nil {
			draft.Dimensions = &AmazonDimensions{
				Length: product.Specifications.Dimensions.Length,
				Width:  product.Specifications.Dimensions.Width,
				Height: product.Specifications.Dimensions.Height,
				Unit:   product.Specifications.Dimensions.Unit,
			}
		}
		if product.Specifications.Weight != nil {
			draft.Weight = &AmazonWeight{
				Value: product.Specifications.Weight.Value,
				Unit:  product.Specifications.Weight.Unit,
			}
		}
		if product.Specifications.Package != nil {
			draft.Package = &AmazonPackageInfo{
				Quantity: product.Specifications.Package.Quantity,
			}
			if product.Specifications.Package.Dimensions != nil {
				draft.Package.Dimensions = &AmazonDimensions{
					Length: product.Specifications.Package.Dimensions.Length,
					Width:  product.Specifications.Package.Dimensions.Width,
					Height: product.Specifications.Package.Dimensions.Height,
					Unit:   product.Specifications.Package.Dimensions.Unit,
				}
			}
			if product.Specifications.Package.Weight != nil {
				draft.Package.Weight = &AmazonWeight{
					Value: product.Specifications.Package.Weight.Value,
					Unit:  product.Specifications.Package.Weight.Unit,
				}
			}
		}
	}
	if len(product.Variants) > 0 {
		draft.Variants = draft.Variants[:0]
		for _, variant := range product.Variants {
			converted := AmazonVariantDraft{
				SKU:       variant.SKU,
				Inventory: variant.Stock,
				Barcode:   variant.Barcode,
				IsDefault: variant.IsDefault,
			}
			if len(variant.Attributes) > 0 {
				converted.Attributes = make(map[string]string, len(variant.Attributes))
				for key, attr := range variant.Attributes {
					converted.Attributes[key] = attr.Value
				}
			}
			if variant.Price != nil {
				converted.Price = &AmazonMoney{
					Currency: variant.Price.Currency,
					Amount:   variant.Price.Amount,
				}
				if variant.Price.CostPrice > 0 {
					converted.CostPrice = &AmazonMoney{
						Currency: variant.Price.Currency,
						Amount:   variant.Price.CostPrice,
					}
				}
			}
			if len(variant.Images) > 0 {
				converted.MainImage = variant.Images[0].URL
			}
			draft.Variants = append(draft.Variants, converted)
		}
	}
}

func canonicalProductFromDraft(draft *AmazonListingDraft) *canonical.Product {
	if draft == nil {
		return nil
	}
	product := &canonical.Product{
		Title:         draft.Title,
		Brand:         draft.Brand,
		CategoryPath:  append([]string(nil), draft.CategoryPath...),
		Description:   draft.Description,
		SellingPoints: append([]string(nil), draft.BulletPoints...),
		SEOKeywords:   append([]string(nil), draft.SearchTerms...),
		Attributes:    map[string]canonical.Attribute{},
		FieldTraces:   map[string]canonical.FieldTrace{},
	}
	if draft.Dimensions != nil || draft.Weight != nil {
		product.Specifications = &canonical.ProductSpecs{}
		if draft.Dimensions != nil {
			product.Specifications.Dimensions = &canonical.Dimensions{
				Length: draft.Dimensions.Length,
				Width:  draft.Dimensions.Width,
				Height: draft.Dimensions.Height,
				Unit:   draft.Dimensions.Unit,
			}
		}
		if draft.Weight != nil {
			product.Specifications.Weight = &canonical.Weight{
				Value: draft.Weight.Value,
				Unit:  draft.Weight.Unit,
			}
		}
	}
	if draft.Package != nil {
		ensureCanonicalSpecifications(product)
		product.Specifications.Package = &canonical.PackageInfo{
			Quantity: draft.Package.Quantity,
		}
		if draft.Package.Dimensions != nil {
			product.Specifications.Package.Dimensions = &canonical.Dimensions{
				Length: draft.Package.Dimensions.Length,
				Width:  draft.Package.Dimensions.Width,
				Height: draft.Package.Dimensions.Height,
				Unit:   draft.Package.Dimensions.Unit,
			}
		}
		if draft.Package.Weight != nil {
			product.Specifications.Package.Weight = &canonical.Weight{
				Value: draft.Package.Weight.Value,
				Unit:  draft.Package.Weight.Unit,
			}
		}
	}
	for key, value := range draft.Attributes {
		product.Attributes[key] = canonical.Attribute{
			Value: value,
			Trace: manualFieldTrace(),
		}
	}
	if len(draft.Variants) > 0 {
		product.Variants = make([]canonical.Variant, 0, len(draft.Variants))
		for _, variant := range draft.Variants {
			converted := canonical.Variant{
				SKU:        variant.SKU,
				Attributes: map[string]canonical.Attribute{},
				Stock:      variant.Inventory,
				Barcode:    variant.Barcode,
				IsDefault:  variant.IsDefault,
				Trace:      manualFieldTrace(),
			}
			for key, value := range variant.Attributes {
				converted.Attributes[key] = canonical.Attribute{
					Value: value,
					Trace: manualFieldTrace(),
				}
			}
			if variant.Price != nil {
				converted.Price = &canonical.PriceInfo{
					Currency: variant.Price.Currency,
					Amount:   variant.Price.Amount,
				}
				if variant.CostPrice != nil {
					converted.Price.CostPrice = variant.CostPrice.Amount
				}
			}
			if strings.TrimSpace(variant.MainImage) != "" {
				converted.Images = []canonical.Image{{
					URL:   strings.TrimSpace(variant.MainImage),
					Role:  "variant",
					Trace: manualFieldTrace(),
				}}
			}
			product.Variants = append(product.Variants, converted)
		}
	}
	product.FieldTraces["title"] = manualFieldTrace()
	product.FieldTraces["brand"] = manualFieldTrace()
	product.FieldTraces["category_path"] = manualFieldTrace()
	product.FieldTraces["description"] = manualFieldTrace()
	product.FieldTraces["selling_points"] = manualFieldTrace()
	product.FieldTraces["seo_keywords"] = manualFieldTrace()
	if product.Specifications != nil {
		product.FieldTraces["specifications"] = manualFieldTrace()
	}
	product.NeedsReview = canonicalProductNeedsReview(product)
	return product
}

func refreshCanonicalReviewItems(items []AmazonReviewItem, product *canonical.Product) []AmazonReviewItem {
	if product == nil {
		return items
	}
	filtered := make([]AmazonReviewItem, 0, len(items))
	for _, item := range items {
		switch item.Field {
		case "title", "brand", "category_path", "description", "selling_points", "seo_keywords", "attributes", "specifications", "product", "dimensions.unit", "weight.unit":
			continue
		default:
			if strings.HasPrefix(item.Field, "attributes.") {
				continue
			}
			if strings.HasPrefix(item.Field, "specifications.technical.") || strings.HasPrefix(item.Field, "dimensions.") || strings.HasPrefix(item.Field, "weight.") {
				continue
			}
			if strings.HasPrefix(item.Field, "package.") || strings.HasPrefix(item.Field, "variants[") {
				continue
			}
			filtered = append(filtered, item)
		}
	}
	return dedupeReviewItems(append(filtered, buildReviewItemsFromCanonical(product)...))
}

func manualFieldTrace() canonical.FieldTrace {
	return canonical.FieldTrace{
		Sources: []canonical.Source{
			{Type: canonical.SourceDerived, Detail: "manual_review_edit"},
		},
		Confidence:  1,
		IsInferred:  false,
		NeedsReview: false,
	}
}

func canonicalProductNeedsReview(product *canonical.Product) bool {
	if product == nil {
		return true
	}
	if strings.TrimSpace(product.Title) == "" || strings.TrimSpace(product.Description) == "" {
		return true
	}
	if len(product.CategoryPath) == 0 {
		return true
	}
	for _, trace := range product.FieldTraces {
		if trace.NeedsReview {
			return true
		}
	}
	return false
}

func removeResolvedReviewItems(items []AmazonReviewItem, edits []DraftFieldEdit) []AmazonReviewItem {
	if len(items) == 0 || len(edits) == 0 {
		return items
	}
	edited := make(map[string]struct{}, len(edits))
	for _, edit := range edits {
		for _, field := range relatedReviewFields(strings.TrimSpace(edit.Field)) {
			edited[field] = struct{}{}
		}
	}
	filtered := make([]AmazonReviewItem, 0, len(items))
	for _, item := range items {
		if _, ok := edited[item.Field]; ok {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered
}

func trimStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		result = append(result, value)
	}
	return result
}

func ensureDraftImages(draft *AmazonListingDraft) {
	if draft.Images == nil {
		draft.Images = &AmazonImageBundle{}
	}
}

func ensureDraftPricing(draft *AmazonListingDraft) {
	if draft.Pricing == nil {
		draft.Pricing = &AmazonPricingDraft{}
	}
}

func ensureDraftDimensions(draft *AmazonListingDraft) {
	if draft.Dimensions == nil {
		draft.Dimensions = &AmazonDimensions{}
	}
}

func ensureDraftWeight(draft *AmazonListingDraft) {
	if draft.Weight == nil {
		draft.Weight = &AmazonWeight{}
	}
}

func ensureDraftPackage(draft *AmazonListingDraft) {
	if draft.Package == nil {
		draft.Package = &AmazonPackageInfo{}
	}
}

func ensureDraftPackageDimensions(draft *AmazonListingDraft) {
	ensureDraftPackage(draft)
	if draft.Package.Dimensions == nil {
		draft.Package.Dimensions = &AmazonDimensions{}
	}
}

func ensureDraftPackageWeight(draft *AmazonListingDraft) {
	ensureDraftPackage(draft)
	if draft.Package.Weight == nil {
		draft.Package.Weight = &AmazonWeight{}
	}
}

func ensureCanonicalSpecifications(product *canonical.Product) {
	if product.Specifications == nil {
		product.Specifications = &canonical.ProductSpecs{}
	}
}

func ensureCanonicalDimensions(specs *canonical.ProductSpecs) {
	if specs.Dimensions == nil {
		specs.Dimensions = &canonical.Dimensions{}
	}
}

func ensureCanonicalWeight(specs *canonical.ProductSpecs) {
	if specs.Weight == nil {
		specs.Weight = &canonical.Weight{}
	}
}

func ensureCanonicalPackage(specs *canonical.ProductSpecs) {
	if specs.Package == nil {
		specs.Package = &canonical.PackageInfo{}
	}
}

func ensureCanonicalPackageDimensions(specs *canonical.ProductSpecs) {
	ensureCanonicalPackage(specs)
	if specs.Package.Dimensions == nil {
		specs.Package.Dimensions = &canonical.Dimensions{}
	}
}

func ensureCanonicalPackageWeight(specs *canonical.ProductSpecs) {
	ensureCanonicalPackage(specs)
	if specs.Package.Weight == nil {
		specs.Package.Weight = &canonical.Weight{}
	}
}

func parseIndexedField(field string, collection string) (int, string, bool) {
	prefix := collection + "["
	if !strings.HasPrefix(field, prefix) {
		return 0, "", false
	}
	rest := strings.TrimPrefix(field, prefix)
	end := strings.Index(rest, "]")
	if end <= 0 || len(rest) <= end+1 || rest[end+1] != '.' {
		return 0, "", false
	}
	index, err := strconv.Atoi(rest[:end])
	if err != nil || index < 0 {
		return 0, "", false
	}
	return index, rest[end+2:], true
}

func applyVariantDraftEdit(draft *AmazonListingDraft, index int, subfield string, edit DraftFieldEdit) error {
	ensureDraftVariant(draft, index)
	variant := &draft.Variants[index]
	switch {
	case subfield == "sku":
		variant.SKU = strings.TrimSpace(edit.StringValue)
	case subfield == "barcode":
		variant.Barcode = strings.TrimSpace(edit.StringValue)
	case subfield == "inventory":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].inventory requires number_value", index)
		}
		variant.Inventory = int(*edit.NumberValue)
	case subfield == "is_default":
		value, err := parseBooleanEdit(edit, fmt.Sprintf("variants[%d].is_default", index))
		if err != nil {
			return err
		}
		variant.IsDefault = value
	case subfield == "main_image":
		variant.MainImage = strings.TrimSpace(edit.StringValue)
	case subfield == "price.amount":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].price.amount requires number_value", index)
		}
		ensureDraftVariantPrice(variant)
		variant.Price.Amount = *edit.NumberValue
	case subfield == "price.currency":
		ensureDraftVariantPrice(variant)
		variant.Price.Currency = strings.TrimSpace(edit.StringValue)
	case subfield == "cost_price.amount":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].cost_price.amount requires number_value", index)
		}
		ensureDraftVariantCostPrice(variant)
		variant.CostPrice.Amount = *edit.NumberValue
	case subfield == "cost_price.currency":
		ensureDraftVariantCostPrice(variant)
		variant.CostPrice.Currency = strings.TrimSpace(edit.StringValue)
	case strings.HasPrefix(subfield, "attributes."):
		key := strings.TrimSpace(strings.TrimPrefix(subfield, "attributes."))
		if key == "" {
			return fmt.Errorf("unsupported edit field: variants[%d].%s", index, subfield)
		}
		if variant.Attributes == nil {
			variant.Attributes = map[string]string{}
		}
		variant.Attributes[key] = strings.TrimSpace(edit.StringValue)
	default:
		return fmt.Errorf("unsupported edit field: variants[%d].%s", index, subfield)
	}
	return nil
}

func applyVariantCanonicalEdit(product *canonical.Product, index int, subfield string, edit DraftFieldEdit) error {
	for len(product.Variants) <= index {
		product.Variants = append(product.Variants, canonical.Variant{
			Attributes: map[string]canonical.Attribute{},
			Trace:      manualFieldTrace(),
		})
	}
	variant := &product.Variants[index]
	if variant.Attributes == nil {
		variant.Attributes = map[string]canonical.Attribute{}
	}
	variant.Trace = manualFieldTrace()
	switch {
	case subfield == "sku":
		variant.SKU = strings.TrimSpace(edit.StringValue)
	case subfield == "barcode":
		variant.Barcode = strings.TrimSpace(edit.StringValue)
	case subfield == "inventory":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].inventory requires number_value", index)
		}
		variant.Stock = int(*edit.NumberValue)
	case subfield == "is_default":
		value, err := parseBooleanEdit(edit, fmt.Sprintf("variants[%d].is_default", index))
		if err != nil {
			return err
		}
		variant.IsDefault = value
	case subfield == "main_image":
		url := strings.TrimSpace(edit.StringValue)
		if url == "" {
			variant.Images = nil
		} else if len(variant.Images) == 0 {
			variant.Images = []canonical.Image{{URL: url, Role: "variant", Trace: manualFieldTrace()}}
		} else {
			variant.Images[0] = canonical.Image{URL: url, Role: "variant", Trace: manualFieldTrace()}
		}
	case subfield == "price.amount":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].price.amount requires number_value", index)
		}
		ensureCanonicalVariantPrice(variant)
		variant.Price.Amount = *edit.NumberValue
	case subfield == "price.currency":
		ensureCanonicalVariantPrice(variant)
		variant.Price.Currency = strings.TrimSpace(edit.StringValue)
	case subfield == "cost_price.amount":
		if edit.NumberValue == nil {
			return fmt.Errorf("variants[%d].cost_price.amount requires number_value", index)
		}
		ensureCanonicalVariantPrice(variant)
		variant.Price.CostPrice = *edit.NumberValue
	case subfield == "cost_price.currency":
		ensureCanonicalVariantPrice(variant)
		variant.Price.Currency = strings.TrimSpace(edit.StringValue)
	case strings.HasPrefix(subfield, "attributes."):
		key := strings.TrimSpace(strings.TrimPrefix(subfield, "attributes."))
		if key == "" {
			return fmt.Errorf("unsupported edit field: variants[%d].%s", index, subfield)
		}
		variant.Attributes[key] = canonical.Attribute{
			Value: strings.TrimSpace(edit.StringValue),
			Trace: manualFieldTrace(),
		}
	default:
		return fmt.Errorf("unsupported edit field: variants[%d].%s", index, subfield)
	}
	return nil
}

func ensureDraftVariant(draft *AmazonListingDraft, index int) {
	for len(draft.Variants) <= index {
		draft.Variants = append(draft.Variants, AmazonVariantDraft{})
	}
}

func ensureDraftVariantPrice(variant *AmazonVariantDraft) {
	if variant.Price == nil {
		variant.Price = &AmazonMoney{}
	}
}

func ensureDraftVariantCostPrice(variant *AmazonVariantDraft) {
	if variant.CostPrice == nil {
		variant.CostPrice = &AmazonMoney{}
	}
}

func ensureCanonicalVariantPrice(variant *canonical.Variant) {
	if variant.Price == nil {
		variant.Price = &canonical.PriceInfo{}
	}
}

func parseBooleanEdit(edit DraftFieldEdit, field string) (bool, error) {
	value := strings.TrimSpace(strings.ToLower(edit.StringValue))
	switch value {
	case "true", "1", "yes":
		return true, nil
	case "false", "0", "no":
		return false, nil
	default:
		return false, fmt.Errorf("%s requires string_value true/false", field)
	}
}

func relatedReviewFields(field string) []string {
	fields := []string{field}
	if _, subfield, ok := parseIndexedField(field, "variants"); ok {
		switch subfield {
		case "sku":
			fields = append(fields, "variants.sku")
		case "is_default":
			fields = append(fields, "variants.default")
		case "price.amount", "price.currency", "cost_price.amount", "cost_price.currency":
			fields = append(fields, "variants.price")
		}
	}
	return fields
}

func validateRequest(req *GenerateRequest) error {
	hasImages := len(req.ImageURLs) > 0
	hasText := strings.TrimSpace(req.Text) != ""
	hasProductURL := strings.TrimSpace(req.ProductURL) != ""

	if req.Marketplace == "" {
		req.Marketplace = "amazon"
	}
	if req.Marketplace != "amazon" {
		return fmt.Errorf("only amazon marketplace is supported currently")
	}
	if !hasImages && !hasText && !hasProductURL {
		return fmt.Errorf("at least one input type is required")
	}
	if hasProductURL {
		return nil
	}
	if hasImages && hasText {
		return nil
	}
	return fmt.Errorf("provide product_url, or provide both image_urls and text")
}
