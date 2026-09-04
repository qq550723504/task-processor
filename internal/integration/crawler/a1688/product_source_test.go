package a1688

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	crawlermodel "task-processor/internal/crawler/alibaba1688/model"
	"task-processor/internal/product/sourcing"
)

func TestAlibaba1688SourceEnvelopeMapsProductFacts(t *testing.T) {
	product := &Alibaba1688ProductSnapshot{
		ID:               "123",
		Title:            "Canvas Tote Bag",
		URL:              "https://detail.1688.com/offer/123.html?foo=bar",
		MainImage:        " https://img.example/main.jpg ",
		Images:           []string{"https://img.example/main.jpg", "https://img.example/gallery.jpg"},
		MinPrice:         8.5,
		MaxPrice:         12.0,
		Currency:         "CNY",
		MinOrderQuantity: 2,
		Unit:             "件",
		Category:         "Bags > Lunch Bags",
		Brand:            "Factory Brand",
		Keywords:         []string{" tote ", "canvas"},
		IsCustomized:     true,
		SalesVolume:      100,
		ReviewCount:      8,
		Rating:           4.8,
		Supplier: Alibaba1688SupplierSnapshot{
			ID:              "supplier-1",
			Name:            "Supplier One",
			CompanyName:     "Supplier Co",
			Location:        "Guangdong",
			ShopURL:         "https://shop.1688.com/supplier-1",
			CardType:        "factory",
			YearsInBusiness: 5,
			Rating:          4.7,
			ResponseRate:    98.5,
			IsGoldSupplier:  true,
			IsVerified:      true,
		},
		Specifications: []Alibaba1688SpecificationSnapshot{{Name: "Material", Value: "Canvas"}},
		ProductDetails: []Alibaba1688ProductDetailSnapshot{{Content: "Durable bag", Images: []string{"https://img.example/detail.jpg"}}},
		PackInfo: &Alibaba1688PackInfoSnapshot{
			PackageType:   "box",
			Weight:        500,
			PackageImages: []string{"https://img.example/pack.jpg"},
		},
		Variants: []Alibaba1688VariantSnapshot{{
			Name:       "Blue / M",
			Image:      "https://img.example/variant.jpg",
			Stock:      20,
			Price:      9.9,
			Attributes: map[string]any{"Color": "Blue", "Size": "M"},
		}},
		Videos:   []Alibaba1688VideoSnapshot{{VideoURL: "https://video.example/1.mp4", CoverURL: "https://img.example/video-cover.jpg"}},
		Shipping: Alibaba1688ShippingSnapshot{ShippingFrom: "Guangdong", ProcessingTime: "3 days"},
	}
	if field := reflect.ValueOf(product).Elem().FieldByName("PriceRangeCount"); field.IsValid() && field.CanSet() {
		field.SetInt(2)
	}
	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Request:     Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/123.html?spm=test", StoreID: 9},
		Product:     product,
		RawSnapshot: "raw-1688-1",
		SourceRunID: "run-1",
		RequestID:   "request-1",
	})

	if envelope.Identity.SourceType != sourcing.SourceTypeCrawler {
		t.Fatalf("SourceType = %q, want crawler", envelope.Identity.SourceType)
	}
	if envelope.Identity.SourcePlatform != Alibaba1688SourcePlatform {
		t.Fatalf("SourcePlatform = %q, want 1688", envelope.Identity.SourcePlatform)
	}
	if envelope.Identity.SourceID != "123" {
		t.Fatalf("SourceID = %q, want 123", envelope.Identity.SourceID)
	}
	if got := envelope.Identity.Key(); got != "1688:cn:123:9" {
		t.Fatalf("Key() = %q, want legacy key with store", got)
	}
	if got := envelope.Identity.SourceKey(); got != "crawler:1688:123" {
		t.Fatalf("SourceKey() = %q, want source key", got)
	}
	if envelope.RawReference.ReferenceType != alibaba1688SourceReferenceType || envelope.RawReference.ReferenceID != "123" {
		t.Fatalf("RawReference = %+v, want 1688 product reference", envelope.RawReference)
	}
	if envelope.RawReference.URL != "https://detail.1688.com/offer/123.html" {
		t.Fatalf("RawReference.URL = %q, want normalized URL without query", envelope.RawReference.URL)
	}
	if envelope.RawReference.SnapshotID != "" {
		t.Fatalf("RawReference.SnapshotID = %q, want raw evidence excluded from canonical snapshot identity", envelope.RawReference.SnapshotID)
	}
	if envelope.ProductCandidate.Title != "Canvas Tote Bag" || envelope.ProductCandidate.Brand != "Factory Brand" {
		t.Fatalf("sourcing.ProductCandidate = %+v, want title and brand", envelope.ProductCandidate)
	}
	if want := []string{"Bags", "Lunch Bags"}; !reflect.DeepEqual(envelope.ProductCandidate.CategoryPath, want) {
		t.Fatalf("category path = %#v, want %#v", envelope.ProductCandidate.CategoryPath, want)
	}
	snapshot, err := sourcing.ToSnapshot(envelope)
	if err != nil {
		t.Fatalf("sourcing.ToSnapshot() error = %v", err)
	}
	if want := []string{"Bags", "Lunch Bags"}; !reflect.DeepEqual(snapshot.CategoryPath, want) {
		t.Fatalf("catalog snapshot category path = %#v, want %#v", snapshot.CategoryPath, want)
	}
	if envelope.ProductCandidate.Description != "Durable bag" {
		t.Fatalf("Description = %q, want product detail content", envelope.ProductCandidate.Description)
	}
	if envelope.ProductCandidate.Attributes["spec:Material"] != "Canvas" {
		t.Fatalf("Material spec = %q, want Canvas", envelope.ProductCandidate.Attributes["spec:Material"])
	}
	if envelope.ProductCandidate.Attributes["keywords"] != "tote,canvas" {
		t.Fatalf("keywords = %q, want normalized keywords", envelope.ProductCandidate.Attributes["keywords"])
	}
	if len(envelope.ProductCandidate.Variants) != 1 {
		t.Fatalf("variants = %d, want 1", len(envelope.ProductCandidate.Variants))
	}
	variant := envelope.ProductCandidate.Variants[0]
	if variant.Attributes["Color"] != "Blue" || variant.Attributes["price"] != "9.9" || variant.Attributes["stock"] != "20" {
		t.Fatalf("variant attributes = %+v, want color price and stock", variant.Attributes)
	}
	if variant.Price != 9.9 || variant.Stock != 20 || variant.Currency != "CNY" {
		t.Fatalf("typed variant commerce facts = %+v, want CNY/9.9/20", variant)
	}
	if len(envelope.AssetCandidates) != 7 {
		t.Fatalf("assets = %d, want main/gallery/detail/variant/package/video cover/video", len(envelope.AssetCandidates))
	}
	if envelope.AssetCandidates[0].Role != alibaba1688ImageRolePrimary {
		t.Fatalf("first asset role = %q, want primary", envelope.AssetCandidates[0].Role)
	}
	if envelope.SupplierOrCostFacts.SupplierID != "supplier-1" || envelope.SupplierOrCostFacts.Price != "8.5" {
		t.Fatalf("sourcing.SupplierOrCostFacts = %+v, want supplier and min price", envelope.SupplierOrCostFacts)
	}
	if envelope.SupplierOrCostFacts.Facts["is_gold_supplier"] != "true" {
		t.Fatalf("is_gold_supplier = %q, want true", envelope.SupplierOrCostFacts.Facts["is_gold_supplier"])
	}
	if _, ok := envelope.SupplierOrCostFacts.Facts["price_range_count"]; ok {
		t.Fatalf("price_range_count fact = %q, want omitted", envelope.SupplierOrCostFacts.Facts["price_range_count"])
	}
	if len(envelope.Warnings) != 0 {
		t.Fatalf("Warnings = %+v, want none", envelope.Warnings)
	}
}

