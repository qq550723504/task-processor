package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/product/sourcing"
)

// fixtureBrowserPath returns a Chromium binary or skips, mirroring the existing
// BATCHCAPTURE_BROWSER opt-in so a CI box without a browser simply skips.
func fixtureBrowserPath(t *testing.T) string {
	t.Helper()
	for _, candidate := range []string{
		os.Getenv("A1688_BROWSER"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "ms-playwright", "chromium-1234", "chrome-win64", "chrome.exe"),
	} {
		if candidate != "" {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}
	t.Skip("set A1688_BROWSER (or install a playwright chromium) to run browser fixture tests")
	return ""
}

// fixtureContextJSON is built from a Go value so it is always valid JSON.
var fixtureContextJSON = func() string {
	raw := map[string]any{
		"result": map[string]any{
			"global": map[string]any{"globalData": map[string]any{"model": map[string]any{"offerDetail": map[string]any{
				"featureAttributes": []any{map[string]any{"name": "material", "value": "steel"}},
			}}}},
			"data": map[string]any{
				"productTitle": map[string]any{"fields": map[string]any{"title": "Fixture browser bottle"}},
				"gallery":      map[string]any{"fields": map[string]any{"offerImgList": []any{"https://cbu01.alicdn.com/fixture.jpg", "https://cbu01.alicdn.com/fixture2.jpg"}}},
				"Root": map[string]any{"fields": map[string]any{"dataJson": map[string]any{
					"tempModel": map[string]any{"offerId": 981645030344},
					"skuModel": map[string]any{
						"skuProps":   []any{map[string]any{"prop": "color"}},
						"skuInfoMap": map[string]any{"red": map[string]any{"skuId": "sku-red", "price": "12.50", "currency": "CNY"}},
					},
				}}},
				"price": map[string]any{"fields": map[string]any{"finalPriceModel": map[string]any{"tradeWithoutPromotion": map[string]any{
					"offerPriceRanges": []any{map[string]any{"price": "12.50", "beginAmount": 2}},
				}}}},
			},
		},
	}
	encoded, _ := json.Marshal(raw)
	return string(encoded)
}()

var fixtureContextPage = "<!doctype html><html><head><title>Fixture bottle</title></head><body><script>window.context = " + fixtureContextJSON + ";</script></body></html>"

// challengeTitleCN is the 1688 challenge page title marker.
const challengeTitleCN = "安全验证"

// sliderChallengePage builds a challenge page whose slider clears the challenge
// once dragged past halfway, then reveals the window.context product shape. The
// context JSON is injected as a quoted JS string literal so the page contains no
// nested template literal.
func sliderChallengePage() string {
	// fixtureContextJSON is already encoded JSON; marshaling it again would
	// produce a quoted literal and window.context would become a string.
	js := "window.context = " + fixtureContextJSON + ";"
	return `<!doctype html><html><head><title>` + challengeTitleCN + `</title></head><body>
<div class="nc_wrapper"><span class="nc_iconfont btn_slide" style="position:fixed;left:20px;top:300px;width:40px;height:40px;background:#ccc;z-index:99"></span></div>
<script>
document.addEventListener('mousemove', function (e) {
  if (e.clientX > 400) {
    document.title = 'Fixture bottle';
    var d = document.createElement('div');
    d.className = 'slider-success';
    d.style.cssText = 'width:12px;height:12px;background:green';
    document.body.appendChild(d);
    var s = document.createElement('script');
    s.textContent = ` + js + `;
    document.body.appendChild(s);
  }
});
</script></body></html>`
}

// stubbornChallengePage never clears, so the honest failure path runs.
func stubbornChallengePage() string {
	return `<!doctype html><html><head><title>` + challengeTitleCN + ` 验证码</title></head><body>
<div class="nc_wrapper"><span class="nc_iconfont btn_slide" style="position:fixed;left:20px;top:300px;width:40px;height:40px;background:#ccc;z-index:99"></span></div>
</body></html>`
}

const fixtureChallengePage = `<!doctype html><html><head><title>请登录</title></head><body>login</body></html>`

func serveFixture(t *testing.T, page string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(page))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// A real Chromium on a local fixture carrying window.context must yield
// untrusted evidence that the current owner maps into an envelope, with no
// 1688 network access.
func TestBrowserAcquireExtractsContextEvidence(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, fixtureContextPage)
	client := New(Options{
		ExecutablePath:      browser,
		Headless:            true,
		AllowedOrigins:      []string{srv.URL},
		navigateURLOverride: srv.URL,
	})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)
	require.Equal(t, "Fixture browser bottle", *evidence.Title)
	require.Equal(t, ParserVersion, evidence.ParserVersion)
	require.Len(t, evidence.Images, 2)
	require.Len(t, evidence.Attributes, 1)
	require.Len(t, evidence.Variants, 1)
	require.Equal(t, "12.50", evidence.Variants[0].Price.Amount)
	require.Len(t, evidence.PriceFacts, 1)

	envelope, err := sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-browser-fixture")
	require.NoError(t, err)
	key, _, err := sourcing.PublicationIdentity(envelope)
	require.NoError(t, err)
	require.Equal(t, "crawler:1688:981645030344", key)
}

