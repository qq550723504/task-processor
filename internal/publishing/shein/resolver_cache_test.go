package shein

import (
	"context"
	"testing"

	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
)

type countingCategoryResolver struct {
	calls int
	out   *CategoryResolution
}

func (r *countingCategoryResolver) Resolve(_ *BuildRequest, _ *canonical.Product, _ *Package) *CategoryResolution {
	r.calls++
	return cloneCategoryResolution(r.out)
}

type countingAttributeResolver struct {
	calls int
	out   *AttributeResolution
}

func (r *countingAttributeResolver) Resolve(_ *BuildRequest, _ *canonical.Product, _ *Package) *AttributeResolution {
	r.calls++
	return cloneAttributeResolution(r.out)
}

type countingSaleAttributeResolver struct {
	calls int
	out   *SaleAttributeResolution
}

func (r *countingSaleAttributeResolver) Resolve(_ *BuildRequest, _ *canonical.Product, _ *Package) *SaleAttributeResolution {
	r.calls++
	return cloneSaleAttributeResolution(r.out)
}

func TestCachedCategoryResolverReusesStableSDSBaseProduct(t *testing.T) {
	inner := &countingCategoryResolver{
		out: &CategoryResolution{
			Status:         "resolved",
			Source:         "test",
			CategoryID:     8218,
			CategoryIDList: []int{2030, 6012, 8218},
			MatchedPath:    []string{"Home", "Decor", "Cushion Covers"},
		},
	}
	resolver := NewCachedCategoryResolver(inner)
	req := &BuildRequest{SheinStoreID: 42}
	canonical := &canonical.Product{
		Title:        "Envelope Pillow Cover",
		CategoryPath: []string{"美国本地直发", "生活用品", "抱枕套"},
	}
	pkg := &Package{}

	first := resolver.Resolve(req, canonical, pkg)
	second := resolver.Resolve(req, canonical, pkg)
	if inner.calls != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.calls)
	}
	if first == second {
		t.Fatal("cached resolver should return cloned resolutions")
	}
	if second.CategoryID != 8218 {
		t.Fatalf("category id = %d, want 8218", second.CategoryID)
	}
	if len(second.ReviewNotes) != 1 || second.ReviewNotes[0] == "" {
		t.Fatalf("cache note not attached: %#v", second.ReviewNotes)
	}
}

func TestCachedCategoryResolverCanRememberManualResolutionForSourceCategory(t *testing.T) {
	inner := &countingCategoryResolver{
		out: &CategoryResolution{Status: "resolved", Source: "inner", CategoryID: 9999},
	}
	resolver := NewCachedCategoryResolver(inner)
	cache := resolver.(CategoryResolutionCache)
	req := &BuildRequest{SheinStoreID: 42, TargetCategoryHint: "8218"}
	canonical := &canonical.Product{
		Title:        "Envelope Pillow Cover",
		CategoryPath: []string{"美国本地直发", "生活用品", "抱枕套"},
	}
	manualPkg := &Package{
		SpuName:      "Envelope Pillow Cover",
		CategoryPath: []string{"Home", "Decor", "Cushion Covers"},
	}
	cache.RememberCategoryResolution(req, canonical, manualPkg, &CategoryResolution{
		Status:         "resolved",
		Source:         "manual",
		CategoryID:     8218,
		CategoryIDList: []int{2030, 6012, 8218},
		MatchedPath:    []string{"Home", "Decor", "Cushion Covers"},
	})

	next := resolver.Resolve(&BuildRequest{SheinStoreID: 42}, canonical, &Package{SpuName: "Envelope Pillow Cover"})
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if next.CategoryID != 8218 {
		t.Fatalf("category id = %d, want 8218", next.CategoryID)
	}
}

func TestCachedAttributeResolverSkipsZeroHitResolution(t *testing.T) {
	inner := &countingAttributeResolver{
		out: &AttributeResolution{
			Status:          "partial",
			Source:          "attribute_templates",
			CategoryID:      8218,
			TemplateCount:   1,
			ResolvedCount:   0,
			UnresolvedCount: 2,
		},
	}
	resolver := NewCachedAttributeResolver(inner)
	req := &BuildRequest{SheinStoreID: 42}
	pkg := &Package{
		CategoryID: 8218,
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8014192"},
			{Name: "material", Value: "涤纶"},
		},
	}

	resolver.Resolve(req, nil, pkg)
	resolver.Resolve(req, nil, pkg)
	if inner.calls != 2 {
		t.Fatalf("inner calls = %d, want 2", inner.calls)
	}
}