func TestAlibaba1688SourceEnvelopeBindsLegacyCaptureEvidence(t *testing.T) {
	crawledAt := time.Date(2026, time.August, 31, 10, 11, 12, 123, time.FixedZone("CST", 8*60*60))
	updatedAt := crawledAt.Add(2 * time.Hour)
	product := SnapshotFromLegacyProduct(&crawlermodel.Product1688{
		ID:        "123",
		URL:       "https://detail.1688.com/offer/123.html",
		CrawledAt: crawledAt,
		UpdatedAt: updatedAt,
	})

	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Product:     product,
		RawSnapshot: "raw-1688-1",
	})

	if !envelope.RawReference.CapturedAt.Equal(crawledAt.UTC()) {
		t.Fatalf("CapturedAt = %s, want CrawledAt %s", envelope.RawReference.CapturedAt, crawledAt.UTC())
	}
	if got := envelope.RawReference.Metadata["crawled_at"]; got != crawledAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("metadata[crawled_at] = %q, want UTC crawler timestamp", got)
	}
	if got := envelope.RawReference.Metadata["updated_at"]; got != updatedAt.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("metadata[updated_at] = %q, want UTC update timestamp", got)
	}
	if got := envelope.RawReference.Checksum; got != "sha256:ae61ffcfff451b2bca3eafa7ba0d7254095b8d3146d99f21d1323727051373c7" {
		t.Fatalf("Checksum = %q, want stable SHA-256 evidence checksum", got)
	}
}

