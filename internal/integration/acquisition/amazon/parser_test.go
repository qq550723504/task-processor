package amazon

import (
	"errors"
	"task-processor/internal/product/dataacquisition"
	"testing"
	"time"
)

func TestDiscoveryDoesNotCollectAdvertisementsOrForeignLinks(t *testing.T) {
	input := `<div data-component-type="s-search-result" data-asin="B000123456"><a href="/dp/B000123456">one</a></div><div data-component-type="s-search-result" data-asin="B000123456"></div><div data-asin="B000999999">recommended</div><div data-component-type="s-search-result" data-asin="B000222222"><span>Sponsored</span></div><a href="http://127.0.0.1/dp/B000333333">foreign</a>`
	ids, err := ParseDiscovery(input, 200)
	if err != nil || len(ids) != 1 || ids[0] != "B000123456" {
		t.Fatalf("%#v %v", ids, err)
	}
	if _, err := ParseDiscovery(`<form action="/errors/validateCaptcha"><input name="field-keywords"></form>`, 1); !errors.Is(err, ErrChallenge) {
		t.Fatalf("challenge was not rejected: %v", err)
	}
}
func TestProductParserUsesActualPriceAndExplicitUnavailable(t *testing.T) {
	input := `<input id="ASIN" value="B000123456"><span id="productTitle"> Actual chair </span><img id="landingImage" src="https://m.media-amazon.com/images/a.jpg"><div id="availability">In Stock</div><span class="a-price"><span class="a-offscreen">$19.95</span></span>`
	e, err := ParseProduct(input, "us", "B000123456", time.Now())
	if err != nil || e.Price != 19.95 || e.Currency != "USD" || e.Availability != "available" {
		t.Fatalf("%#v %v", e, err)
	}
	missing := `<input id="ASIN" value="B000123456"><span id="productTitle">Chair</span><img id="landingImage" src="https://m.media-amazon.com/images/a.jpg">`
	if _, err := ParseProduct(missing, "us", "B000123456", time.Now()); !errors.Is(err, dataacquisition.ErrInvalid) {
		t.Fatalf("unknown availability fabricated: %v", err)
	}
}

func TestSearchNeedsSupportedShapeAndPriceHonorsExplicitCurrency(t *testing.T) {
	if _, err := ParseDiscovery("<html><p>maintenance</p></html>", 1); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unsupported page became zero successful results: %v", err)
	}
	ids, err := ParseDiscovery(`<div class="s-main-slot"></div>`, 1)
	if err != nil || len(ids) != 0 {
		t.Fatalf("empty supported search: %#v %v", ids, err)
	}
	for _, c := range []struct {
		text, site, currency string
		price                float64
	}{
		{"USD $19.95", "ca", "USD", 19.95}, {"1.234,56 €", "de", "EUR", 1234.56}, {"￥1,234", "jp", "JPY", 1234},
	} {
		price, currency := parsePrice(c.text, c.site)
		if price != c.price || currency != c.currency {
			t.Fatalf("%s: %v %s", c.text, price, currency)
		}
	}
}