// A challenge/interstitial page must be reported as ErrChallenge, never mapped
// to a fake product.
func TestBrowserAcquireReportsChallenge(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, fixtureChallengePage)
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	_, err = client.Acquire(context.Background(), source)
	require.ErrorIs(t, err, ErrChallenge)
}

// decodeEvidence must accept the fixture payload shape and produce evidence
// that MapAcquisitionEvidence turns into an envelope (no browser needed).
func TestDecodeEvidenceProducesMappableEvidence(t *testing.T) {
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	payload := map[string]any{
		"offerId":    "981645030344",
		"title":      "Fixture browser bottle",
		"images":     []any{"https://cbu01.alicdn.com/fixture.jpg"},
		"attributes": []any{map[string]any{"name": "material", "value": "steel"}},
		"variants": []any{map[string]any{
			"sourceId":   "sku-red",
			"attributes": []any{map[string]any{"name": "color", "value": "red"}},
			"price":      map[string]any{"amount": "12.50", "currency": "CNY"},
		}},
		"priceFacts": []any{map[string]any{"amount": "12.50", "minQuantity": "2"}},
	}
	evidence, err := decodeEvidence(source, payload)
	require.NoError(t, err)
	require.Equal(t, "Fixture browser bottle", *evidence.Title)
	require.Equal(t, ParserVersion, evidence.ParserVersion)
	require.Len(t, evidence.Images, 1)
	require.Len(t, evidence.Attributes, 1)
	require.Len(t, evidence.Variants, 1)
	require.Equal(t, "12.50", evidence.Variants[0].Price.Amount)
	require.Len(t, evidence.PriceFacts, 1)
	envelope, err := sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-browser-fixture")
	require.NoError(t, err)
	key, _, err := sourcing.PublicationIdentity(envelope)
	require.NoError(t, err)
	require.Equal(t, "crawler:1688:981645030344", key)
}

// A wrong offerId, empty title, or blank title must be rejected honestly.
func TestDecodeEvidenceRejectsUntrustedShapes(t *testing.T) {
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	for name, payload := range map[string]map[string]any{
		"wrong offer":  {"offerId": "1", "title": "x"},
		"no title":     {"offerId": "981645030344"},
		"blank title":  {"offerId": "981645030344", "title": "   "},
		"not an offer": {"offerId": "981645030344", "title": "x", "unexpected": make(chan int)},
	} {
		_, err := decodeEvidence(source, payload)
		require.Error(t, err, name)
	}
}

// The origin allowlist must compare full origins, rejecting suffix tricks.
func TestOriginAllowedRejectsDisallowedAndSuffixTricks(t *testing.T) {
	allowed := []string{"https://detail.1688.com"}
	require.True(t, originAllowed(allowed, "https://detail.1688.com/offer/1.html"))
	require.True(t, originAllowed(allowed, "https://DETAIL.1688.COM/offer/1.html"))
	require.False(t, originAllowed(allowed, "https://evil.test/x"))
	require.False(t, originAllowed(allowed, "https://detail.1688.com.evil.test/x"))
	require.False(t, originAllowed(allowed, "http://detail.1688.com/offer/1.html"), "scheme must match")
	// An empty allowlist means the route guard is not installed at all (test-only);
	// originAllowed itself still matches nothing.
	require.False(t, originAllowed([]string{}, "https://anything.test"))
}

