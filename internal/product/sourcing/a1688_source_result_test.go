package sourcing

import (
	"encoding/json"
	"errors"
	"testing"
)

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
