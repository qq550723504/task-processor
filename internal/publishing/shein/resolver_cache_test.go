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

func TestCachedCategoryResolverDoesNotRememberUnsubmittedLiveResolution(t *testing.T) {
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
	if inner.calls != 2 {
		t.Fatalf("inner calls = %d, want 2", inner.calls)
	}
	if first == second {
		t.Fatal("cached resolver should return cloned resolutions")
	}
	if second.CategoryID != 8218 {
		t.Fatalf("category id = %d, want 8218", second.CategoryID)
	}
	if second.Cache != nil {
		t.Fatalf("live generated category cache metadata = %#v, want nil before final submit", second.Cache)
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

	next := resolver.Resolve(&BuildRequest{SheinStoreID: 42, TargetCategoryHint: "8218"}, canonical, &Package{SpuName: "Envelope Pillow Cover"})
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if next.CategoryID != 8218 {
		t.Fatalf("category id = %d, want 8218", next.CategoryID)
	}

	withoutHint := resolver.Resolve(&BuildRequest{SheinStoreID: 42}, canonical, &Package{SpuName: "Envelope Pillow Cover"})
	if inner.calls != 1 {
		t.Fatalf("inner calls without target hint = %d, want 1", inner.calls)
	}
	if withoutHint.CategoryID != 9999 {
		t.Fatalf("category id without target hint = %d, want live resolver result", withoutHint.CategoryID)
	}
}

func TestCategoryResolverCacheKeyUsesStableSDSIdentifiers(t *testing.T) {
	req := &BuildRequest{SheinStoreID: 42}
	canonical := &canonical.Product{
		Title:        "啤酒盖铁板（包邮仅限美国直发）",
		CategoryPath: []string{"美国本地直发", "生活用品", "铁板画"},
	}
	first := &Package{
		SpuName:       "啤酒盖铁板（包邮仅限美国直发）",
		ProductNameEn: "Vintage Metal Bottle Cap Wall Sign - Professional Lazy Expert Sloth Print",
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
			{Name: "sku", Value: "MG8014062"},
		},
	}
	second := &Package{
		SpuName:       "啤酒盖铁板（包邮仅限美国直发）",
		ProductNameEn: "Professional Lazy Expert Metal Bottle Cap Wall Sign, Funny Sloth Gaming Decor",
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
			{Name: "sku", Value: "MG8014062"},
		},
	}

	firstKey := categoryResolverCacheKey(req, canonical, first)
	secondKey := categoryResolverCacheKey(req, canonical, second)
	if firstKey == "" || secondKey == "" {
		t.Fatalf("cache keys should not be empty: first=%q second=%q", firstKey, secondKey)
	}
	if firstKey != secondKey {
		t.Fatalf("category cache key drifted for stable SDS identifiers: first=%s second=%s", firstKey, secondKey)
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

func TestCachedAttributeResolverDoesNotPersistUnsubmittedLiveResolution(t *testing.T) {
	valueID := 2001
	store := newResolutionCacheTestStore(t)
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
	resolver := NewCachedAttributeResolver(inner, store)
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
	if inner.calls != 2 {
		t.Fatalf("inner calls = %d, want 2", inner.calls)
	}
	if first == second {
		t.Fatal("cached resolver should return cloned resolutions")
	}
	if got := second.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 2001 {
		t.Fatalf("attribute value id = %v, want 2001", got)
	}
	if second.Cache != nil {
		t.Fatalf("live generated attribute cache metadata = %#v, want nil before final submit", second.Cache)
	}
	got, err := store.GetResolutionCache(context.Background(), ResolutionCacheKindAttribute, "42", attributeResolverCacheKey(req, nil, pkg))
	if err != nil {
		t.Fatalf("get persisted cache: %v", err)
	}
	if got != nil {
		t.Fatalf("persisted live generated attribute cache = %#v, want nil before final submit", got)
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

func TestCachedAttributeResolverCanReusePublishedResolutionAfterListingCopyNormalization(t *testing.T) {
	valueID := 2001
	inner := &countingAttributeResolver{
		out: &AttributeResolution{Status: "partial", CategoryID: 8218, TemplateCount: 1},
	}
	resolver := NewCachedAttributeResolver(inner)
	cache := resolver.(AttributeResolutionCache)
	req := &BuildRequest{SheinStoreID: 42}
	preNormalizationPkg := &Package{
		SpuName:        "抱枕套 MG8014192",
		ProductNameEn:  "抱枕套",
		Description:    "中文描述",
		CategoryPath:   []string{"家居", "装饰", "抱枕套"},
		CategoryID:     8218,
		CategoryIDList: []int{2030, 6012, 8218},
		Attributes:     map[string]string{"color": "White"},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8014192"},
			{Name: "material", Value: "涤纶"},
			{Name: "product_size", Value: "45x45cm"},
		},
		RequestDraft: &RequestDraft{},
	}
	postNormalizationPkg := &Package{
		SpuName:          "抱枕套 MG8014192",
		ProductNameEn:    "Envelope Pillow Cover",
		ProductNameMulti: "Envelope Pillow Cover",
		Description:      "Soft polyester pillow cover for home decor.",
		CategoryPath:     []string{"家居", "装饰", "抱枕套"},
		CategoryID:       8218,
		CategoryIDList:   []int{2030, 6012, 8218},
		Attributes:       map[string]string{"color": "White"},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8014192"},
			{Name: "material", Value: "涤纶"},
			{Name: "product_size", Value: "45x45cm"},
		},
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SKUList: []SKUDraft{{
					SupplierSKU: "MG8014192",
					Length:      "45",
					Width:       "45",
					Height:      "1",
				}},
			}},
		},
	}
	cache.RememberAttributeResolution(req, nil, postNormalizationPkg, &AttributeResolution{
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

	next := resolver.Resolve(req, nil, preNormalizationPkg)
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if next.Cache == nil {
		t.Fatal("expected cached resolution metadata after publish-history hit")
	}
	if got := next.ResolvedAttributes[0].AttributeValueID; got == nil || *got != 2001 {
		t.Fatalf("attribute value id = %v, want 2001", got)
	}
}

func TestAttributeResolverCacheKeyUsesStableSDSIdentifiers(t *testing.T) {
	req := &BuildRequest{SheinStoreID: 42}
	first := &Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		SpuName:        "啤酒盖铁板（包邮仅限美国直发）",
		ProductNameEn:  "Vintage Metal Bottle Cap Wall Sign - Professional Lazy Expert Sloth Print",
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
			{Name: "sku", Value: "MG8014062"},
			{Name: "material", Value: "金属"},
		},
	}
	second := &Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		SpuName:        "啤酒盖铁板（包邮仅限美国直发）",
		ProductNameEn:  "Professional Lazy Expert Metal Bottle Cap Wall Sign, Funny Sloth Gaming Decor",
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
			{Name: "sku", Value: "MG8014062"},
			{Name: "material", Value: "金属"},
		},
	}

	firstKey := attributeResolverCacheKey(req, nil, first)
	secondKey := attributeResolverCacheKey(req, nil, second)
	if firstKey == "" || secondKey == "" {
		t.Fatalf("cache keys should not be empty: first=%q second=%q", firstKey, secondKey)
	}
	if firstKey != secondKey {
		t.Fatalf("attribute cache key drifted for stable SDS identifiers: first=%s second=%s", firstKey, secondKey)
	}
}

