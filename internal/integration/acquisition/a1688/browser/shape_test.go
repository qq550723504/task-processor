package browser

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

func mustSourceForShape(t *testing.T) sourcing.AcquisitionSource {
	t.Helper()
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	return source
}

// buildContextPage renders a page whose window.context is the given data object.
func buildContextPage(data map[string]any) string {
	raw := map[string]any{"result": map[string]any{"data": data}}
	encoded, _ := json.Marshal(raw)
	return "<!doctype html><html><head><title>Shape</title></head><body><script>window.context = " +
		string(encoded) + ";</script></body></html>"
}

// A nonpositive or unparsable beginAmount must be normalized, because a "0"
// minimum quantity fails the acquisition during mapping.
func TestBrowserAcquireNormalizesNonpositiveMinimumQuantity(t *testing.T) {
	browser := fixtureBrowserPath(t)
	page := buildContextPage(map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Min qty bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
		"price": map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
			"tradeWithoutPromotion": map[string]any{"offerPriceRanges": []any{
				map[string]any{"price": "12.50", "beginAmount": 0},
			}},
		}}},
	})
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source := mustSourceForShape(t)
	evidence, err := client.Acquire(t.Context(), source)
	require.NoError(t, err, "a zero minimum quantity must not fail the acquisition")
	require.NotEmpty(t, evidence.PriceFacts)
	// The full mapping must succeed, which is what a "0" quantity would break.
	_, err = sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-minqty")
	require.NoError(t, err, "the normalized minimum quantity must pass mapping validation")
}

// An oversized price-range list must drop the whole set rather than publish a
// prefix of it.
func TestBrowserAcquireDropsOversizedPriceRanges(t *testing.T) {
	browser := fixtureBrowserPath(t)
	ranges := make([]any, 0, maxPriceFacts+5)
	for i := 0; i < maxPriceFacts+5; i++ {
		ranges = append(ranges, map[string]any{"price": "1.00", "beginAmount": 1})
	}
	page := buildContextPage(map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Many ranges bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
		"price": map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
			"tradeWithoutPromotion": map[string]any{"offerPriceRanges": ranges},
		}}},
	})
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Empty(t, evidence.PriceFacts, "a lossy price prefix must not be published")
	require.True(t, hasTruncation(evidence, "price_facts"), "the dropped price set must be reported")
}

// A custom-item page with a title and product data but no offer id must still be
// collected, using the canonical URL, exactly as a standard page is.
func TestBrowserAcquireRecoversCustomItemOfferIDFromURL(t *testing.T) {
	browser := fixtureBrowserPath(t)
	page := `<!doctype html><html><head><title>No id</title></head><body><script>
window.__INIT_DATA = {"data":{"main":{"data":{"title":"No id custom item",
  "propsList":[{"name":"material","value":"steel"}]}}}};
</script></body></html>`
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL + "/offer/981645030344.html"})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err, "a custom item without an inline offer id must still be collected")
	require.Equal(t, "981645030344", evidence.OfferID)
}

// When a block exposes both a partial skuModel and the complete nySkuModel, the
// complete one must win.
func TestBrowserAcquirePrefersBlockLocalNySkuModel(t *testing.T) {
	browser := fixtureBrowserPath(t)
	page := `<!doctype html><html><head><title>Prefer ny</title></head><body><script>
window.__INIT_DATA = {"data":{"main":{"data":{
  "offerId": 981645030344,
  "title": "Prefer ny item",
  "skuModel": {"skuProps":[{"prop":"size"}], "skuInfoMap":{"partial":{"skuId":7,"price":1.5}}},
  "nySkuModel": {"skuProps":[{"prop":"color"}], "skuInfoMap":{"red":{"skuId":99,"price":2.5}}}
}}}};
</script></body></html>`
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Len(t, evidence.Variants, 1)
	require.NotNil(t, evidence.Variants[0].SourceID)
	require.Equal(t, "99", *evidence.Variants[0].SourceID, "the complete nySkuModel must win over the partial skuModel")
}

func hasTruncation(e sourcing.AcquisitionEvidence, field string) bool {
	for _, w := range e.Warnings {
		if w.Code == "source_evidence_truncated" && w.Field == field {
			return true
		}
	}
	return false
}

// The price cap must be enforced in one shared place; a second inline guard is
// exactly the defect this change set out to remove.
func TestExtractorHasASingleSharedPriceGuard(t *testing.T) {
	script := extractScript()
	require.Equal(t, 1, strings.Count(script, "const pushPrice ="),
		"there must be exactly one price append helper")
	// No price reader may push directly any more.
	require.NotContains(t, script, "out.priceFacts.push({ amount: amount, currency: clip(r.currency",
		"price readers must go through pushPrice")
}

