package workflow

import "testing"

func TestShouldSyncSDS(t *testing.T) {
	t.Parallel()

	if ShouldSyncSDS(RequestPolicyInput{}) {
		t.Fatal("expected empty request not to sync SDS")
	}
	if !ShouldSyncSDS(RequestPolicyInput{SDS: SDSPolicyInput{VariantID: 12}}) {
		t.Fatal("expected variant id to enable SDS sync")
	}
	if !ShouldSyncSDS(RequestPolicyInput{SDS: SDSPolicyInput{VariantCount: 2}}) {
		t.Fatal("expected variant count to enable SDS sync")
	}
}

func TestShouldUseSDSCatalogSource(t *testing.T) {
	t.Parallel()

	if ShouldUseSDSCatalogSource(RequestPolicyInput{
		ProcessImages: true,
		SDS:           SDSPolicyInput{ProductName: "missing sync precondition"},
	}) {
		t.Fatal("expected SDS catalog source to require SDS sync eligibility")
	}

	if !ShouldUseSDSCatalogSource(RequestPolicyInput{
		SDS: SDSPolicyInput{
			VariantID:   99,
			ProductName: "studio tee",
		},
	}) {
		t.Fatal("expected SDS sync request with product facts to use catalog source")
	}
}

func TestShouldProcessImages(t *testing.T) {
	t.Parallel()

	if ShouldProcessImages(RequestPolicyInput{
		ProcessImages: true,
		ImageURLs:     []string{"https://example.com/a.png"},
		SDS: SDSPolicyInput{
			VariantID:   99,
			ProductName: "studio tee",
		},
	}) {
		t.Fatal("expected SDS catalog source request to skip image processing")
	}

	if !ShouldProcessImages(RequestPolicyInput{
		ProcessImages: true,
		ImageURLs:     []string{"https://example.com/a.png"},
	}) {
		t.Fatal("expected image request to process images")
	}
}

func TestShouldGenerateAssets(t *testing.T) {
	t.Parallel()

	if !ShouldGenerateAssets(RequestPolicyInput{
		ProcessImages: true,
	}) {
		t.Fatal("expected process-images request to generate assets")
	}

	if ShouldGenerateAssets(RequestPolicyInput{
		ProcessImages: true,
		SDS: SDSPolicyInput{
			VariantID:   99,
			ProductSKU:  "sku-1",
			CategoryPath: []string{"Apparel"},
		},
	}) {
		t.Fatal("expected SDS catalog source request to skip asset generation")
	}
}

func TestShouldUseStudioProductFallback(t *testing.T) {
	t.Parallel()

	if ShouldUseStudioProductFallback(RequestPolicyInput{
		ImageURLs: []string{"https://example.com/a.png"},
	}) {
		t.Fatal("expected image-only request not to enable studio fallback")
	}

	if !ShouldUseStudioProductFallback(RequestPolicyInput{
		ImageURLs: []string{"https://example.com/a.png"},
		SDS:       SDSPolicyInput{VariantID: 88},
	}) {
		t.Fatal("expected SDS image request to enable studio fallback")
	}
}

func TestShouldUseStudioCatalogCanonical(t *testing.T) {
	t.Parallel()

	if !ShouldUseStudioCatalogCanonical(RequestPolicyInput{
		ImageURLs: []string{"https://example.com/a.png"},
		SDS:       SDSPolicyInput{VariantID: 88},
	}) {
		t.Fatal("expected any SDS request to use studio catalog canonical")
	}

	if !ShouldUseStudioCatalogCanonical(RequestPolicyInput{
		ImageURLs: []string{"https://example.com/a.png"},
		SDS: SDSPolicyInput{
			VariantID:   88,
			ProductName: "studio tee",
		},
	}) {
		t.Fatal("expected SDS request with product facts to use studio catalog canonical")
	}
}

func TestShouldRunStudioInline(t *testing.T) {
	t.Parallel()

	if !ShouldRunStudioInline(RequestPolicyInput{
		ImageURLs:              []string{"https://example.com/a.png"},
		Platforms:              []string{"shein"},
		UseSheinStudioAIImages: true,
		SDS:                    SDSPolicyInput{VariantID: 88},
	}) {
		t.Fatal("expected shein single-platform studio request to run inline")
	}

	if ShouldRunStudioInline(RequestPolicyInput{
		ImageURLs:              []string{"https://example.com/a.png"},
		Platforms:              []string{"shein", "temu"},
		UseSheinStudioAIImages: true,
		SDS:                    SDSPolicyInput{VariantID: 88},
	}) {
		t.Fatal("expected multi-platform request not to run inline")
	}
}

func TestShouldRunRemoteSDSDesignSync(t *testing.T) {
	t.Parallel()

	if !ShouldRunRemoteSDSDesignSync(RequestPolicyInput{
		ImageURLs:                   []string{"https://example.com/a.png"},
		RenderSheinSizeImagesWithSDS: true,
		SDS:                         SDSPolicyInput{VariantID: 88},
	}) {
		t.Fatal("expected SDS request with remote size rendering to run remote sync")
	}

	if ShouldRunRemoteSDSDesignSync(RequestPolicyInput{
		ImageURLs:                   []string{"https://example.com/a.png"},
		RenderSheinSizeImagesWithSDS: false,
		SDS:                         SDSPolicyInput{VariantID: 88},
	}) {
		t.Fatal("expected request without remote size rendering not to run remote sync")
	}
}
