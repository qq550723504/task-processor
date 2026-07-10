package shein

import (
	"context"
	"testing"

	"task-processor/internal/catalog/canonical"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
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

func TestCachedCategoryResolverCanReuseRememberedResolutionForStudioUnresolvedAlias(t *testing.T) {
	store := newResolutionCacheTestStore(t)
	inner := &countingCategoryResolver{
		out: &CategoryResolution{Status: "resolved", Source: "inner", CategoryID: 9999},
	}
	writer := NewCachedCategoryResolver(inner, store).(CategoryResolutionCache)
	req := &BuildRequest{SheinStoreID: 42}
	resolvedCanonical := &canonical.Product{
		Title:        "啤酒盖铁板（包邮仅限美国直发）",
		CategoryPath: []string{"美国本地直发", "生活用品", "铁板画"},
		Attributes: map[string]canonical.Attribute{
			"product_sku": {Value: "MG8014062"},
			"variant_sku": {Value: "MG8014062001"},
		},
		Variants: []canonical.Variant{{
			SKU: "MG8014062001-BBF9B007",
			Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8014062001"},
			},
		}},
	}
	resolvedPkg := &Package{
		SpuName:      "啤酒盖铁板（包邮仅限美国直发）",
		CategoryPath: []string{"家居&生活", "家居装饰", "装饰挂饰和风铃", "装饰挂饰"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
		},
	}
	writer.RememberCategoryResolution(req, resolvedCanonical, resolvedPkg, &CategoryResolution{
		Status:         "resolved",
		Source:         "manual",
		CategoryID:     2486,
		CategoryIDList: []int{7, 2486},
		MatchedPath:    []string{"家居&生活", "家居装饰", "装饰挂饰和风铃", "装饰挂饰"},
	})

	readerInner := &countingCategoryResolver{
		out: &CategoryResolution{Status: "resolved", Source: "inner", CategoryID: 9999},
	}
	reader := NewCachedCategoryResolver(readerInner, store)
	unresolvedCanonical := &canonical.Product{
		Title: "啤酒盖铁板（包邮仅限美国直发）",
		Attributes: map[string]canonical.Attribute{
			"product_sku": {Value: "MG8014062"},
			"variant_sku": {Value: "MG8014062001"},
		},
		Variants: []canonical.Variant{{
			SKU: "MG8014062001-BBF9B007",
			Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8014062001"},
			},
		}},
	}
	unresolvedPkg := &Package{
		SpuName: "啤酒盖铁板（包邮仅限美国直发）",
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8014062"},
			{Name: "variant_sku", Value: "MG8014062001"},
		},
	}
	aliasKey := categoryResolverUnresolvedAliasKey(req, resolvedCanonical, resolvedPkg)
	unresolvedKey := categoryResolverCacheKey(req, unresolvedCanonical, unresolvedPkg)
	if aliasKey == "" || unresolvedKey == "" {
		t.Fatalf("alias/unresolved keys should not be empty: alias=%q unresolved=%q", aliasKey, unresolvedKey)
	}
	if aliasKey != unresolvedKey {
		t.Fatalf("alias key mismatch: alias=%s unresolved=%s", aliasKey, unresolvedKey)
	}

	got := reader.Resolve(req, unresolvedCanonical, unresolvedPkg)
	if readerInner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", readerInner.calls)
	}
	if got == nil || got.CategoryID != 2486 {
		t.Fatalf("category resolution = %+v, want remembered category 2486", got)
	}
	if got.Cache == nil || got.Cache.Source != "manual_cache" {
		t.Fatalf("category cache metadata = %+v, want manual cache hit", got.Cache)
	}
}