// A valid positive integer minimum quantity beyond the safe-integer range must
// be published exactly; parseInt would round it.
func TestBrowserAcquirePreservesExactLargeMinimumQuantity(t *testing.T) {
	browser := fixtureBrowserPath(t)
	page := buildContextPage(map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Big min qty bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
		"price": map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
			"tradeWithoutPromotion": map[string]any{"offerPriceRanges": []any{
				map[string]any{"price": "12.50", "beginAmount": "9007199254740993"},
			}},
		}}},
	})
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.NotEmpty(t, evidence.PriceFacts)
	require.NotNil(t, evidence.PriceFacts[0].MinQuantity)
	require.Equal(t, "9007199254740993", *evidence.PriceFacts[0].MinQuantity,
		"an exact positive digit string must not be rounded")
}

// A truthy but unusable nySkuModel must not shadow a complete skuModel.
func TestBrowserAcquireFallsThroughUnusableSkuModel(t *testing.T) {
	browser := fixtureBrowserPath(t)
	page := `<!doctype html><html><head><title>Fallback</title></head><body><script>
window.__INIT_DATA = {"data":{"main":{"data":{
  "offerId": 981645030344,
  "title": "Fallback item",
  "nySkuModel": {"note": "present but unusable"},
  "skuModel": {"skuProps":[{"prop":"color"}], "skuInfoMap":{"red":{"skuId":55,"price":4.5}}}
}}}};
</script></body></html>`
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Len(t, evidence.Variants, 1, "an unusable model must not shadow the complete fallback")
	require.NotNil(t, evidence.Variants[0].SourceID)
	require.Equal(t, "55", *evidence.Variants[0].SourceID)
}

// The same fall-through must apply to the global model chain.
func TestBrowserAcquireFallsThroughUnusableGlobalSkuModel(t *testing.T) {
	browser := fixtureBrowserPath(t)
	page := `<!doctype html><html><head><title>Global fallback</title></head><body><script>
window.__INIT_DATA = {
  "data": {"main": {"data": {"offerId": 981645030344, "title": "Global fallback item"}}},
  "globalData": {
    "nySkuModel": {},
    "skuModel": {"skuProps":[{"prop":"size"}], "skuInfoMap":{"l":{"skuId":88,"price":6.5}}}
  }
};
</script></body></html>`
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Len(t, evidence.Variants, 1, "an empty global model must not shadow the complete one")
	require.NotNil(t, evidence.Variants[0].SourceID)
	require.Equal(t, "88", *evidence.Variants[0].SourceID)
}

// Scanning must be bounded even when nothing is kept, so a page-created huge
// array cannot keep the evaluation busy until the deadline.
func TestBrowserAcquireBoundsPriceScanning(t *testing.T) {
	script := extractScript()
	require.Contains(t, script, "MAX_PRICE_SCAN",
		"price scanning must be bounded independently of how much is kept")
	require.NotContains(t, script, "for (const r of ranges) {\n        if (out.priceFacts.length",
		"no price loop may be bounded only by the output cap")
}

// Many individually small price arrays must not add up to an unbounded scan.
func TestBrowserAcquireBoundsAggregatePriceScan(t *testing.T) {
	browser := fixtureBrowserPath(t)
	// 40 separate blocks, each with 5 ranges: every array is far under any
	// per-array bound, but the aggregate is 200 entries.
	data := map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Many arrays bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
	}
	for i := 0; i < 40; i++ {
		ranges := []any{}
		for j := 0; j < 5; j++ {
			ranges = append(ranges, map[string]any{"price": "1.00", "beginAmount": 1})
		}
		data["blk"+itoa(i)] = map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
			"tradeWithoutPromotion": map[string]any{"offerPriceRanges": ranges},
		}}}
	}
	srv := serveFixture(t, buildContextPage(data))
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Empty(t, evidence.PriceFacts, "exceeding the extraction-wide scan budget must drop the price set")
	require.True(t, hasTruncation(evidence, "price_facts"))
}

// The scan budget must be extraction-wide, not per array.
func TestExtractorPriceScanBudgetIsExtractionWide(t *testing.T) {
	script := extractScript()
	require.Contains(t, script, "let priceScanned = 0", "the scan budget must be a single extraction-wide counter")
	require.Contains(t, script, "priceScanned + list.length > MAX_PRICE_SCAN",
		"the bound must compare against the accumulated count, not one array")
}