func TestAlibaba1688SourceEnvelopeFallsBackToUpdatedAt(t *testing.T) {
	updatedAt := time.Date(2026, time.August, 31, 8, 0, 0, 0, time.UTC)
	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Product: SnapshotFromLegacyProduct(&crawlermodel.Product1688{
			ID:        "123",
			UpdatedAt: updatedAt,
		}),
	})

	if !envelope.RawReference.CapturedAt.Equal(updatedAt) {
		t.Fatalf("CapturedAt = %s, want UpdatedAt fallback %s", envelope.RawReference.CapturedAt, updatedAt)
	}
	if _, ok := envelope.RawReference.Metadata["crawled_at"]; ok {
		t.Fatalf("metadata = %+v, want no zero crawled_at", envelope.RawReference.Metadata)
	}
}

func TestAlibaba1688SourceEnvelopeFallsBackToRequestIdentityAndWarnings(t *testing.T) {
	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Request: Alibaba1688CrawlRequestInput{URL: "detail.1688.com/offer/456.html"},
		Product: &Alibaba1688ProductSnapshot{},
	})

	if envelope.Identity.SourceID != "456" {
		t.Fatalf("SourceID = %q, want request offer id", envelope.Identity.SourceID)
	}
	codes := map[string]bool{}
	for _, warning := range envelope.Warnings {
		codes[warning.Code] = true
	}
	for _, want := range []string{"missing_title", "missing_assets", "missing_cost"} {
		if !codes[want] {
			t.Fatalf("warning codes = %+v, missing %s", codes, want)
		}
	}
}

func TestAlibaba1688SourceEnvelopeDisambiguatesDuplicateVariantAttributes(t *testing.T) {
	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Request: Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/123.html"},
		Product: &Alibaba1688ProductSnapshot{
			Variants: []Alibaba1688VariantSnapshot{
				{Attributes: map[string]any{"Color": "Blue"}},
				{Attributes: map[string]any{"Color": "Blue"}},
			},
		},
	})

	if len(envelope.ProductCandidate.Variants) != 2 {
		t.Fatalf("variants = %d, want 2", len(envelope.ProductCandidate.Variants))
	}
	first, second := envelope.ProductCandidate.Variants[0], envelope.ProductCandidate.Variants[1]
	if first.SourceID == second.SourceID || first.SKU == second.SKU {
		t.Fatalf("duplicate variant identities: first=%+v second=%+v", first, second)
	}
}

func TestAlibaba1688SourceEnvelopeHandlesMissingProductAndError(t *testing.T) {
	envelope := Alibaba1688SourceEnvelope(Alibaba1688SourceEnvelopeInput{
		Request: Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/789.html"},
		Error:   errors.New("crawler failed"),
	})

	if envelope.Identity.SourceID != "789" {
		t.Fatalf("SourceID = %q, want request identity", envelope.Identity.SourceID)
	}
	codes := map[string]bool{}
	for _, warning := range envelope.Warnings {
		codes[warning.Code] = true
	}
	if !codes["missing_product"] || !codes["source_error"] {
		t.Fatalf("warning codes = %+v, want missing_product and source_error", codes)
	}
}

