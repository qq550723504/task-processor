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
