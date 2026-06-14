package sourcing

import (
	"testing"
)

type stubAmazonDomainResolver struct {
	domain string
}

func (r stubAmazonDomainResolver) GetAmazonDomainByRegion(string) string {
	return r.domain
}

func (r stubAmazonDomainResolver) BuildAmazonProductURL(region, asin string) string {
	return "https://example." + region + "/dp/" + asin
}

type stubAmazonZipcodePolicy struct {
	useDefault bool
	defaultZip string
}

func (p stubAmazonZipcodePolicy) ShouldUseDefaultZipcode(string) bool {
	return p.useDefault
}

func (p stubAmazonZipcodePolicy) DefaultZipcode(string) string {
	return p.defaultZip
}

func TestAmazonCrawlRequestPlannerBuildRequestUsesExplicitZipcode(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{
		DomainResolver: stubAmazonDomainResolver{domain: "amazon.co.uk"},
		ZipcodePolicy:  stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
	}

	got, err := planner.BuildRequest(AmazonCrawlRequestInput{
		Region:    " UK ",
		ProductID: " B001 ",
		Zipcode:   " EC1A 1BB ",
	})
	if err != nil {
		t.Fatalf("BuildRequest() error = %v", err)
	}
	if got.URL != "https://example.uk/dp/B001" {
		t.Fatalf("URL = %q, want generated URL", got.URL)
	}
	if got.Zipcode != "EC1A 1BB" {
		t.Fatalf("Zipcode = %q, want explicit zipcode", got.Zipcode)
	}
}

func TestAmazonCrawlRequestPlannerBuildBatchRequestsUsesConfiguredDefaultZipcode(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{
		DomainResolver: stubAmazonDomainResolver{domain: "amazon.co.uk"},
		ZipcodePolicy:  stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
		Zipcodes:       map[string]string{"uk": "W1A 1AA"},
	}

	got, err := planner.BuildBatchRequests(AmazonCrawlRequestInput{Region: " UK "}, []string{" B001 ", "B002"})
	if err != nil {
		t.Fatalf("BuildBatchRequests() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	if got[0].Zipcode != "W1A 1AA" || got[1].Zipcode != "W1A 1AA" {
		t.Fatalf("zipcodes = %q/%q, want configured default", got[0].Zipcode, got[1].Zipcode)
	}
	if got[0].URL != "https://example.uk/dp/B001" {
		t.Fatalf("got[0].URL = %q, want trimmed first product URL", got[0].URL)
	}
	if got[1].URL != "https://example.uk/dp/B002" {
		t.Fatalf("got[1].URL = %q, want URL for second product", got[1].URL)
	}
}

func TestAmazonCrawlRequestPlannerBuildRequestRejectsUnsupportedRegion(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{DomainResolver: stubAmazonDomainResolver{}}

	_, err := planner.BuildRequest(AmazonCrawlRequestInput{Region: "unknown", ProductID: "B001"})
	if err == nil {
		t.Fatal("BuildRequest() error = nil, want unsupported region error")
	}
}

func TestAmazonCrawlRequestPlannerResolveZipcodeUsesConfiguredDefault(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{
		ZipcodePolicy: stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
		Zipcodes:      map[string]string{"uk": "W1A 1AA"},
	}

	got := planner.ResolveZipcode("UK", "")
	if got != "W1A 1AA" {
		t.Fatalf("ResolveZipcode() = %q, want configured default", got)
	}
}

func TestAmazonCrawlRequestPlannerResolveZipcodePreservesExplicit(t *testing.T) {
	planner := AmazonCrawlRequestPlanner{
		ZipcodePolicy: stubAmazonZipcodePolicy{useDefault: true, defaultZip: "SW1A 1AA"},
		Zipcodes:      map[string]string{"uk": "W1A 1AA"},
	}

	got := planner.ResolveZipcode("UK", " EC1A 1BB ")
	if got != "EC1A 1BB" {
		t.Fatalf("ResolveZipcode() = %q, want explicit zipcode", got)
	}
}