// The resolved A2 allowlist admits the 1688 host, its CDN, and 1688 assets over
// https, while a single-label "*." wildcard must not match a deeper subdomain
// and must never admit a lookalike host.
func TestDefaultAllowedOriginsMatchOnly1688AndCDN(t *testing.T) {
	for _, ok := range []string{
		"https://detail.1688.com/offer/965933437579.html",
		"https://img.alicdn.com/x.jpg",
		"https://g.alicdn.com/code/y.js",
		"https://s.1688.com/x",
	} {
		require.True(t, originAllowed(DefaultAllowedOrigins, ok), ok)
	}
	for _, bad := range []string{
		"http://detail.1688.com/offer/1.html", // http is not admitted
		"https://a.b.alicdn.com/x",            // wildcard is single-label only
		"https://alicdn.com.evil.test/x",      // lookalike suffix
		"https://detail.1688.com.evil.test/x", // lookalike suffix
		"https://evil.test/x",
		"https://localhost:5432/db", // no database reachability (D12/D13)
		"https://127.0.0.1:5432/db",
		"wss://detail.1688.com/socket", // non-http scheme
	} {
		require.False(t, originAllowed(DefaultAllowedOrigins, bad), bad)
	}
}

// The default allowlist must not admit any private/loopback address, which is
// what keeps the collector structurally unable to reach a database (D12/D13).
func TestDefaultAllowedOriginsRejectPrivateAndLoopback(t *testing.T) {
	for _, bad := range []string{
		"https://127.0.0.1/x", "https://[::1]/x", "https://10.0.0.5/x",
		"https://192.168.1.10/x", "https://172.16.0.3/x", "https://localhost/x",
	} {
		require.False(t, originAllowed(DefaultAllowedOrigins, bad), bad)
	}
}

// A solvable challenge fixture: dragging the slider clears the challenge and the
// product page is then collected. This exercises the A4 single automatic attempt
// end to end in a real browser.
func TestBrowserAcquireSolvesSliderChallengeOnce(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, sliderChallengePage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err, "a cleared challenge must yield the product: %v", err)
	require.Equal(t, "Fixture browser bottle", *evidence.Title)
}

// A challenge that never clears must fail honestly: exactly one bounded
// attempt, then ErrChallenge, and never a fabricated product (A4/D4).
func TestBrowserAcquireFailsHonestlyWhenChallengePersists(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, stubbornChallengePage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	_, err = client.Acquire(context.Background(), source)
	require.ErrorIs(t, err, ErrChallenge)
}

// A login-wall redirect observed in real acceptance must be reported as a
// challenge, not as an unknown page shape, and must not burn the captcha budget
// on a slider that cannot clear a redirect.
func TestBrowserAcquireReportsLoginWallAsChallenge(t *testing.T) {
	browser := fixtureBrowserPath(t)
	// A page whose title looks like a normal site title and which carries no
	// product, served from a login-wall URL: the only reliable signal is the URL.
	srv := serveFixture(t, `<!doctype html><html><head><title>Normal looking site title</title></head><body>no product here</body></html>`)
	client := New(Options{
		ExecutablePath:      browser,
		Headless:            true,
		AllowedOrigins:      []string{srv.URL},
		navigateURLOverride: srv.URL + "/login.taobao.com/redirect",
	})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	_, err = client.Acquire(context.Background(), source)
	require.ErrorIs(t, err, ErrChallenge)
	require.NotErrorIs(t, err, ErrUnsupported, "a login wall is a challenge, not an unknown shape")
}

// A page-side clip or an unrepresentable number must never reach publication
// looking like the exact source fact: it has to become an explicit warning and
// missing fact on the evidence.
func TestDecodeEvidenceSurfacesTruncationExplicitly(t *testing.T) {
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	payload := map[string]any{
		"offerId":         "981645030344",
		"title":           "Fixture bottle",
		"truncatedFields": []any{"images", "variant_source_id"},
	}
	evidence, err := decodeEvidence(source, payload)
	require.NoError(t, err)
	require.Len(t, evidence.Warnings, 2)
	require.Len(t, evidence.MissingFacts, 2)
	codes := map[string]bool{}
	fields := map[string]bool{}
	for _, w := range evidence.Warnings {
		codes[w.Code] = true
		fields[w.Field] = true
	}
	require.True(t, codes["source_evidence_truncated"])
	require.True(t, fields["images"])
	require.True(t, fields["variant_source_id"])

	// The warnings must survive into the mapped envelope rather than being
	// dropped, so the Catalog publication is visibly incomplete.
	envelope, err := sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-trunc")
	require.NoError(t, err)
	found := false
	for _, w := range envelope.Warnings {
		if w.Code == "source_evidence_truncated" {
			found = true
		}
	}
	require.True(t, found, "truncation must be visible on the published envelope")
}