func TestAlibaba1688CrawlRequestJSONOmitsPublicAccountSelector(t *testing.T) {
	payload, err := json.Marshal(Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/1.html"})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(payload) != `{"url":"https://detail.1688.com/offer/1.html"}` {
		t.Fatalf("public request JSON = %s, want account selector omitted", payload)
	}
}

func TestAlibaba1688CrawlRequestJSONIncludesAccountSelectorForAssistedMode(t *testing.T) {
	payload, err := json.Marshal(Alibaba1688CrawlRequestInput{URL: "https://detail.1688.com/offer/1.html", AccountID: 42})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(payload) != `{"url":"https://detail.1688.com/offer/1.html","account_id":42}` {
		t.Fatalf("assisted request JSON = %s, want account selector", payload)
	}
}

func TestAlibaba1688SourceRequestUsesOfferIDIdentity(t *testing.T) {
	got := Alibaba1688SourceRequest(Alibaba1688CrawlRequestInput{
		URL:       " HTTPS://DETAIL.1688.COM/offer/123456789.html?spm=abc#sku ",
		AccountID: 42,
	}).Identity()

	if got.Platform != "1688" || got.Region != "cn" || got.ProductID != "123456789" || got.StoreID != 0 {
		t.Fatalf("Identity() = %+v, want 1688 cn offer identity", got)
	}
	if key := got.Key(); key != "1688:cn:123456789" {
		t.Fatalf("Key() = %q, want 1688:cn:123456789", key)
	}
}

func TestAlibaba1688SourceRequestDoesNotProjectAccountIntoNeutralStoreIdentity(t *testing.T) {
	got := Alibaba1688SourceRequest(Alibaba1688CrawlRequestInput{
		URL:       "https://detail.1688.com/offer/123.html",
		AccountID: 3001,
	}).Identity()

	if got.StoreID != 0 {
		t.Fatalf("Identity().StoreID = %d, want neutral store id omitted", got.StoreID)
	}
}

func TestAlibaba1688SourceRequestFallsBackToCleanURL(t *testing.T) {
	got := Alibaba1688SourceRequest(Alibaba1688CrawlRequestInput{
		URL: "detail.1688.com/item/custom?foo=bar#frag",
	}).Identity()

	if got.ProductID != "https://detail.1688.com/item/custom" {
		t.Fatalf("ProductID fallback = %q, want cleaned URL", got.ProductID)
	}
}

func TestNormalizeAlibaba1688SourceResultAttachesIdentity(t *testing.T) {
	wantErr := errors.New("captcha")
	product := &Alibaba1688ProductSnapshot{ID: "123", Title: "sample"}

	got := NormalizeAlibaba1688SourceResult(Alibaba1688CrawlRequestInput{
		URL: "https://detail.1688.com/offer/123.html",
	}, product, wantErr)

	if got.Identity.Key() != "1688:cn:123" {
		t.Fatalf("Identity.Key() = %q, want 1688:cn:123", got.Identity.Key())
	}
	if got.Product != product {
		t.Fatalf("Product was not preserved")
	}
	if !errors.Is(got.Error, wantErr) {
		t.Fatalf("Error = %v, want %v", got.Error, wantErr)
	}
}

func TestNormalizeAlibaba1688BatchResultsAlignsShortResults(t *testing.T) {
	requests := []Alibaba1688CrawlRequestInput{
		{URL: "https://detail.1688.com/offer/1.html"},
		{URL: "https://detail.1688.com/offer/2.html"},
	}
	results := []Alibaba1688CrawlResultInput{
		{Product: &Alibaba1688ProductSnapshot{ID: "1", Title: "first"}},
	}

	got := NormalizeAlibaba1688BatchResults(requests, results)

	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Identity.Key() != "1688:cn:1" || got[0].Product.Title != "first" {
		t.Fatalf("got[0] = %+v, want first request result", got[0])
	}
	if got[1].Identity.Key() != "1688:cn:2" || got[1].Product != nil || got[1].Error != nil {
		t.Fatalf("got[1] = %+v, want empty result aligned to second request", got[1])
	}
}