func TestCachedAttributeResolverReusesResolvedBaseProductAttributes(t *testing.T) {
	valueID := 2001
	inner := &countingAttributeResolver{
		out: &AttributeResolution{
			Status:        "resolved",
			Source:        "attribute_templates",
			CategoryID:    8218,
			TemplateCount: 1,
			ResolvedCount: 1,
			ResolvedAttributes: []ResolvedAttribute{{
				Name:             "Material",
				Value:            "Polyester",
				AttributeID:      160,
				AttributeValueID: &valueID,
			}},
		},
	}
	resolver := NewCachedAttributeResolver(inner)
	req := &BuildRequest{SheinStoreID: 42}
	pkg := &Package{
		CategoryID:       8218,
		CategoryIDList:   []int{2030, 6012, 8218},
		ProductNameEn:    "Envelope Pillow Cover",
		ProductNameMulti: "Envelope Pillow Cover",
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8014192"},
			{Name: "material", Value: "涤纶"},
		},
	}

	first := resolver.Resolve(req, nil, pkg)
	second := resolver.Resolve(req, nil, pkg)
	if inner.calls != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.calls)
	}
	if first == second {
		t.Fatal("cached resolver should return cloned resolutions")
	}
	if got := second.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 2001 {
		t.Fatalf("attribute value id = %v, want 2001", got)
	}
	if len(second.ReviewNotes) != 1 || second.ReviewNotes[0] == "" {
		t.Fatalf("cache note not attached: %#v", second.ReviewNotes)
	}
}

func TestCachedAttributeResolverCanRememberManualResolution(t *testing.T) {
	valueID := 2001
	inner := &countingAttributeResolver{
		out: &AttributeResolution{Status: "partial", CategoryID: 8218, TemplateCount: 1},
	}
	resolver := NewCachedAttributeResolver(inner)
	cache := resolver.(AttributeResolutionCache)
	req := &BuildRequest{SheinStoreID: 42}
	pkg := &Package{
		CategoryID:     8218,
		CategoryIDList: []int{2030, 6012, 8218},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8014192"},
			{Name: "material", Value: "涤纶"},
		},
	}
	cache.RememberAttributeResolution(req, nil, pkg, &AttributeResolution{
		Status:        "resolved",
		Source:        "manual",
		CategoryID:    8218,
		TemplateCount: 1,
		ResolvedCount: 1,
		ResolvedAttributes: []ResolvedAttribute{{
			Name:             "Material",
			Value:            "Polyester",
			AttributeID:      160,
			AttributeValueID: &valueID,
		}},
	})

	next := resolver.Resolve(req, nil, pkg)
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if got := next.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 2001 {
		t.Fatalf("attribute value id = %v, want 2001", got)
	}
}

func TestCachedAttributeResolverLoadsPersistentCacheAndRefillsMemory(t *testing.T) {
	valueID := 2001
	store := newResolutionCacheTestStore(t)
	req := &BuildRequest{SheinStoreID: 42}
	pkg := &Package{
		CategoryID:     8218,
		CategoryIDList: []int{2030, 6012, 8218},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8014192"},
			{Name: "material", Value: "涤纶"},
		},
	}
	cacheKey := attributeResolverCacheKey(req, pkg)
	if err := store.SaveResolutionCache(context.Background(), &SheinResolutionCacheEntry{
		StoreID:        "42",
		CacheKind:      ResolutionCacheKindAttribute,
		CacheKey:       cacheKey,
		ShortKey:       shortResolutionCacheKey(cacheKey),
		Source:         "manual_cache",
		Manual:         true,
		ResolutionJSON: `{"status":"resolved","category_id":8218,"template_count":1,"resolved_count":1,"resolved_attributes":[{"name":"Material","value":"Polyester","attribute_id":160,"attribute_value_id":2001}]}`,
	}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	inner := &countingAttributeResolver{
		out: &AttributeResolution{Status: "partial", CategoryID: 8218, TemplateCount: 1},
	}
	resolver := NewCachedAttributeResolver(inner, store)

	first := resolver.Resolve(req, nil, pkg)
	second := resolver.Resolve(req, nil, pkg)
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if first.Cache == nil || first.Cache.Source != "manual_cache" {
		t.Fatalf("first cache metadata = %#v, want manual_cache", first.Cache)
	}
	if second.Cache == nil || second.Cache.Source != "manual_cache" {
		t.Fatalf("second cache metadata = %#v, want manual memory hit", second.Cache)
	}
	if got := second.ResolvedAttributes[0].AttributeValueID; got == nil || *got != valueID {
		t.Fatalf("attribute value id = %v, want %d", got, valueID)
	}
}