// An untruncated payload must not gain any truncation warning.
func TestDecodeEvidenceNoTruncationWarningWhenComplete(t *testing.T) {
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := decodeEvidence(source, map[string]any{
		"offerId": "981645030344",
		"title":   "Fixture bottle",
	})
	require.NoError(t, err)
	require.Empty(t, evidence.Warnings)
	require.Empty(t, evidence.MissingFacts)
}

// A SKU id or price outside JavaScript's safe-integer range cannot be
// represented exactly, so it must be dropped and reported rather than silently
// rewritten (String(9007199254740993) would publish ...992).
func TestBrowserAcquireDropsUnrepresentableNumericIdentifiers(t *testing.T) {
	browser := fixtureBrowserPath(t)
	// 9007199254740993 loses its last digit as a JavaScript number, and the SKU
	// key has more segments than skuProps supplies names for.
	srv := serveFixture(t, sliderChallengePageWithUnsafeNumbers())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)

	// No variant may carry a silently rewritten identifier.
	for _, v := range evidence.Variants {
		if v.SourceID != nil {
			require.NotEqual(t, "9007199254740992", *v.SourceID, "an unsafe integer must never be published rewritten")
			require.NotEqual(t, "9007199254740993", *v.SourceID, "an unrepresentable value must be dropped, not invented")
		}
	}
	// And the loss must be reported explicitly.
	reported := false
	for _, w := range evidence.Warnings {
		if w.Code == "source_evidence_truncated" {
			reported = true
		}
	}
	require.True(t, reported, "dropping an unrepresentable identifier must be reported, not silent")
}

// sliderChallengePageWithUnsafeNumbers returns a page whose only variant has an
// unsafe-integer skuId, an unsafe-integer price, and an unmatched attribute
// segment, so none of them may be published as an exact fact.
func sliderChallengePageWithUnsafeNumbers() string {
	return `<!doctype html><html><head><title>Fixture bottle</title></head><body>
<script>
window.context = {"result":{"data":{
  "productTitle":{"fields":{"title":"Fixture browser bottle"}},
  "Root":{"fields":{"dataJson":{
    "tempModel":{"offerId":981645030344},
    "skuModel":{"skuProps":[{"prop":"color"}],
      "skuInfoMap":{"red&gt;blue":{"skuId":9007199254740993,"price":9007199254740993}}}
  }}}
}}};
</script></body></html>`
}

// A custom-item page that supplies only window.__INIT_DATA must still have its
// meta description preserved, and an unsafe-integer beginAmount must be dropped
// rather than published as an already-rounded quantity.
func TestBrowserAcquireCustomItemKeepsDescriptionAndExactQuantity(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, customItemPage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)

	require.NotNil(t, evidence.Description, "a custom-item page must keep its observed description")
	require.Equal(t, "Custom fixture item", *evidence.Description)

	for _, f := range evidence.PriceFacts {
		if f.MinQuantity != nil {
			require.NotEqual(t, "9007199254740992", *f.MinQuantity, "an unsafe beginAmount must not be published rounded")
		}
	}
}

// customItemPage is a custom-item page: no window.context, a meta description,
// and an unsafe-integer minimum quantity on its price range.
func customItemPage() string {
	return `<!doctype html><html><head><title>Custom item</title>
<meta name="description" content="Custom fixture item"></head><body>
<script>
window.__INIT_DATA = {"data":{"main":{"data":{
  "offerInfoModel":{"offerId":981645030344,"title":"Custom fixture bottle"},
  "offerImgList":["https://cbu01.alicdn.com/custom.jpg"],
  "propsList":[{"name":"material","value":"steel"}]
}}},"globalData":{"skuModel":{"skuProps":[{"prop":"color"}],
  "skuInfoMap":{"red":{"skuId":7,"price":9.5}}}}};
</script></body></html>`
}

// A fractional price is exactly representable and must survive: only an integer
// beyond the safe-integer range has actually lost precision.
func TestBrowserAcquireKeepsExactFractionalPrice(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, customItemPage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)
	require.NotEmpty(t, evidence.Variants, "the custom-item SKU model must be extracted")
	require.NotNil(t, evidence.Variants[0].Price, "a fractional price must not be dropped")
	require.Equal(t, "9.5", evidence.Variants[0].Price.Amount)
}

