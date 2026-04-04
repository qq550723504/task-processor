package amazonlisting

import (
	"context"
	"strings"
	"testing"
	"time"

	"task-processor/internal/productenrich"
)

func TestCreateGenerateTask_RejectsTextOnlyRequest(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		Text:        "only text is not enough",
	})
	if err == nil {
		t.Fatal("expected text-only request to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid request: provide product_url, or provide both image_urls and text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateGenerateTask_RejectsImageOnlyRequest(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	_, err = svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		ImageURLs:   []string{"https://example.com/a.jpg"},
	})
	if err == nil {
		t.Fatal("expected image-only request to be rejected")
	}
	if !strings.Contains(err.Error(), "invalid request: provide product_url, or provide both image_urls and text") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCreateGenerateTask_AllowsProductURLOnlyRequest(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		ProductURL:  "https://example.com/product",
	})
	if err != nil {
		t.Fatalf("expected product-url-only request to pass, got %v", err)
	}
	if task == nil || task.ID == "" {
		t.Fatal("expected task to be created")
	}
}

func TestCreateGenerateTask_AllowsImageAndTextRequest(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task, err := svc.CreateGenerateTask(context.Background(), &GenerateRequest{
		Marketplace: "amazon",
		ImageURLs:   []string{"https://example.com/a.jpg"},
		Text:        "supplemental product description",
	})
	if err != nil {
		t.Fatalf("expected image+text request to pass, got %v", err)
	}
	if task == nil || task.ID == "" {
		t.Fatal("expected task to be created")
	}
}