func TestCachedAttributeResolverClearUsesStoredCacheMetadata(t *testing.T) {
	store := newResolutionCacheTestStore(t)
	req := &BuildRequest{SheinStoreID: 42}
	cachedPkg := &Package{
		CategoryID:     8218,
		CategoryIDList: []int{2030, 6012, 8218},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8014192"},
			{Name: "material", Value: "涤纶"},
		},
	}
	storedKey := attributeResolverCacheKey(req, cachedPkg)
	if err := store.SaveResolutionCache(context.Background(), &SheinResolutionCacheEntry{
		StoreID:        "42",
		CacheKind:      ResolutionCacheKindAttribute,
		CacheKey:       storedKey,
		ShortKey:       shortResolutionCacheKey(storedKey),
		Source:         "history_cache",
		ResolutionJSON: `{"status":"resolved","category_id":8218,"template_count":1,"resolved_count":1,"resolved_attributes":[{"name":"Material","value":"Polyester","attribute_id":160,"attribute_value_id":2001}]}`,
	}); err != nil {
		t.Fatalf("seed cache: %v", err)
	}
	clearPkg := &Package{
		CategoryID:     8218,
		CategoryIDList: []int{8218},
		AttributeResolution: &AttributeResolution{
			Cache: &ResolutionCacheInfo{
				ShortKey: shortResolutionCacheKey(storedKey),
			},
		},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "changed"},
		},
	}
	resolver := NewCachedAttributeResolver(&countingAttributeResolver{out: &AttributeResolution{Status: "partial"}}, store)
	cache := resolver.(AttributeResolutionCache)
	if err := cache.ClearAttributeResolution(req, nil, clearPkg); err != nil {
		t.Fatalf("clear cache: %v", err)
	}
	got, err := store.GetResolutionCache(context.Background(), ResolutionCacheKindAttribute, "42", storedKey)
	if err != nil {
		t.Fatalf("get after clear: %v", err)
	}
	if got != nil {
		t.Fatalf("stored cache after metadata clear = %#v, want nil", got)
	}
}

func TestCachedAttributeResolverRememberOverwritesPreviousHitKey(t *testing.T) {
	store := newResolutionCacheTestStore(t)
	req := &BuildRequest{SheinStoreID: 42}
	originalPkg := &Package{
		CategoryID:     8794,
		CategoryIDList: []int{2866, 4439, 5548, 8794},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8012004"},
			{Name: "material", Value: "100%涤纶"},
		},
	}
	originalKey := attributeResolverCacheKey(req, originalPkg)
	menValueID := 427
	resolution := &AttributeResolution{
		Status:        "resolved",
		Source:        "manual_review",
		CategoryID:    8794,
		TemplateCount: 1,
		ResolvedCount: 2,
		Cache: &ResolutionCacheInfo{
			CacheKey: originalKey,
		},
		ResolvedAttributes: []ResolvedAttribute{
			{Name: "Material", Value: "Polyester", AttributeID: 160, Required: true},
			{Name: "Gender", Value: "Men", AttributeID: 42, AttributeValueID: &menValueID, Required: true},
		},
	}
	patchedPkg := &Package{
		CategoryID:     8794,
		CategoryIDList: []int{2866, 4439, 5548, 8794},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8012004"},
			{Name: "material", Value: "100%涤纶"},
			{Name: "manual attribute", Value: "Gender=Men"},
		},
	}
	resolver := NewCachedAttributeResolver(&countingAttributeResolver{}, store)
	cache := resolver.(AttributeResolutionCache)
	cache.RememberAttributeResolution(req, nil, patchedPkg, resolution)

	got, err := store.GetResolutionCache(context.Background(), ResolutionCacheKindAttribute, "42", originalKey)
	if err != nil {
		t.Fatalf("get original key: %v", err)
	}
	if got == nil || !got.Manual || got.Source != "manual_cache" {
		t.Fatalf("original key cache = %#v, want manual_cache", got)
	}
	decoded := decodeAttributeCacheEntry(got)
	if decoded == nil || decoded.Status != "resolved" || decoded.ResolvedCount != 2 {
		t.Fatalf("decoded original key = %#v", decoded)
	}
}