func TestAttributeResolverCacheKeyIgnoresDecoratedSubmitSupplierSKUsForSDS(t *testing.T) {
	req := &BuildRequest{SheinStoreID: 42}
	first := &Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
		},
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SupplierCode: "MG8014062001-8A78E611",
				SKUList: []SKUDraft{{
					SupplierSKU: "MG8014062001-V124111-T838E0EBE-R84A7E-8A78E611",
					Attributes:  map[string]string{"source_sds_sku": "MG8014062001"},
				}},
			}},
		},
	}
	second := &Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
		},
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SupplierCode: "MG8014062001-8A78E611",
				SKUList: []SKUDraft{{
					SupplierSKU: "MG8014062001-8A78E611",
					Attributes:  map[string]string{"source_sds_sku": "MG8014062001"},
				}},
			}},
		},
	}

	firstKey := attributeResolverCacheKey(req, nil, first)
	secondKey := attributeResolverCacheKey(req, nil, second)
	if firstKey == "" || secondKey == "" {
		t.Fatalf("cache keys should not be empty: first=%q second=%q", firstKey, secondKey)
	}
	if firstKey != secondKey {
		t.Fatalf("attribute cache key drifted across decorated submit SKUs: first=%s second=%s", firstKey, secondKey)
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
	cacheKey := attributeResolverCacheKey(req, nil, pkg)
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
	storedKey := attributeResolverCacheKey(req, nil, cachedPkg)
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

func TestCachedAttributeResolverRememberStoresOnlyCurrentKey(t *testing.T) {
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
	originalKey := attributeResolverCacheKey(req, nil, originalPkg)
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

	currentKey := attributeResolverCacheKey(req, nil, patchedPkg)
	got, err := store.GetResolutionCache(context.Background(), ResolutionCacheKindAttribute, "42", currentKey)
	if err != nil {
		t.Fatalf("get current key: %v", err)
	}
	if got == nil || !got.Manual || got.Source != "manual_cache" {
		t.Fatalf("current key cache = %#v, want manual_cache", got)
	}
	if stale, err := store.GetResolutionCache(context.Background(), ResolutionCacheKindAttribute, "42", originalKey); err != nil {
		t.Fatalf("get stale key: %v", err)
	} else if stale != nil {
		t.Fatalf("stale key cache = %#v, want nil", stale)
	}
	decoded := decodeAttributeCacheEntry(got)
	if decoded == nil || decoded.Status != "resolved" || decoded.ResolvedCount != 2 {
		t.Fatalf("decoded current key = %#v", decoded)
	}
}

func TestCachedSaleAttributeResolverDoesNotRememberUnsubmittedVariantMatrix(t *testing.T) {
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
	if inner.calls != 2 {
		t.Fatalf("inner calls = %d, want 2", inner.calls)
	}
	if first == second {
		t.Fatal("cached resolver should return cloned resolutions")
	}
	if got := second.skcValueAssignments["white"].AttributeValueID; got == nil || *got != 103 {
		t.Fatalf("cached value assignment = %v, want 103", got)
	}
	if second.Cache != nil {
		t.Fatalf("live generated sale attribute cache metadata = %#v, want nil before final submit", second.Cache)
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

func TestSaleAttributeResolverCacheKeyUsesCanonicalSDSIdentifiers(t *testing.T) {
	req := &BuildRequest{SheinStoreID: 42}
	firstCanonical := &canonical.Product{
		Title: "啤酒盖铁板（包邮仅限美国直发）",
		Variants: []canonical.Variant{{
			SKU: "MG8014062001-OLDSTYLE",
			Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8014062001"},
				"Color":          {Value: "white"},
			},
		}},
	}
	secondCanonical := &canonical.Product{
		Title: "啤酒盖铁板（包邮仅限美国直发）",
		Variants: []canonical.Variant{{
			SKU: "MG8014062001-NEWSTYLE",
			Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8014062001"},
				"Color":          {Value: "white"},
			},
		}},
	}
	pkg := &Package{
		CategoryID:     2486,
		CategoryIDList: []int{2030, 1952, 8007, 2486},
	}

	firstKey := saleAttributeResolverCacheKey(req, firstCanonical, pkg)
	secondKey := saleAttributeResolverCacheKey(req, secondCanonical, pkg)
	if firstKey == "" || secondKey == "" {
		t.Fatalf("sale attribute cache keys should not be empty: first=%q second=%q", firstKey, secondKey)
	}
	if firstKey != secondKey {
		t.Fatalf("sale attribute cache key drifted for same canonical SDS source sku: first=%s second=%s", firstKey, secondKey)
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
