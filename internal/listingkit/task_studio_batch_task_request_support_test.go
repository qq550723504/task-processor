package listingkit

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/listingsubscription"
)

func TestStudioBatchTaskProductReferenceImageURLsForVariantExcludeOtherColors(t *testing.T) {
	selection := SheinStudioSelection{
		SizeReferenceImageURLs: []string{"shared-size.jpg"},
		Variants: []SheinStudioSelectionVariant{
			{Color: "Red", SizeReferenceImageURLs: []string{"red.jpg"}},
			{Color: "Blue", SizeReferenceImageURLs: []string{"blue.jpg"}},
		},
	}
	got := studioBatchTaskProductReferenceImageURLsForVariant(selection, selection.Variants[1])
	if containsStudioBatchTaskImageURL(got, "red.jpg") {
		t.Fatalf("blue variant references include red image: %#v", got)
	}
	for _, want := range []string{"blue.jpg", "shared-size.jpg"} {
		if !containsStudioBatchTaskImageURL(got, want) {
			t.Fatalf("blue variant references = %#v, want %q", got, want)
		}
	}
}

func containsStudioBatchTaskImageURL(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestAuthorizeStudioBatchProductImageUsageFailsClosedWithoutUsageAdapter(t *testing.T) {
	s := &taskStudioBatchService{}
	err := s.authorizeStudioBatchProductImageUsage(context.Background(), &StudioBatchRecord{}, studioBatchTaskCandidate{}, 1)
	if !errors.Is(err, listingsubscription.ErrSubscriptionRequired) {
		t.Fatalf("authorize error = %v, want ErrSubscriptionRequired", err)
	}
}
