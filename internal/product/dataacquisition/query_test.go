package dataacquisition

import "testing"

func TestQueryRejectsUnknownSiteAndCrossSiteURL(t *testing.T) {
	for _, q := range []Query{
		{Site: "zz", Mode: "asin", ASINs: []string{"B000123456"}, Limit: 1},
		{Site: "us", Mode: "asin", ASINs: []string{"https://www.amazon.co.jp/dp/B000123456"}, Limit: 1},
		{Site: "us", Mode: "asin", ASINs: []string{"https://www.amazon.com.evil.test/dp/B000123456"}, Limit: 1},
		{Site: "us", Mode: "keyword", Keyword: "chair", Limit: 201},
		{Site: "us", Mode: "category", CategoryNode: "12&url=http://127.0.0.1", Limit: 1},
		{Site: "us", Mode: "asin", ASINs: []string{"B000123456"}, Limit: 1, Fields: []string{"cookie"}},
	} {
		if _, err := NormalizeQuery(q); err == nil {
			t.Fatalf("accepted unsafe query: %#v", q)
		}
	}
}

func TestQuerySupportsAllInputsAndDeduplicates(t *testing.T) {
	for _, site := range Sites() {
		q, err := NormalizeQuery(Query{Site: site.Code, Mode: "asin", ASINs: []string{"b000123456", "B000123456"}, Limit: 2})
		if err != nil || len(q.ASINs) != 1 {
			t.Fatalf("site %s: %#v %v", site.Code, q, err)
		}
		if _, err := NormalizeQuery(Query{Site: site.Code, Mode: "keyword", Keyword: "desk chair", CategoryNode: "123456", Limit: 10}); err != nil {
			t.Fatal(err)
		}
		if _, err := NormalizeQuery(Query{Site: site.Code, Mode: "category", CategoryNode: "123456", Limit: 10}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestEvidenceDoesNotTreatUnknownAvailabilityAsUnavailable(t *testing.T) {
	e := Evidence{Site: "us", ASIN: "B000123456", Title: "Chair", MainImage: "https://m.media-amazon.com/images/a.jpg", CapturedAt: "2026-10-09T12:00:00Z", ParserVersion: "amazon-v1"}
	if e.Validate() == nil {
		t.Fatal("unknown availability was accepted")
	}
	e.Availability = "available"
	if e.Validate() == nil {
		t.Fatal("available without price accepted")
	}
	e.Availability = "unavailable"
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	e.Price = -1
	if e.Validate() == nil {
		t.Fatal("negative source price accepted")
	}
	e.Price, e.Currency = 1, "123"
	if e.Validate() == nil {
		t.Fatal("numeric currency accepted")
	}
}

func TestEnvelopePreservesMissingFields(t *testing.T) {
	e := Evidence{Site: "us", ASIN: "B000123456", Title: "Chair", MainImage: "https://m.media-amazon.com/images/a.jpg", CapturedAt: "2026-10-09T12:00:00Z", ParserVersion: "amazon-v1", Availability: "unavailable", Missing: []string{"price", "currency"}}
	envelope, err := e.Envelope("0511e1d2-b555-4970-b323-3b629a901901")
	if err != nil || len(envelope.Warnings) != 2 {
		t.Fatalf("missing field reasons lost: %#v %v", envelope.Warnings, err)
	}
}