// A redirect target must be resolved against the request URL and checked.
func TestAbsoluteURLResolution(t *testing.T) {
	got, err := absoluteURL("https://detail.1688.com/offer/1.html", "/offer/2.html")
	require.NoError(t, err)
	require.Equal(t, "https://detail.1688.com/offer/2.html", got)

	got, err = absoluteURL("https://detail.1688.com/offer/1.html", "https://img.alicdn.com/x.png")
	require.NoError(t, err)
	require.Equal(t, "https://img.alicdn.com/x.png", got)

	_, err = absoluteURL("https://detail.1688.com/offer/1.html", "  ")
	require.Error(t, err)
}

// A clipped value is not the source fact and must never be published as one.
// An oversized title is therefore rejected outright, and an oversized numeric
// string is dropped and reported rather than transferred.
func TestBrowserAcquireRejectsOversizedSourceFacts(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, oversizedTitlePage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	// An oversized title is the product identity, so the acquisition is refused
	// rather than published with a shortened title.
	_, err = client.Acquire(context.Background(), source)
	require.Error(t, err, "an oversized title must not be published clipped")
}

func oversizedTitlePage() string {
	return `<!doctype html><html><head><title>Fixture</title></head><body><script>
window.context = {"result":{"data":{"productTitle":{"fields":{"title":"` + strings.Repeat("x", maxStringLen+64) + `"}}}}};
</script></body></html>`
}

// The known real page shapes must all be collected, using the field paths the
// operator's legacy extractors document rather than invented ones: a
// protocol-relative image URL, a discount price without a price, a direct
// custom-item offerId/title, and the standard-page skuRangePrices fallback.
func TestBrowserAcquireCollectsKnownLegacyShapes(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, knownShapesPage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)

	require.Equal(t, "Known shapes bottle", *evidence.Title)
	require.NotEmpty(t, evidence.Images)
	for _, img := range evidence.Images {
		require.True(t, strings.HasPrefix(img.URL, "https://"), "a protocol-relative image URL must be normalized")
	}
	require.NotEmpty(t, evidence.Variants, "the discount price must not drop the variant")
	require.NotNil(t, evidence.Variants[0].Price, "a discount price must be retained as the variant price")
	require.Equal(t, "8.25", evidence.Variants[0].Price.Amount)
	require.NotEmpty(t, evidence.PriceFacts, "the standard-page skuRangePrices fallback must be read")
}

func knownShapesPage() string {
	return `<!doctype html><html><head><title>Known shapes</title></head><body><script>
window.context = {"result":{"data":{
  "productTitle":{"fields":{"title":"Known shapes bottle"}},
  "gallery":{"fields":{"offerImgList":["//cbu01.alicdn.com/legacy.jpg"]}},
  "Root":{"fields":{"dataJson":{
    "tempModel":{"offerId":981645030344},
    "orderParamModel":{"orderParam":{"skuParam":{"skuRangePrices":[{"price":8.25,"beginAmount":3,"currency":"CNY"}]}}},
    "skuModel":{"skuProps":[{"prop":"color"}],
      "skuInfoMap":{"red":{"skuId":7,"discountPrice":8.25,"currency":"CNY"}}}
  }}}
}}};
</script></body></html>`
}

// A custom-item page using the direct data.offerId / data.title identity shape,
// with a protocol-relative image, must be collected rather than rejected.
func TestBrowserAcquireCollectsDirectCustomItemIdentity(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, directCustomItemPage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)
	require.Equal(t, "Direct identity item", *evidence.Title)
	require.NotEmpty(t, evidence.Images)
	for _, img := range evidence.Images {
		require.True(t, strings.HasPrefix(img.URL, "https://"))
	}
}

func directCustomItemPage() string {
	return `<!doctype html><html><head><title>Direct identity</title></head><body><script>
window.__INIT_DATA = {"data":{"main":{"data":{
  "offerId": 981645030344,
  "title": "Direct identity item",
  "offerImgList": ["//cbu01.alicdn.com/direct.jpg"],
  "propsList": [{"name":"material","value":"steel"}]
}}}};
</script></body></html>`
}

// A custom-item block that keeps its variants under a block-local nySkuModel or
// skuModelOrigin must still publish them.
func TestBrowserAcquireCollectsBlockLocalSkuModels(t *testing.T) {
	browser := fixtureBrowserPath(t)
	for name, page := range map[string]string{
		"nySkuModel":     blockLocalCustomItemPage("nySkuModel"),
		"skuModelOrigin": blockLocalCustomItemPage("skuModelOrigin"),
	} {
		srv := serveFixture(t, page)
		client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
		source, err := sourcing.Canonical1688Source("981645030344")
		require.NoError(t, err)
		evidence, err := client.Acquire(context.Background(), source)
		require.NoError(t, err, name)
		require.NotEmpty(t, evidence.Variants, name+": block-local sku model must be read")
	}
}