func TestCachedSaleAttributeResolverReusesVariantMatrix(t *testing.T) {
	valueID := 103
	inner := &countingSaleAttributeResolver{
		out: &SaleAttributeResolution{
			Status:                 "resolved",
			Source:                 "sale_attribute_templates",
			CategoryID:             8218,
			PrimaryAttributeID:     27,
			PrimarySourceDimension: "Color",
			SKCAttributes: []ResolvedSaleAttribute{{
				Scope:            "skc",
				Name:             "Color",
				Value:            "White",
				AttributeID:      27,
				AttributeValueID: &valueID,
			}},
			skcValueAssignments: map[string]ResolvedSaleAttribute{
				"white": {
					Scope:            "skc",
					Name:             "Color",
					Value:            "White",
					AttributeID:      27,
					AttributeValueID: &valueID,
				},
			},
		},
	}
	resolver := NewCachedSaleAttributeResolver(inner)
	req := &BuildRequest{SheinStoreID: 42}
	canonical := saleCacheCanonical()
	pkg := &Package{CategoryID: 8218, CategoryIDList: []int{2030, 6012, 8218}}

	first := resolver.Resolve(req, canonical, pkg)
	second := resolver.Resolve(req, canonical, pkg)
	if inner.calls != 1 {
		t.Fatalf("inner calls = %d, want 1", inner.calls)
	}
	if first == second {
		t.Fatal("cached resolver should return cloned resolutions")
	}
	if got := second.skcValueAssignments["white"].AttributeValueID; got == nil || *got != 103 {
		t.Fatalf("cached value assignment = %v, want 103", got)
	}
	if len(second.ReviewNotes) != 1 || second.ReviewNotes[0] == "" {
		t.Fatalf("cache note not attached: %#v", second.ReviewNotes)
	}
}

func TestCachedSaleAttributeResolverCanRememberManualResolution(t *testing.T) {
	valueID := 103
	inner := &countingSaleAttributeResolver{
		out: &SaleAttributeResolution{Status: "partial", CategoryID: 8218},
	}
	resolver := NewCachedSaleAttributeResolver(inner)
	cache := resolver.(SaleAttributeResolutionCache)
	req := &BuildRequest{SheinStoreID: 42}
	canonical := saleCacheCanonical()
	pkg := &Package{CategoryID: 8218, CategoryIDList: []int{2030, 6012, 8218}}
	cache.RememberSaleAttributeResolution(req, canonical, pkg, &SaleAttributeResolution{
		Status:                 "resolved",
		Source:                 "manual",
		CategoryID:             8218,
		PrimaryAttributeID:     27,
		PrimarySourceDimension: "Color",
		SKCAttributes: []ResolvedSaleAttribute{{
			Scope:            "skc",
			Name:             "Color",
			Value:            "White",
			AttributeID:      27,
			AttributeValueID: &valueID,
		}},
	})

	next := resolver.Resolve(req, canonical, pkg)
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if next.PrimaryAttributeID != 27 {
		t.Fatalf("primary attribute id = %d, want 27", next.PrimaryAttributeID)
	}
}