func TestCachedCategoryResolverDoesNotCreateUnresolvedAliasWithoutStableIdentifiers(t *testing.T) {
	store := newResolutionCacheTestStore(t)
	inner := &countingCategoryResolver{
		out: &CategoryResolution{Status: "resolved", Source: "inner", CategoryID: 9999},
	}
	writer := NewCachedCategoryResolver(inner, store).(CategoryResolutionCache)
	req := &BuildRequest{SheinStoreID: 42}
	resolvedCanonical := &canonical.Product{
		Title:        "Generic Wall Decor",
		CategoryPath: []string{"Home", "Decor"},
	}
	resolvedPkg := &Package{
		SpuName:      "Generic Wall Decor",
		CategoryPath: []string{"家居&生活", "家居装饰"},
	}
	writer.RememberCategoryResolution(req, resolvedCanonical, resolvedPkg, &CategoryResolution{
		Status:      "resolved",
		Source:      "manual",
		CategoryID:  2486,
		MatchedPath: []string{"家居&生活", "家居装饰"},
	})

	readerInner := &countingCategoryResolver{
		out: &CategoryResolution{Status: "resolved", Source: "inner", CategoryID: 9999},
	}
	reader := NewCachedCategoryResolver(readerInner, store)
	unresolvedCanonical := &canonical.Product{Title: "Generic Wall Decor"}
	unresolvedPkg := &Package{SpuName: "Generic Wall Decor"}

	got := reader.Resolve(req, unresolvedCanonical, unresolvedPkg)
	if readerInner.calls != 1 {
		t.Fatalf("inner calls = %d, want 1", readerInner.calls)
	}
	if got == nil || got.CategoryID != 9999 {
		t.Fatalf("category resolution = %+v, want live resolver result", got)
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

func TestCategoryResolverCacheKeyIgnoresResolvedSheinCategoryPathWhenSourcePathMissing(t *testing.T) {
	req := &BuildRequest{SheinStoreID: 42}
	canonical := &canonical.Product{
		Title: "护照保护套",
		Variants: []canonical.Variant{
			{
				SKU: "JJ0529207001-B8E25941",
				Attributes: map[string]canonical.Attribute{
					"source_sds_sku": {Value: "JJ0529207001"},
				},
			},
		},
	}
	before := &Package{
		SpuName: "护照保护套",
		ProductAttributes: []common.Attribute{
			{Name: "source_sds_sku", Value: "JJ0529207001"},
		},
	}
	after := &Package{
		SpuName:       "护照保护套",
		CategoryPath:  []string{"箱包", "旅行箱包&配件用品", "旅行配件&用品", "护照夹"},
		CategoryName:  "护照夹",
		ProductNameEn: "Classic Academy Travel Passport Holder",
		ProductAttributes: []common.Attribute{
			{Name: "source_sds_sku", Value: "JJ0529207001"},
		},
	}

	beforeKey := categoryResolverCacheKey(req, canonical, before)
	afterKey := categoryResolverCacheKey(req, canonical, after)
	if beforeKey == "" || afterKey == "" {
		t.Fatalf("cache keys should not be empty: before=%q after=%q", beforeKey, afterKey)
	}
	if beforeKey != afterKey {
		t.Fatalf("category cache key drifted after resolved SHEIN category path was applied: before=%s after=%s", beforeKey, afterKey)
	}
}

func TestCategoryResolverCacheKeyMatchesLegacyParentAndVariantSDSIdentity(t *testing.T) {
	req := &BuildRequest{SheinStoreID: 42}
	legacyCanonical := &canonical.Product{
		Title:        "40oz大容量汽车杯（包邮仅限美国直发）",
		CategoryPath: []string{"美国本地直发", "杯具用品", "汽车杯"},
		Attributes: map[string]canonical.Attribute{
			"product_sku": {Value: "MG8009019"},
			"variant_sku": {Value: "MG8009019002"},
		},
		Variants: []canonical.Variant{{
			SKU: "MG8009019002-LEGACY",
			Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8009019002"},
			},
		}},
	}
	legacyPkg := &Package{
		SpuName:      "40oz大容量汽车杯（包邮仅限美国直发）",
		CategoryPath: []string{"美国本地直发", "杯具用品", "汽车杯"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8009019"},
			{Name: "variant_sku", Value: "MG8009019002"},
		},
	}

	currentCanonical := &canonical.Product{
		Title: "40oz大容量汽车杯（包邮仅限美国直发）",
		Attributes: map[string]canonical.Attribute{
			"variant_sku": {Value: "MG8009019002"},
		},
		Variants: []canonical.Variant{{
			SKU: "MG8009019002-NEW",
			Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8009019002"},
			},
		}},
	}
	currentPkg := &Package{
		SpuName: "40oz大容量汽车杯（包邮仅限美国直发）",
		ProductAttributes: []common.Attribute{
			{Name: "variant_sku", Value: "MG8009019002"},
			{Name: "source_sds_sku", Value: "MG8009019002"},
		},
	}

	legacyAliasKey := categoryResolverUnresolvedAliasKey(req, legacyCanonical, legacyPkg)
	currentKey := categoryResolverCacheKey(req, currentCanonical, currentPkg)
	if legacyAliasKey == "" || currentKey == "" {
		t.Fatalf("cache keys should not be empty: legacy=%q current=%q", legacyAliasKey, currentKey)
	}
	if legacyAliasKey != currentKey {
		t.Fatalf("category cache key drifted between legacy parent/variant identity and current variant-only identity: legacy=%s current=%s", legacyAliasKey, currentKey)
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

func TestCachedAttributeResolverRejectsManualResolutionWithStaleTemplateAttributes(t *testing.T) {
	valueID := 526
	resolver := NewCachedAttributeResolver(NewAttributeResolver(stubAttributeAPI{
		templates: &sheinattribute.AttributeTemplateInfo{
			Data: []sheinattribute.AttributeTemplate{{
				AttributeInfos: []sheinattribute.AttributeInfo{{
					AttributeID:     160,
					AttributeNameEn: "Material",
					AttributeType:   4,
					AttributeStatus: 3,
					AttributeValueInfoList: []sheinattribute.AttributeValue{{
						AttributeValueID: 526,
						AttributeValueEn: "Polyester",
					}},
				}},
			}},
		},
	}, nil))
	cache := resolver.(AttributeResolutionCache)
	req := &BuildRequest{SheinStoreID: 42}
	pkg := &Package{
		CategoryID:     10484,
		CategoryIDList: []int{2866, 4359, 2868, 10484},
		ProductAttributes: []common.Attribute{
			{Name: "Material", Value: "Polyester"},
		},
		RequestDraft: &RequestDraft{
			SKCList: []SKCRequestDraft{{
				SKUList: []SKUDraft{{
					SupplierSKU: "XB0604115001",
					Length:      "15",
					Width:       "13",
					Height:      "3",
				}},
			}},
		},
	}
	cache.RememberAttributeResolution(req, nil, pkg, &AttributeResolution{
		Status:        "resolved",
		Source:        "manual",
		CategoryID:    10484,
		TemplateCount: 1,
		ResolvedCount: 4,
		ResolvedAttributes: []ResolvedAttribute{
			{Name: "Material", Value: "Polyester", AttributeID: 160, AttributeValueID: &valueID},
			{Name: "Width (cm)", Value: "13", AttributeID: 118, AttributeExtraValue: "13"},
			{Name: "Length (cm)", Value: "15", AttributeID: 55, AttributeExtraValue: "15"},
			{Name: "Height (cm)", Value: "3", AttributeID: 48, AttributeExtraValue: "3"},
		},
	})

	next := resolver.Resolve(req, nil, pkg)
	for _, attr := range next.ResolvedAttributes {
		switch attr.AttributeID {
		case 48, 55, 118:
			t.Fatalf("stale cached attribute still reused: %#v", attr)
		}
	}
	if len(next.ResolvedAttributes) != 1 || next.ResolvedAttributes[0].AttributeID != 160 {
		t.Fatalf("resolved attributes = %#v, want live Material-only resolution", next.ResolvedAttributes)
	}
}

func TestAttributeResolutionMatchesTemplatesRejectsMissingNewRequiredAttribute(t *testing.T) {
	materialValueID := 526
	templates := &sheinattribute.AttributeTemplateInfo{
		Data: []sheinattribute.AttributeTemplate{{
			AttributeInfos: []sheinattribute.AttributeInfo{
				{
					AttributeID:     160,
					AttributeNameEn: "Material",
					AttributeStatus: 3,
					AttributeValueInfoList: []sheinattribute.AttributeValue{{
						AttributeValueID: materialValueID,
						AttributeValueEn: "Polyester",
					}},
				},
				{
					AttributeID:     1000462,
					AttributeNameEn: "Hazardous materials classification",
					AttributeStatus: 3,
				},
			},
		}},
	}
	resolution := &AttributeResolution{
		ResolvedAttributes: []ResolvedAttribute{{
			Name:             "Material",
			Value:            "Polyester",
			AttributeID:      160,
			AttributeValueID: &materialValueID,
		}},
	}

	fresh, reason := attributeResolutionMatchesTemplates(resolution, templates)
	if fresh {
		t.Fatal("fresh = true, want cached resolution rejected when a new required attribute is missing")
	}
	if reason == "" {
		t.Fatal("reason is empty, want missing required attribute detail")
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

func TestAttributeResolverCacheKeyMatchesLegacyParentAndVariantSDSIdentity(t *testing.T) {
	req := &BuildRequest{SheinStoreID: 42}
	legacyPkg := &Package{
		CategoryID:     3221,
		CategoryIDList: []int{2030, 1946, 3213, 3221},
		CategoryPath:   []string{"美国本地直发", "杯具用品", "汽车杯"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8009019"},
			{Name: "sku", Value: "MG8009019"},
			{Name: "variant_sku", Value: "MG8009019002"},
			{Name: "material", Value: "304不锈钢"},
			{Name: "product_model", Value: "MG8009019002"},
		},
	}
	currentPkg := &Package{
		CategoryID:     3221,
		CategoryIDList: []int{2030, 1946, 3213, 3221},
		CategoryPath:   []string{"美国本地直发", "杯具用品", "汽车杯"},
		ProductAttributes: []common.Attribute{
			{Name: "product_sku", Value: "MG8009019"},
			{Name: "sku", Value: "MG8009019"},
			{Name: "variant_sku", Value: "MG8009019002"},
			{Name: "material", Value: "304不锈钢"},
			{Name: "product_model", Value: "MG8009019002"},
		},
	}

	legacyKey := attributeResolverCacheKey(req, nil, legacyPkg)
	currentKey := attributeResolverCacheKey(req, nil, currentPkg)
	if legacyKey == "" || currentKey == "" {
		t.Fatalf("cache keys should not be empty: legacy=%q current=%q", legacyKey, currentKey)
	}
	if legacyKey != currentKey {
		t.Fatalf("attribute cache key drifted between legacy parent/variant identity and current variant-only identity: legacy=%s current=%s", legacyKey, currentKey)
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
	if first.Cache.HitSource != ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("first hit source = %q, want %q", first.Cache.HitSource, ResolutionCacheHitSourcePersistentManualCache)
	}
	if second.Cache == nil || second.Cache.Source != "manual_cache" {
		t.Fatalf("second cache metadata = %#v, want manual memory hit", second.Cache)
	}
	if second.Cache.HitSource != ResolutionCacheHitSourceMemoryCache {
		t.Fatalf("second hit source = %q, want %q", second.Cache.HitSource, ResolutionCacheHitSourceMemoryCache)
	}
	if got := second.ResolvedAttributes[0].AttributeValueID; got == nil || *got != valueID {
		t.Fatalf("attribute value id = %v, want %d", got, valueID)
	}
}

func TestCachedAttributeResolverMigratesLegacyVariantOnlySDSCache(t *testing.T) {
	valueID := 2001
	store := newResolutionCacheTestStore(t)
	req := &BuildRequest{SheinStoreID: 42}
	pkg := &Package{
		CategoryID:     1764,
		CategoryIDList: []int{3294, 2041, 1764},
		CategoryPath:   []string{"箱包", "女士包包", "女士单肩包"},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "XB0601059"},
			{Name: "product_sku", Value: "XB0601059"},
			{Name: "variant_sku", Value: "XB0601059001"},
			{Name: "material", Value: "polyester"},
		},
	}
	legacyKey := attributeResolverLegacyVariantOnlyCacheKeyForTest(req, pkg, "XB0601059001")
	currentKey := attributeResolverCacheKey(req, nil, pkg)
	if legacyKey == "" || currentKey == "" || legacyKey == currentKey {
		t.Fatalf("expected distinct legacy/current keys: legacy=%q current=%q", legacyKey, currentKey)
	}
	if err := store.SaveResolutionCache(context.Background(), &SheinResolutionCacheEntry{
		StoreID:        "42",
		CacheKind:      ResolutionCacheKindAttribute,
		CacheKey:       legacyKey,
		ShortKey:       shortResolutionCacheKey(legacyKey),
		Source:         "manual_cache",
		Manual:         true,
		ResolutionJSON: `{"status":"resolved","category_id":1764,"template_count":1,"resolved_count":1,"resolved_attributes":[{"name":"Material","value":"Polyester","attribute_id":160,"attribute_value_id":2001}]}`,
	}); err != nil {
		t.Fatalf("seed legacy cache: %v", err)
	}
	inner := &countingAttributeResolver{
		out: &AttributeResolution{Status: "partial", CategoryID: 1764, TemplateCount: 1},
	}
	resolver := NewCachedAttributeResolver(inner, store)

	first := resolver.Resolve(req, nil, pkg)
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if first.Cache == nil || first.Cache.HitSource != ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("first cache metadata = %#v, want persistent manual cache", first.Cache)
	}
	if got := first.ResolvedAttributes[0].AttributeValueID; got == nil || *got != valueID {
		t.Fatalf("attribute value id = %v, want %d", got, valueID)
	}
	migrated, err := store.GetResolutionCache(context.Background(), ResolutionCacheKindAttribute, "42", currentKey)
	if err != nil {
		t.Fatalf("get migrated cache: %v", err)
	}
	if migrated == nil || !migrated.Manual || migrated.Source != "manual_cache" {
		t.Fatalf("migrated cache = %#v, want current-key manual cache", migrated)
	}
}

func TestCachedAttributeResolverReusesManualCacheWhenAttributeInputsShrink(t *testing.T) {
	valueID := 1009028
	store := newResolutionCacheTestStore(t)
	req := &BuildRequest{SheinStoreID: 870}
	cachedPkg := &Package{
		CategoryID:     4503,
		CategoryIDList: []int{3294, 4502, 13597, 4503},
		CategoryPath:   []string{"箱包", "功能包", "运动包", "健身包"},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "XB0606006"},
			{Name: "product_sku", Value: "XB0606006"},
			{Name: "variant_sku", Value: "XB0606006001"},
			{Name: "variant_color", Value: "white"},
			{Name: "variant_size", Value: "One size"},
			{Name: "material", Value: "Oxford"},
			{Name: "features", Value: "High-capacity"},
		},
	}
	currentPkg := &Package{
		CategoryID:     4503,
		CategoryIDList: []int{3294, 4502, 13597, 4503},
		CategoryPath:   []string{"箱包", "功能包", "运动包", "健身包"},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "XB0606006"},
			{Name: "product_sku", Value: "XB0606006"},
			{Name: "variant_sku", Value: "XB0606006001"},
			{Name: "variant_color", Value: "white"},
			{Name: "variant_size", Value: "One size"},
		},
	}
	cachedKey := attributeResolverCacheKey(req, nil, cachedPkg)
	currentKey := attributeResolverCacheKey(req, nil, currentPkg)
	if cachedKey == "" || currentKey == "" || cachedKey == currentKey {
		t.Fatalf("expected distinct cache keys after attribute input shrink: cached=%q current=%q", cachedKey, currentKey)
	}
	if buildResolutionCacheSourceIdentity(ResolutionCacheKindAttribute, nil, cachedPkg) != buildResolutionCacheSourceIdentity(ResolutionCacheKindAttribute, nil, currentPkg) {
		t.Fatalf("expected source identity to stay stable across attribute input shrink")
	}
	if err := store.SaveResolutionCache(context.Background(), buildResolutionCacheEntry(
		ResolutionCacheKindAttribute,
		req,
		nil,
		cachedPkg,
		cachedKey,
		&AttributeResolution{
			Status:        "resolved",
			Source:        "manual_review",
			CategoryID:    4503,
			TemplateCount: 1,
			ResolvedCount: 1,
			ResolvedAttributes: []ResolvedAttribute{{
				Name:             "Features",
				Value:            "High-capacity",
				AttributeID:      217,
				AttributeValueID: &valueID,
				Required:         true,
			}},
		},
		true,
	)); err != nil {
		t.Fatalf("seed source-identity cache: %v", err)
	}
	inner := &countingAttributeResolver{
		out: &AttributeResolution{Status: "partial", CategoryID: 4503, TemplateCount: 1},
	}
	resolver := NewCachedAttributeResolver(inner, store)

	got := resolver.Resolve(req, nil, currentPkg)
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if got.Cache == nil || got.Cache.HitSource != ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("cache metadata = %#v, want persistent manual cache", got.Cache)
	}
	if got.Cache.CacheKey != currentKey {
		t.Fatalf("cache key = %q, want migrated current key %q", got.Cache.CacheKey, currentKey)
	}
	if got.ResolvedCount != 1 || len(got.ResolvedAttributes) != 1 {
		t.Fatalf("resolution = %#v, want reused resolved attributes", got)
	}
	migrated, err := store.GetResolutionCache(context.Background(), ResolutionCacheKindAttribute, "870", currentKey)
	if err != nil {
		t.Fatalf("get migrated cache: %v", err)
	}
	if migrated == nil || !migrated.Manual || migrated.Source != "manual_cache" {
		t.Fatalf("migrated cache = %#v, want manual current-key cache", migrated)
	}
}