// A preferred SKU model that was found but dropped as truncated must stop
// candidate selection: falling through would publish a different variant set than
// the authoritative one.
func TestBrowserAcquireStopsAfterTruncatedSkuModel(t *testing.T) {
	browser := fixtureBrowserPath(t)
	// The authoritative model exceeds the variant cap; the fallback is complete
	// but must NOT be used.
	big := map[string]any{}
	for i := 0; i < maxVariants+3; i++ {
		big["c"+itoa(i)] = map[string]any{"skuId": i + 1, "price": 1.0}
	}
	page := `<!doctype html><html><head><title>Truncated primary</title></head><body><script>
window.__INIT_DATA = {
  "data": {"main": {"data": {"offerId": 981645030344, "title": "Truncated primary item"}}},
  "globalData": {
    "nySkuModel": {"skuProps":[{"prop":"color"}], "skuInfoMap": BIG},
    "skuModel": {"skuProps":[{"prop":"size"}], "skuInfoMap":{"fallback":{"skuId":777,"price":9.9}}}
  }
};
</script></body></html>`
	page = strings.Replace(page, "BIG", mustJSON(big), 1)
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Empty(t, evidence.Variants,
		"a truncated authoritative model must not be replaced by a lower-priority fallback")
}

// A noncanonical minimum quantity must be omitted, not coerced, so a page with
// "01" or an over-long digit string still publishes.
func TestBrowserAcquireOmitsNoncanonicalMinimumQuantity(t *testing.T) {
	browser := fixtureBrowserPath(t)
	for _, begin := range []any{"01", "123456789012345678901"} {
		page := buildContextPage(map[string]any{
			"productTitle": map[string]any{"fields": map[string]any{"title": "Odd min qty bottle"}},
			"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
				"tempModel": map[string]any{"offerId": 981645030344},
			}}},
			"price": map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
				"tradeWithoutPromotion": map[string]any{"offerPriceRanges": []any{
					map[string]any{"price": "12.50", "beginAmount": begin},
				}},
			}}},
		})
		srv := serveFixture(t, page)
		client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
		source := mustSourceForShape(t)
		evidence, err := client.Acquire(t.Context(), source)
		require.NoError(t, err)
		require.NotEmpty(t, evidence.PriceFacts)
		_, err = sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-minq")
		require.NoError(t, err, "a noncanonical minimum quantity must not fail the acquisition")
	}
}

func mustJSON(v any) string {
	encoded, _ := json.Marshal(v)
	return string(encoded)
}

// A truncated authoritative global model must stop the whole extraction: a later
// block with a usable model must not publish a lower-priority variant set.
func TestBrowserAcquireStopsExtractionAfterTruncatedGlobalModel(t *testing.T) {
	browser := fixtureBrowserPath(t)
	big := map[string]any{}
	for i := 0; i < maxVariants+3; i++ {
		big["c"+itoa(i)] = map[string]any{"skuId": i + 1, "price": 1.0}
	}
	page := `<!doctype html><html><head><title>Global truncated</title></head><body><script>
window.__INIT_DATA = {
  "data": {
    "main": {"data": {"offerId": 981645030344, "title": "Global truncated item",
      "skuModel": {"skuProps":[{"prop":"size"}], "skuInfoMap":{"blockFallback":{"skuId":777,"price":9.9}}}}}
  },
  "globalData": {
    "nySkuModel": {"skuProps":[{"prop":"color"}], "skuInfoMap": BIG}
  }
};
</script></body></html>`
	page = strings.Replace(page, "BIG", mustJSON(big), 1)
	srv := serveFixture(t, page)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Empty(t, evidence.Variants,
		"a truncated global model must stop the whole extraction, not just its own candidate list")
}

// The outer price-block loop must stop once the scan budget is spent.
func TestExtractorPriceOuterLoopStopsOnOverflow(t *testing.T) {
	script := extractScript()
	require.Contains(t, script, "if (priceScanned >= MAX_PRICE_SCAN || out.priceFacts.length >= CAP.priceFacts) {",
		"the outer block loop must break once the extraction-wide budget is spent")
}

// The truncation flag must be declared before any reader that consults it.
func TestVariantTruncationFlagIsDeclaredBeforeUse(t *testing.T) {
	script := extractScript()
	decl := strings.Index(script, "let variantTruncated = false;")
	use := strings.Index(script, "variantTruncated = true;")
	require.GreaterOrEqual(t, decl, 0, "the flag must exist")
	require.Greater(t, decl, -1)
	require.Less(t, decl, use, "the flag must be declared before any reader sets it")
}