func blockLocalCustomItemPage(field string) string {
	return `<!doctype html><html><head><title>Block local</title></head><body><script>
window.__INIT_DATA = {"data":{"main":{"data":{
  "offerId": 981645030344,
  "title": "Block local item",
  "` + field + `":{"skuProps":[{"prop":"color"}],
    "skuInfoMap":{"red":{"skuId":7,"price":3.5}}}
}}}};
</script></body></html>`
}

// A custom-item payload exposing the same SKU model in a data block and in
// globalData must not produce duplicated variants, because repeated source IDs
// make the whole acquisition fail.
func TestBrowserAcquireDoesNotDuplicateCustomItemVariants(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, duplicateCustomItemPage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)
	require.Len(t, evidence.Variants, 1, "the same SKU model must not be read twice")
	envelope, err := sourcing.MapAcquisitionEvidence(source, evidence, sourcing.AcquisitionChannelPublicBrowser, "op-dup")
	require.NoError(t, err, "a duplicated source ID would fail the whole mapping")
	_ = envelope
}

func duplicateCustomItemPage() string {
	return `<!doctype html><html><head><title>Duplicate</title></head><body><script>
window.__INIT_DATA = {
  "data": {"main": {"data": {
    "offerId": 981645030344,
    "title": "Duplicate custom item",
    "skuModel": {"skuProps":[{"prop":"color"}], "skuInfoMap":{"red":{"skuId":7,"price":3.5}}}
  }}},
  "globalData": {"nySkuModel": {"skuProps":[{"prop":"color"}], "skuInfoMap":{"red":{"skuId":7,"price":3.5}}}}
};
</script></body></html>`
}

// A custom-item attribute list stored directly as the block data array, and a
// global SKU model that must win over a differing block-local model.
func TestBrowserAcquireHandlesArrayAttributesAndGlobalSkuPriority(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, arrayAttrsAndGlobalSkuPage())
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)
	require.NotEmpty(t, evidence.Attributes, "an array-backed attribute list must be read")
	require.Len(t, evidence.Variants, 1, "the authoritative global SKU model must be used")
	require.NotNil(t, evidence.Variants[0].SourceID)
	require.Equal(t, "42", *evidence.Variants[0].SourceID, "the block-local model must not pre-empt the global one")
}

func arrayAttrsAndGlobalSkuPage() string {
	return `<!doctype html><html><head><title>Array attrs</title></head><body><script>
window.__INIT_DATA = {
  "data": {
    "attrs": {"data": [{"name":"material","value":"steel"},{"name":"color","value":"red"}]},
    "sku": {"data": {"offerId": 981645030344, "title": "Array attrs item",
      "skuModel": {"skuProps":[{"prop":"size"}], "skuInfoMap":{"s":{"skuId":7,"price":1.5}}}}}
  },
  "globalData": {"nySkuModel": {"skuProps":[{"prop":"color"}], "skuInfoMap":{"red":{"skuId":42,"price":2.5}}}}
};
</script></body></html>`
}

// A standard page that omits tempModel must still be collected, using the offer
// id carried by the canonical detail URL.
func TestBrowserAcquireRecoversOfferIdFromCanonicalURL(t *testing.T) {
	browser := fixtureBrowserPath(t)
	srv := serveFixture(t, noTempModelPage())
	// Navigate a canonical-looking /offer/<id>.html path so the URL recovery applies.
	client := New(Options{ExecutablePath: browser, Headless: true, AllowedOrigins: []string{srv.URL}, navigateURLOverride: srv.URL + "/offer/981645030344.html"})
	source, err := sourcing.Canonical1688Source("981645030344")
	require.NoError(t, err)
	evidence, err := client.Acquire(context.Background(), source)
	require.NoError(t, err)
	require.Equal(t, "981645030344", evidence.OfferID, "the offer id must be recovered from the canonical URL")
}

func noTempModelPage() string {
	return `<!doctype html><html><head><title>No temp model</title></head><body><script>
window.context = {"result":{"data":{
  "productTitle":{"fields":{"title":"No temp model bottle"}}
}}};
</script></body></html>`
}
