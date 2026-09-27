package browser

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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