func TestReviewTask_ApplyEditsClearsMatchingReviewItems(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task := &Task{
		ID:        "task-edit-review",
		Status:    TaskStatusNeedsReview,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			ProductURL:  "https://detail.1688.com/offer/123.html",
		},
		Result: &AmazonListingDraft{
			TaskID:       "task-edit-review",
			Status:       string(TaskStatusNeedsReview),
			Marketplace:  "amazon",
			Country:      "US",
			Title:        "Short title",
			Description:  "This is a sufficiently long description for review edits test.",
			ProductType:  "Kitchen",
			CategoryPath: []string{"Home", "Kitchen"},
			Images:       &AmazonImageBundle{MainImage: "https://example.com/main.jpg", WhiteBgImage: "https://example.com/white.jpg", GalleryImages: []string{"https://example.com/gallery.jpg"}},
			Pricing:      &AmazonPricingDraft{Currency: "USD"},
			Variants:     []AmazonVariantDraft{{SKU: "SKU-1", IsDefault: true}},
			CanonicalProduct: &productenrich.CanonicalProduct{
				Title:         "Short title",
				Description:   "This is a sufficiently long description for review edits test.",
				CategoryPath:  []string{"Home", "Kitchen"},
				Attributes:    map[string]productenrich.CanonicalAttribute{"brand": {Value: "", Trace: productenrich.FieldTrace{NeedsReview: true}}},
				FieldTraces:   map[string]productenrich.FieldTrace{"brand": {NeedsReview: true}},
				NeedsReview:   true,
			},
			ReviewItems: []AmazonReviewItem{
				{Field: "brand", Action: OperatorActionFillBrand, Reason: "missing brand", NeedsHuman: true},
				{Field: "title", Action: OperatorActionEditTitle, Reason: "title may be too short", NeedsHuman: true},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := svc.ReviewTask(context.Background(), "task-edit-review", &ReviewTaskRequest{
		Action: "apply_edits",
		Edits: []DraftFieldEdit{
			{Field: "brand", StringValue: "Acme"},
		},
	})
	if err != nil {
		t.Fatalf("ReviewTask(apply_edits): %v", err)
	}
	if result.Result == nil {
		t.Fatal("expected result")
	}
	if result.Result.Brand != "Acme" {
		t.Fatalf("brand = %q, want Acme", result.Result.Brand)
	}
	if result.Result.CanonicalProduct == nil || result.Result.CanonicalProduct.Brand != "Acme" {
		t.Fatalf("canonical brand = %+v, want Acme", result.Result.CanonicalProduct)
	}
	for _, item := range result.Result.ReviewItems {
		if item.Field == "brand" {
			t.Fatalf("expected brand review item to be removed, got %+v", item)
		}
	}
}

func TestReviewTask_ApplyEditsCanCompleteTask(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task := &Task{
		ID:        "task-edit-complete",
		Status:    TaskStatusNeedsReview,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			ProductURL:  "https://detail.1688.com/offer/123.html",
		},
		Result: &AmazonListingDraft{
			TaskID:       "task-edit-complete",
			Status:       string(TaskStatusNeedsReview),
			Marketplace:  "amazon",
			Country:      "US",
			Title:        "",
			Brand:        "",
			Description:  "",
			ProductType:  "Kitchen",
			CategoryPath: nil,
			Images:       &AmazonImageBundle{},
			Pricing:      &AmazonPricingDraft{},
			Variants:     []AmazonVariantDraft{{SKU: "SKU-1", IsDefault: true}},
			CanonicalProduct: &productenrich.CanonicalProduct{
				Attributes:  map[string]productenrich.CanonicalAttribute{},
				FieldTraces: map[string]productenrich.FieldTrace{},
				NeedsReview: true,
			},
			ReviewItems: []AmazonReviewItem{
				{Field: "title", Action: OperatorActionEditTitle, Reason: "title is required", NeedsHuman: true},
				{Field: "brand", Action: OperatorActionFillBrand, Reason: "missing brand", NeedsHuman: true},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	price := 19.99
	result, err := svc.ReviewTask(context.Background(), "task-edit-complete", &ReviewTaskRequest{
		Action: "apply_edits",
		Edits: []DraftFieldEdit{
			{Field: "title", StringValue: "High Quality Ceramic Coffee Mug for Home Kitchen Use"},
			{Field: "brand", StringValue: "Acme"},
			{Field: "description", StringValue: "This ceramic mug is suitable for coffee, tea, and daily kitchen use with durable material and comfortable handling."},
			{Field: "category_path", StringList: []string{"Home & Kitchen", "Drinkware"}},
			{Field: "bullet_points", StringList: []string{"Durable ceramic material", "Suitable for coffee and tea", "Comfortable daily-use mug"}},
			{Field: "images.main_image", StringValue: "https://example.com/main.jpg"},
			{Field: "images.white_bg_image", StringValue: "https://example.com/white.jpg"},
			{Field: "images.gallery", StringList: []string{"https://example.com/gallery1.jpg"}},
			{Field: "pricing.currency", StringValue: "USD"},
			{Field: "pricing.suggested_price", NumberValue: &price},
		},
	})
	if err != nil {
		t.Fatalf("ReviewTask(apply_edits): %v", err)
	}
	if result.Status != TaskStatusCompleted {
		t.Fatalf("status = %s, want completed", result.Status)
	}
	if result.Result == nil || result.Result.Review == nil || result.Result.Review.NeedsReview {
		t.Fatalf("expected review to be cleared, got %+v", result.Result)
	}
	if result.Result.CanonicalProduct == nil || result.Result.CanonicalProduct.Title == "" || result.Result.CanonicalProduct.Brand != "Acme" {
		t.Fatalf("expected canonical product to be updated, got %+v", result.Result.CanonicalProduct)
	}
}

func TestReviewTask_ApplyAttributeEditUpdatesCanonicalAndClearsAttributeReviewItem(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	task := &Task{
		ID:        "task-edit-attribute",
		Status:    TaskStatusNeedsReview,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			ProductURL:  "https://detail.1688.com/offer/123.html",
		},
		Result: &AmazonListingDraft{
			TaskID:       "task-edit-attribute",
			Status:       string(TaskStatusNeedsReview),
			Marketplace:  "amazon",
			Country:      "US",
			Title:        "SoundPeak Headphones",
			Brand:        "SoundPeak",
			Description:  "Noise cancelling bluetooth headphones with long battery life.",
			ProductType:  "Headphones",
			CategoryPath: []string{"Electronics", "Headphones"},
			Attributes: map[string]string{
				"material": "unknown",
				"brand":    "SoundPeak",
			},
			Images:  &AmazonImageBundle{MainImage: "https://example.com/main.jpg", WhiteBgImage: "https://example.com/white.jpg"},
			Pricing: &AmazonPricingDraft{Currency: "USD", SuggestedPrice: 59.9},
			Variants: []AmazonVariantDraft{
				{SKU: "SKU-1", IsDefault: true},
			},
			CanonicalProduct: &productenrich.CanonicalProduct{
				Title:        "SoundPeak Headphones",
				Brand:        "SoundPeak",
				Description:  "Noise cancelling bluetooth headphones with long battery life.",
				CategoryPath: []string{"Electronics", "Headphones"},
				Attributes: map[string]productenrich.CanonicalAttribute{
					"material": {Value: "unknown", Trace: productenrich.FieldTrace{NeedsReview: true, Confidence: 0.4}},
					"brand":    {Value: "SoundPeak", Trace: productenrich.FieldTrace{Confidence: 1}},
				},
				FieldTraces: map[string]productenrich.FieldTrace{
					"title":         {Confidence: 1},
					"brand":         {Confidence: 1},
					"description":   {Confidence: 1},
					"category_path": {Confidence: 1},
				},
				NeedsReview: true,
			},
			ReviewItems: []AmazonReviewItem{
				{Field: "attributes.material", Action: OperatorActionFillAttributes, Reason: "material has low confidence", NeedsHuman: true},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := svc.ReviewTask(context.Background(), "task-edit-attribute", &ReviewTaskRequest{
		Action: "apply_edits",
		Edits: []DraftFieldEdit{
			{Field: "attributes.material", StringValue: "ABS Plastic"},
		},
	})
	if err != nil {
		t.Fatalf("ReviewTask(apply_edits attribute): %v", err)
	}
	if result.Result == nil {
		t.Fatal("expected result")
	}
	if got := result.Result.Attributes["material"]; got != "ABS Plastic" {
		t.Fatalf("draft material = %q, want ABS Plastic", got)
	}
	if result.Result.CanonicalProduct == nil {
		t.Fatal("expected canonical product")
	}
	attr := result.Result.CanonicalProduct.Attributes["material"]
	if attr.Value != "ABS Plastic" {
		t.Fatalf("canonical material = %q, want ABS Plastic", attr.Value)
	}
	if attr.Trace.NeedsReview {
		t.Fatalf("canonical material trace should no longer need review: %+v", attr.Trace)
	}
	for _, item := range result.Result.ReviewItems {
		if item.Field == "attributes.material" {
			t.Fatalf("expected attribute review item to be removed, got %+v", item)
		}
	}
}

func TestReviewTask_ApplySpecificationEditsUpdateCanonicalAndDraft(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	dimensionLength := 12.5
	weightValue := 0.45
	task := &Task{
		ID:        "task-edit-specs",
		Status:    TaskStatusNeedsReview,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			ProductURL:  "https://detail.1688.com/offer/123.html",
		},
		Result: &AmazonListingDraft{
			TaskID:       "task-edit-specs",
			Status:       string(TaskStatusNeedsReview),
			Marketplace:  "amazon",
			Country:      "US",
			Title:        "SoundPeak Headphones",
			Brand:        "SoundPeak",
			Description:  "Noise cancelling bluetooth headphones with long battery life.",
			ProductType:  "Headphones",
			CategoryPath: []string{"Electronics", "Headphones"},
			Attributes: map[string]string{
				"brand":    "SoundPeak",
				"material": "unknown",
			},
			Images:     &AmazonImageBundle{MainImage: "https://example.com/main.jpg", WhiteBgImage: "https://example.com/white.jpg"},
			Pricing:    &AmazonPricingDraft{Currency: "USD", SuggestedPrice: 59.9},
			Dimensions: &AmazonDimensions{},
			Weight:     &AmazonWeight{},
			Variants:   []AmazonVariantDraft{{SKU: "SKU-1", IsDefault: true}},
			CanonicalProduct: &productenrich.CanonicalProduct{
				Title:        "SoundPeak Headphones",
				Brand:        "SoundPeak",
				Description:  "Noise cancelling bluetooth headphones with long battery life.",
				CategoryPath: []string{"Electronics", "Headphones"},
				Attributes: map[string]productenrich.CanonicalAttribute{
					"brand": {Value: "SoundPeak", Trace: productenrich.FieldTrace{Confidence: 1}},
				},
				Specifications: &productenrich.ProductSpecs{
					Technical: map[string]string{"material": "unknown"},
				},
				FieldTraces: map[string]productenrich.FieldTrace{
					"title":         {Confidence: 1},
					"brand":         {Confidence: 1},
					"description":   {Confidence: 1},
					"category_path": {Confidence: 1},
					"specifications": {NeedsReview: true, Confidence: 0.4},
				},
				NeedsReview: true,
			},
			ReviewItems: []AmazonReviewItem{
				{Field: "specifications.technical.material", Action: OperatorActionFillAttributes, Reason: "material spec missing", NeedsHuman: true},
				{Field: "dimensions.unit", Action: OperatorActionFillAttributes, Reason: "dimensions unit missing", NeedsHuman: true},
				{Field: "weight.unit", Action: OperatorActionFillAttributes, Reason: "weight unit missing", NeedsHuman: true},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := svc.ReviewTask(context.Background(), "task-edit-specs", &ReviewTaskRequest{
		Action: "apply_edits",
		Edits: []DraftFieldEdit{
			{Field: "specifications.technical.material", StringValue: "ABS Plastic"},
			{Field: "dimensions.length", NumberValue: &dimensionLength},
			{Field: "dimensions.unit", StringValue: "cm"},
			{Field: "weight.value", NumberValue: &weightValue},
			{Field: "weight.unit", StringValue: "kg"},
		},
	})
	if err != nil {
		t.Fatalf("ReviewTask(apply_edits specs): %v", err)
	}
	if result.Result == nil {
		t.Fatal("expected result")
	}
	if got := result.Result.Attributes["material"]; got != "ABS Plastic" {
		t.Fatalf("draft material = %q, want ABS Plastic", got)
	}
	if result.Result.Dimensions == nil || result.Result.Dimensions.Length != dimensionLength || result.Result.Dimensions.Unit != "cm" {
		t.Fatalf("draft dimensions = %+v", result.Result.Dimensions)
	}
	if result.Result.Weight == nil || result.Result.Weight.Value != weightValue || result.Result.Weight.Unit != "kg" {
		t.Fatalf("draft weight = %+v", result.Result.Weight)
	}
	if result.Result.CanonicalProduct == nil || result.Result.CanonicalProduct.Specifications == nil {
		t.Fatalf("expected canonical specifications, got %+v", result.Result.CanonicalProduct)
	}
	if result.Result.CanonicalProduct.Specifications.Technical["material"] != "ABS Plastic" {
		t.Fatalf("canonical technical material = %q, want ABS Plastic", result.Result.CanonicalProduct.Specifications.Technical["material"])
	}
	if result.Result.CanonicalProduct.Specifications.Dimensions == nil || result.Result.CanonicalProduct.Specifications.Dimensions.Unit != "cm" {
		t.Fatalf("canonical dimensions = %+v", result.Result.CanonicalProduct.Specifications.Dimensions)
	}
	if result.Result.CanonicalProduct.Specifications.Weight == nil || result.Result.CanonicalProduct.Specifications.Weight.Unit != "kg" {
		t.Fatalf("canonical weight = %+v", result.Result.CanonicalProduct.Specifications.Weight)
	}
	for _, item := range result.Result.ReviewItems {
		if item.Field == "specifications.technical.material" || item.Field == "dimensions.unit" || item.Field == "weight.unit" {
			t.Fatalf("expected specification review items to be removed, got %+v", item)
		}
	}
}

func TestReviewTask_ApplyPackageAndVariantEditsUpdateCanonicalAndDraft(t *testing.T) {
	repo := &stubRepository{}
	svc, err := NewService(&ServiceConfig{
		Repository:     repo,
		ProductService: &stubProductService{},
		Assembler:      NewAssembler(),
		Validator:      NewValidator(),
		AutoFixer:      NewAutoFixer(),
		ExportBuilder:  NewExportBuilder(),
	})
	if err != nil {
		t.Fatalf("new service: %v", err)
	}

	packageQuantity := 2.0
	packageWeight := 0.8
	variantInventory := 25.0
	variantPrice := 79.9
	task := &Task{
		ID:        "task-edit-package-variant",
		Status:    TaskStatusNeedsReview,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &GenerateRequest{
			Marketplace: "amazon",
			Country:     "US",
			ProductURL:  "https://detail.1688.com/offer/123.html",
		},
		Result: &AmazonListingDraft{
			TaskID:       "task-edit-package-variant",
			Status:       string(TaskStatusNeedsReview),
			Marketplace:  "amazon",
			Country:      "US",
			Title:        "SoundPeak Headphones",
			Brand:        "SoundPeak",
			Description:  "Noise cancelling bluetooth headphones with long battery life.",
			ProductType:  "Headphones",
			CategoryPath: []string{"Electronics", "Headphones"},
			Attributes: map[string]string{
				"brand": "SoundPeak",
			},
			Images:  &AmazonImageBundle{MainImage: "https://example.com/main.jpg", WhiteBgImage: "https://example.com/white.jpg"},
			Pricing: &AmazonPricingDraft{Currency: "USD", SuggestedPrice: 99.9},
			Package: &AmazonPackageInfo{},
			Variants: []AmazonVariantDraft{
				{
					SKU:       "",
					Inventory: 0,
					Price:     &AmazonMoney{Currency: "USD", Amount: 0},
					Attributes: map[string]string{
						"color": "unknown",
					},
					IsDefault: false,
				},
			},
			CanonicalProduct: &productenrich.CanonicalProduct{
				Title:        "SoundPeak Headphones",
				Brand:        "SoundPeak",
				Description:  "Noise cancelling bluetooth headphones with long battery life.",
				CategoryPath: []string{"Electronics", "Headphones"},
				Attributes: map[string]productenrich.CanonicalAttribute{
					"brand": {Value: "SoundPeak", Trace: productenrich.FieldTrace{Confidence: 1}},
				},
				Specifications: &productenrich.ProductSpecs{
					Package: &productenrich.PackageInfo{},
				},
				Variants: []productenrich.CanonicalVariant{
					{
						Attributes: map[string]productenrich.CanonicalAttribute{
							"color": {Value: "unknown", Trace: productenrich.FieldTrace{NeedsReview: true, Confidence: 0.4}},
						},
						Trace: productenrich.FieldTrace{NeedsReview: true, Confidence: 0.4},
					},
				},
				FieldTraces: map[string]productenrich.FieldTrace{
					"title":         {Confidence: 1},
					"brand":         {Confidence: 1},
					"description":   {Confidence: 1},
					"category_path": {Confidence: 1},
					"specifications": {NeedsReview: true, Confidence: 0.4},
				},
				NeedsReview: true,
			},
			ReviewItems: []AmazonReviewItem{
				{Field: "package.quantity", Action: OperatorActionFillAttributes, Reason: "package quantity missing", NeedsHuman: true},
				{Field: "package.weight.unit", Action: OperatorActionFillAttributes, Reason: "package weight unit missing", NeedsHuman: true},
				{Field: "variants.sku", Action: OperatorActionFillSKU, Reason: "variant SKU is required", NeedsHuman: true},
				{Field: "variants.price", Action: OperatorActionEditPrice, Reason: "variant price missing", NeedsHuman: true},
				{Field: "variants.default", Action: OperatorActionEditSKU, Reason: "default variant is missing", NeedsHuman: true},
				{Field: "variants[0].attributes.color", Action: OperatorActionFillAttributes, Reason: "variant color has low confidence", NeedsHuman: true},
			},
		},
	}
	if err := repo.CreateTask(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}

	result, err := svc.ReviewTask(context.Background(), "task-edit-package-variant", &ReviewTaskRequest{
		Action: "apply_edits",
		Edits: []DraftFieldEdit{
			{Field: "package.quantity", NumberValue: &packageQuantity},
			{Field: "package.weight.value", NumberValue: &packageWeight},
			{Field: "package.weight.unit", StringValue: "kg"},
			{Field: "variants[0].sku", StringValue: "SP-HP-001-BLK"},
			{Field: "variants[0].inventory", NumberValue: &variantInventory},
			{Field: "variants[0].price.amount", NumberValue: &variantPrice},
			{Field: "variants[0].price.currency", StringValue: "USD"},
			{Field: "variants[0].attributes.color", StringValue: "Black"},
			{Field: "variants[0].main_image", StringValue: "https://example.com/variant-black.jpg"},
			{Field: "variants[0].is_default", StringValue: "true"},
		},
	})
	if err != nil {
		t.Fatalf("ReviewTask(apply_edits package+variant): %v", err)
	}
	if result.Result == nil {
		t.Fatal("expected result")
	}
	if result.Result.Package == nil || result.Result.Package.Quantity != 2 || result.Result.Package.Weight == nil || result.Result.Package.Weight.Unit != "kg" {
		t.Fatalf("draft package = %+v", result.Result.Package)
	}
	if len(result.Result.Variants) != 1 {
		t.Fatalf("draft variants len = %d, want 1", len(result.Result.Variants))
	}
	variant := result.Result.Variants[0]
	if variant.SKU != "SP-HP-001-BLK" || variant.Inventory != 25 || !variant.IsDefault {
		t.Fatalf("draft variant = %+v", variant)
	}
	if variant.Price == nil || variant.Price.Amount != variantPrice || variant.Price.Currency != "USD" {
		t.Fatalf("draft variant price = %+v", variant.Price)
	}
	if variant.Attributes["color"] != "Black" || variant.MainImage != "https://example.com/variant-black.jpg" {
		t.Fatalf("draft variant details = %+v", variant)
	}
	if result.Result.CanonicalProduct == nil || result.Result.CanonicalProduct.Specifications == nil || result.Result.CanonicalProduct.Specifications.Package == nil {
		t.Fatalf("expected canonical package, got %+v", result.Result.CanonicalProduct)
	}
	if result.Result.CanonicalProduct.Specifications.Package.Quantity != 2 || result.Result.CanonicalProduct.Specifications.Package.Weight == nil || result.Result.CanonicalProduct.Specifications.Package.Weight.Unit != "kg" {
		t.Fatalf("canonical package = %+v", result.Result.CanonicalProduct.Specifications.Package)
	}
	if len(result.Result.CanonicalProduct.Variants) != 1 {
		t.Fatalf("canonical variants len = %d, want 1", len(result.Result.CanonicalProduct.Variants))
	}
	canonicalVariant := result.Result.CanonicalProduct.Variants[0]
	if canonicalVariant.SKU != "SP-HP-001-BLK" || canonicalVariant.Stock != 25 || !canonicalVariant.IsDefault {
		t.Fatalf("canonical variant = %+v", canonicalVariant)
	}
	if canonicalVariant.Price == nil || canonicalVariant.Price.Amount != variantPrice || canonicalVariant.Price.Currency != "USD" {
		t.Fatalf("canonical variant price = %+v", canonicalVariant.Price)
	}
	if canonicalVariant.Attributes["color"].Value != "Black" {
		t.Fatalf("canonical variant color = %+v", canonicalVariant.Attributes["color"])
	}
	if len(canonicalVariant.Images) != 1 || canonicalVariant.Images[0].URL != "https://example.com/variant-black.jpg" {
		t.Fatalf("canonical variant images = %+v", canonicalVariant.Images)
	}
	for _, item := range result.Result.ReviewItems {
		if item.Field == "package.quantity" || item.Field == "package.weight.unit" || item.Field == "variants.sku" || item.Field == "variants.price" || item.Field == "variants.default" || item.Field == "variants[0].attributes.color" {
			t.Fatalf("expected package/variant review items to be removed, got %+v", item)
		}
	}
}
