package sourcing

import (
	"errors"
	"testing"

	"task-processor/internal/model"
)

func TestNormalizeAmazonBatchResultsAlignsResultsWithSourceIdentities(t *testing.T) {
	sourceErr := errors.New("source failed")
	got := NormalizeAmazonBatchResults(
		AmazonCrawlRequestInput{Region: " UK ", Zipcode: " W1A 1AA "},
		[]string{" B001 ", "B002", " B003 "},
		[]model.ProductResult{
			{Product: &model.Product{Asin: "B001"}},
			{Error: sourceErr},
		},
	)

	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[0].Identity.Key() != "amazon:uk:B001" || got[0].Product == nil || got[0].Product.Asin != "B001" {
		t.Fatalf("got[0] = %+v, want first product with normalized identity", got[0])
	}
	if got[1].Identity.Key() != "amazon:uk:B002" || !errors.Is(got[1].Error, sourceErr) {
		t.Fatalf("got[1] = %+v, want second error with normalized identity", got[1])
	}
	if got[2].Identity.Key() != "amazon:uk:B003" || got[2].Product != nil || got[2].Error != nil {
		t.Fatalf("got[2] = %+v, want missing source result placeholder", got[2])
	}
}

func TestNormalizeAmazonBatchResultsReturnsEmptyForNoProductIDs(t *testing.T) {
	got := NormalizeAmazonBatchResults(AmazonCrawlRequestInput{Region: "us"}, nil, []model.ProductResult{{Product: &model.Product{Asin: "unused"}}})
	if len(got) != 0 {
		t.Fatalf("len(got) = %d, want 0", len(got))
	}
}
