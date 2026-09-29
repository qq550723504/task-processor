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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL + "/offer/981645030344.html"})
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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
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
		client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
		source := mustSourceForShape(t)
		evidence, err := client.Acquire(t.Context(), source)
		require.NoError(t, err)
		require.NotEmpty(t, evidence.PriceFacts)
		_, err = sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-minq")
		require.NoError(t, err, "a noncanonical minimum quantity must not fail the acquisition")
	}
}

func binPathOf(t *testing.T) string { return fixtureBrowserPath(t) }

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
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Empty(t, evidence.Variants,
		"a truncated global model must stop the whole extraction, not just its own candidate list")
}

// Three different bounds, three different meanings, and only two of them may
// drop the price set.
func TestExtractorPriceOverflowSignalsAreDistinct(t *testing.T) {
	script := extractScript()

	// The scan budget is a work bound only: reaching it is not incompleteness.
	require.Contains(t, script, "const MAX_PRICE_SCAN = CAP.priceFacts * 4;")

	// scanPrice refusing an actual array IS an incompleteness signal.
	require.Contains(t, script, "if (priceScanned + list.length > MAX_PRICE_SCAN) {",
		"scanPrice must refuse an array it cannot afford to traverse")

	// The property cap IS an incompleteness signal, because later properties go
	// unvisited and completeness cannot be established.
	// The property cap is charged per DISTINCT property, so a second reader
	// walking the same object does not pay for it twice.
	require.Contains(t, script, "if (charged.size + 1 > MAX_PRICE_BLOCKS) {",
		"the property cap must count distinct properties, not visits")
	require.Contains(t, script, "if (charged.size + 1 > MAX_PRICE_BLOCKS) {"+"\n"+"          markTrunc('price_facts');",
		"leaving properties unvisited must mark the price field truncated")
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

// A price set that reaches exactly the cap is complete and must be preserved;
// only an actual additional price entry may mark the set as overflowing.
func TestBrowserAcquirePreservesExactlyCappedPriceSet(t *testing.T) {
	browser := fixtureBrowserPath(t)
	ranges := make([]any, 0, maxPriceFacts)
	for i := 0; i < maxPriceFacts; i++ {
		ranges = append(ranges, map[string]any{"price": "1.00", "beginAmount": 1})
	}
	data := map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Exact cap bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
		"prices": map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
			"tradeWithoutPromotion": map[string]any{"offerPriceRanges": ranges},
		}}},
		// A later property with no price range must not trigger a drop.
		"unrelated": map[string]any{"fields": map[string]any{"somethingElse": "x"}},
	}
	srv := serveFixture(t, buildContextPage(data))
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Len(t, evidence.PriceFacts, maxPriceFacts,
		"a price set that exactly reaches the cap is complete and must be kept")
	require.False(t, hasTruncation(evidence, "price_facts"),
		"reaching the cap is not truncation")
}

// A price array that exactly fills the scan budget, followed by an unrelated
// property, must keep the complete valid price set: only an actual refusal by
// scanPrice may drop it.
func TestBrowserAcquirePreservesSetThatExactlyExhaustsScanBudget(t *testing.T) {
	browser := fixtureBrowserPath(t)
	scanBudget := maxPriceFacts * 4
	ranges := make([]any, 0, scanBudget)
	for i := 0; i < scanBudget; i++ {
		// Only the first maxPriceFacts entries are valid prices.
		if i < maxPriceFacts {
			ranges = append(ranges, map[string]any{"price": "1.00", "beginAmount": 1})
			continue
		}
		ranges = append(ranges, map[string]any{"note": "not a price"})
	}
	data := map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Budget bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
		"prices": map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
			"tradeWithoutPromotion": map[string]any{"offerPriceRanges": ranges},
		}}},
		"unrelated": map[string]any{"fields": map[string]any{"somethingElse": "x"}},
	}
	srv := serveFixture(t, buildContextPage(data))
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Len(t, evidence.PriceFacts, maxPriceFacts,
		"exactly exhausting the scan budget must not drop a complete price set")
}

// Hitting the property cap means later properties were never visited, so
// completeness cannot be established and the collected prefix must be dropped.
func TestBrowserAcquireDropsPrefixWhenPropertyCapIsHit(t *testing.T) {
	browser := fixtureBrowserPath(t)
	data := map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Property cap bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
	}
	// A price array before the boundary, and another far after it.
	data["early"] = map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
		"tradeWithoutPromotion": map[string]any{"offerPriceRanges": []any{
			map[string]any{"price": "5.00", "beginAmount": 1},
		}},
	}}}
	for i := 0; i < 600; i++ {
		data["filler"+itoa(i)] = map[string]any{"fields": map[string]any{"n": i}}
	}
	data["late"] = map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
		"tradeWithoutPromotion": map[string]any{"offerPriceRanges": []any{
			map[string]any{"price": "9.00", "beginAmount": 1},
		}},
	}}}
	srv := serveFixture(t, buildContextPage(data))
	client := newTestClient(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Empty(t, evidence.PriceFacts,
		"prices collected before the property cap must not be published as the whole set")
	require.True(t, hasTruncation(evidence, "price_facts"))
}

