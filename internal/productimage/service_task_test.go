package productimage

import "testing"

func TestValidateRequestAcceptsNonAmazonMarketplace(t *testing.T) {
	t.Parallel()

	svc := &service{}
	req := &ImageProcessRequest{
		ProductURL:  "https://detail.1688.com/offer/123.html",
		Marketplace: "shein",
	}

	if err := svc.validateRequest(req); err != nil {
		t.Fatalf("validateRequest returned error for shein marketplace: %v", err)
	}
}

func TestValidateRequestRequiresMarketplace(t *testing.T) {
	t.Parallel()

	svc := &service{}
	req := &ImageProcessRequest{
		ProductURL: "https://detail.1688.com/offer/123.html",
	}

	if err := svc.validateRequest(req); err == nil {
		t.Fatal("expected missing marketplace to be rejected")
	}
}