func TestCachedSaleAttributeResolverSeparatesDifferentVariantMatrices(t *testing.T) {
	inner := &countingSaleAttributeResolver{
		out: &SaleAttributeResolution{
			Status:             "resolved",
			CategoryID:         8218,
			PrimaryAttributeID: 27,
		},
	}
	resolver := NewCachedSaleAttributeResolver(inner)
	req := &BuildRequest{SheinStoreID: 42}
	pkg := &Package{CategoryID: 8218}

	resolver.Resolve(req, saleCacheCanonical(), pkg)
	resolver.Resolve(req, &canonical.Product{
		Title: "Envelope Pillow Cover",
		Variants: []canonical.Variant{{
			SKU: "SKU-1",
			Attributes: map[string]canonical.Attribute{
				"Color": {Value: "Black"},
			},
		}},
	}, pkg)
	if inner.calls != 2 {
		t.Fatalf("inner calls = %d, want 2", inner.calls)
	}
}

func TestCachedSaleAttributeResolverRejectsPromptLikeManualCacheAndRecomputes(t *testing.T) {
	badValueID := 103
	goodValueID := 104
	inner := &countingSaleAttributeResolver{
		out: &SaleAttributeResolution{
			Status:                  "resolved",
			Source:                  "sale_attribute_templates",
			CategoryID:              12014,
			PrimaryAttributeID:      1001184,
			PrimarySourceDimension:  "ai_style",
			ValueSanitized:          true,
			ValueSanitizationSource: "rule_trimmed",
			ValuePromptContaminated: true,
			ValueResolutionNote:     "prompt-like ai_style replaced by rule-trimmed style value",
			SKCAttributes: []ResolvedSaleAttribute{{
				Scope:            "skc",
				Name:             "Style Type",
				Value:            "Blue Dog Graphic",
				AttributeID:      1001184,
				AttributeValueID: &goodValueID,
			}},
			skcValueAssignments: map[string]ResolvedSaleAttribute{
				"please design an image with suitable text and graphics, 3000 pixels wide": {
					Scope:            "skc",
					Name:             "Style Type",
					Value:            "Blue Dog Graphic",
					AttributeID:      1001184,
					AttributeValueID: &goodValueID,
				},
			},
		},
	}
	resolver := NewCachedSaleAttributeResolver(inner)
	cache := resolver.(SaleAttributeResolutionCache)
	req := &BuildRequest{SheinStoreID: 42}
	canonical := &canonical.Product{
		Title: "Flannel non slip floor mat",
		Variants: []canonical.Variant{{
			SKU: "SKU-1",
			Attributes: map[string]canonical.Attribute{
				"ai_style": {Value: "Please design an image with suitable text and graphics, 3000 pixels wide"},
				"Size":     {Value: "40x60cm"},
			},
		}},
	}
	pkg := &Package{CategoryID: 12014, CategoryIDList: []int{2030, 1952, 8144, 12014}}
	cache.RememberSaleAttributeResolution(req, canonical, pkg, &SaleAttributeResolution{
		Status:                 "resolved",
		Source:                 "manual",
		CategoryID:             12014,
		PrimaryAttributeID:     1001184,
		PrimarySourceDimension: "ai_style",
		SKCAttributes: []ResolvedSaleAttribute{{
			Scope:            "skc",
			Name:             "Style Type",
			Value:            "Please design an image with suitable text and graphics, 3000 pixels wide",
			AttributeID:      1001184,
			AttributeValueID: &badValueID,
		}},
		SKCValueAssignments: map[string]ResolvedSaleAttribute{
			"please design an image with suitable text and graphics, 3000 pixels wide": {
				Scope:            "skc",
				Name:             "Style Type",
				Value:            "Please design an image with suitable text and graphics, 3000 pixels wide",
				AttributeID:      1001184,
				AttributeValueID: &badValueID,
			},
		},
	})

	next := resolver.Resolve(req, canonical, pkg)
	if inner.calls != 1 {
		t.Fatalf("inner calls = %d, want 1 after rejecting dirty cache", inner.calls)
	}
	if next.CacheRejectedReason == "" {
		t.Fatal("expected cache rejected reason to be recorded")
	}
	if next.SKCAttributes[0].Value != "Blue Dog Graphic" {
		t.Fatalf("resolved SKC value = %q, want recomputed safe value", next.SKCAttributes[0].Value)
	}
}

func saleCacheCanonical() *canonical.Product {
	return &canonical.Product{
		Title: "Envelope Pillow Cover",
		Variants: []canonical.Variant{{
			SKU: "SKU-1",
			Attributes: map[string]canonical.Attribute{
				"Color": {Value: "White"},
			},
		}},
	}
}