// A model with entries but no usable property names must fall through to the
// complete model, not become authoritative and suppress it.
func TestBrowserAcquireFallsThroughModelWithoutUsableProps(t *testing.T) {
	binPath := fixtureBrowserPath(t)
	page := `<!doctype html><html><head><title>No props</title></head><body><script>
window.__INIT_DATA = {"data":{"main":{"data":{
  "offerId": 981645030344,
  "title": "No props item",
  "nySkuModel": {"skuInfoMap": {"red": {"skuId": 11, "price": 1.5}}},
  "skuModel": {"skuProps":[{"prop":"color"}], "skuInfoMap":{"blue":{"skuId":66,"price":2.5}}}
}}}};
</script></body></html>`
	srv := serveFixture(t, page)
	client := newTestClient(Options{ExecutablePath: binPath, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Len(t, evidence.Variants, 1, "a model without usable props must not suppress the complete model")
	require.NotNil(t, evidence.Variants[0].SourceID)
	require.Equal(t, "66", *evidence.Variants[0].SourceID)
}

// A whitespace-padded minimum quantity is usable and must be kept.
func TestBrowserAcquireTrimsPaddedMinimumQuantity(t *testing.T) {
	binPath := fixtureBrowserPath(t)
	page := buildContextPage(map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Padded qty bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
		"price": map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{
			"tradeWithoutPromotion": map[string]any{"offerPriceRanges": []any{
				map[string]any{"price": "12.50", "beginAmount": " 2 "},
			}},
		}}},
	})
	srv := serveFixture(t, page)
	client := newTestClient(Options{ExecutablePath: binPath, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source := mustSourceForShape(t)
	evidence, err := client.Acquire(t.Context(), source)
	require.NoError(t, err)
	require.NotEmpty(t, evidence.PriceFacts)
	require.NotNil(t, evidence.PriceFacts[0].MinQuantity, "a padded but valid minimum quantity must be kept")
	_, err = sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-pad")
	require.NoError(t, err)
}

// The currentPrices fallback walks the same page-supplied object as readPrices,
// so it must draw on the same property budget rather than starting a fresh
// unbounded scan.
func TestExtractorSharesOnePropertyBudgetAcrossPriceReaders(t *testing.T) {
	script := extractScript()
	require.Equal(t, 1, strings.Count(script, "for (const k in data) {"),
		"only the shared walker may enumerate page-supplied properties directly")
	require.Contains(t, script, "const forEachPriceBlock = (data, fn) => {",
		"every price reader must go through the shared budgeted walker")
	require.Contains(t, script, "forEachPriceBlock(data,")
	require.Contains(t, script, "forEachPriceBlock(init,")
}

// currentPrices is a single representation: only the first block carrying one is
// read, matching the established extractor, rather than concatenating every block
// and producing duplicate or conflicting tiers.
func TestBrowserAcquireStopsAfterFirstCurrentPriceBlock(t *testing.T) {

	mk := func(price string) map[string]any {
		return map[string]any{"fields": map[string]any{"priceModel": map[string]any{
			"currentPrices": []any{map[string]any{"price": price, "beginAmount": 1}},
		}}}
	}
	page := buildContextPage(map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Two current price blocks"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
		"first":  mk("5.00"),
		"second": mk("9.00"),
		"third":  mk("13.00"),
	})
	srv := serveFixture(t, page)
	client := newTestClient(Options{ExecutablePath: binPathOf(t), Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.Len(t, evidence.PriceFacts, 1, "only the first currentPrices representation may be read")
	require.Equal(t, "5.00", evidence.PriceFacts[0].Amount)
}

// Two readers legitimately walk the same page-supplied object. Charging the
// properties twice would exhaust the budget and discard an available price set,
// so the budget must be per object rather than per walk.
func TestBrowserAcquireDoesNotDoubleChargeThePropertyBudget(t *testing.T) {
	bin := fixtureBrowserPath(t)
	mkCurrent := func(price string) map[string]any {
		return map[string]any{"fields": map[string]any{"priceModel": map[string]any{
			"currentPrices": []any{map[string]any{"price": price, "beginAmount": 1}},
		}}}
	}
	data := map[string]any{
		"productTitle": map[string]any{"fields": map[string]any{"title": "Double charge bottle"}},
		"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
			"tempModel": map[string]any{"offerId": 981645030344},
		}}},
	}
	// A large number of properties with no offerPriceRanges, then the single
	// currentPrices block well past the midpoint of the budget.
	for i := 0; i < 300; i++ {
		data["filler"+itoa(i)] = map[string]any{"fields": map[string]any{"n": i}}
	}
	data["late"] = mkCurrent("7.00")
	srv := serveFixture(t, buildContextPage(data))
	client := newTestClient(Options{ExecutablePath: bin, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	evidence, err := client.Acquire(t.Context(), mustSourceForShape(t))
	require.NoError(t, err)
	require.NotEmpty(t, evidence.PriceFacts,
		"a second reader walking the same object must not exhaust the shared budget")
	require.Equal(t, "7.00", evidence.PriceFacts[0].Amount)
}