func TestCachedAttributeResolverReusesManualCacheWhenSourceCategoryPathDrifts(t *testing.T) {
	valueID := 1007420
	store := newResolutionCacheTestStore(t)
	req := &BuildRequest{SheinStoreID: 1043}
	cachedCanonical := &canonical.Product{
		CategoryPath: []string{"美国本地直发", "连衣裙", "连衣裙"},
		Attributes: map[string]canonical.Attribute{
			"sku":         {Value: "MG8038016"},
			"product_sku": {Value: "MG8038016"},
		},
		Variants: []canonical.Variant{
			{Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8038016001"},
				"Color":          {Value: "black"},
				"Size":           {Value: "S"},
			}},
		},
	}
	currentCanonical := &canonical.Product{
		Attributes: map[string]canonical.Attribute{
			"sku":         {Value: "MG8038016"},
			"product_sku": {Value: "MG8038016"},
		},
		Variants: []canonical.Variant{
			{Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8038016001"},
				"Color":          {Value: "black"},
				"Size":           {Value: "S"},
			}},
		},
	}
	cachedPkg := &Package{
		CategoryID:     1727,
		CategoryIDList: []int{4478, 2028, 12716, 1727},
		CategoryPath:   []string{"女士服装", "常规女士服装", "女士连衣裙", "女士短连衣裙"},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8038016"},
			{Name: "product_sku", Value: "MG8038016"},
			{Name: "variant_sku", Value: "MG8038016001"},
			{Name: "material", Value: "Knitted Fabric"},
		},
	}
	currentPkg := &Package{
		CategoryID:     1727,
		CategoryIDList: []int{4478, 2028, 12716, 1727},
		CategoryPath:   []string{"女士服装", "常规女士服装", "女士连衣裙", "女士短连衣裙"},
		ProductAttributes: []common.Attribute{
			{Name: "sku", Value: "MG8038016"},
			{Name: "product_sku", Value: "MG8038016"},
			{Name: "variant_sku", Value: "MG8038016001"},
		},
	}
	if buildResolutionCacheSourceIdentity(ResolutionCacheKindAttribute, cachedCanonical, cachedPkg) == buildResolutionCacheSourceIdentity(ResolutionCacheKindAttribute, currentCanonical, currentPkg) {
		t.Fatalf("expected source identity to differ after source category path drift")
	}
	cachedKey := attributeResolverCacheKey(req, cachedCanonical, cachedPkg)
	currentKey := attributeResolverCacheKey(req, currentCanonical, currentPkg)
	if cachedKey == "" || currentKey == "" || cachedKey == currentKey {
		t.Fatalf("expected distinct keys after source category path drift: cached=%q current=%q", cachedKey, currentKey)
	}
	if err := store.SaveResolutionCache(context.Background(), buildResolutionCacheEntry(
		ResolutionCacheKindAttribute,
		req,
		cachedCanonical,
		cachedPkg,
		cachedKey,
		&AttributeResolution{
			Status:        "resolved",
			Source:        "manual_review",
			CategoryID:    1727,
			TemplateCount: 1,
			ResolvedCount: 1,
			ResolvedAttributes: []ResolvedAttribute{{
				Name:             "Material",
				Value:            "Knitted Fabric",
				AttributeID:      160,
				AttributeValueID: &valueID,
				Required:         true,
			}},
		},
		true,
	)); err != nil {
		t.Fatalf("seed source category path drift cache: %v", err)
	}
	inner := &countingAttributeResolver{
		out: &AttributeResolution{Status: "partial", CategoryID: 1727, TemplateCount: 1},
	}
	resolver := NewCachedAttributeResolver(inner, store)

	got := resolver.Resolve(req, currentCanonical, currentPkg)
	if inner.calls != 0 {
		t.Fatalf("inner calls = %d, want 0", inner.calls)
	}
	if got.Cache == nil || got.Cache.HitSource != ResolutionCacheHitSourcePersistentManualCache {
		t.Fatalf("cache metadata = %#v, want persistent manual cache", got.Cache)
	}
	if got.Cache.CacheKey != currentKey {
		t.Fatalf("cache key = %q, want migrated current key %q", got.Cache.CacheKey, currentKey)
	}
}

func attributeResolverLegacyVariantOnlyCacheKeyForTest(req *BuildRequest, pkg *Package, variantSKU string) string {
	if pkg == nil || categoryID(pkg) == 0 {
		return ""
	}
	payload := map[string]any{
		"version":               14,
		"store_id":              sheinStoreID(req),
		"category_id":           categoryID(pkg),
		"category_id_list":      append([]int(nil), pkg.CategoryIDList...),
		"category_path":         normalizedSourceCategoryPath(nil, pkg),
		"product_identity":      normalizeStableIdentity([]string{variantSKU}),
		"product_attributes":    normalizedAttributeInputs(legacyVariantOnlyAttributeInputs(pkg.ProductAttributes)),
		"supplemental_attrs":    normalizedStringMapInputs(pkg.Attributes),
		"structured_attr_hints": normalizedStructuredAttributeHints(legacyVariantOnlyAttributeInputs(pkg.ProductAttributes)),
	}
	return hashCachePayload(payload)
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
