package listingkit

import "testing"

func TestBuildStudioBatchTaskGenerateRequestIncludesOwnerContext(t *testing.T) {
	t.Parallel()

	req := buildStudioBatchTaskGenerateRequest(
		&SheinStudioSession{
			TenantID:     "tenant-1",
			UserID:       "user-1",
			Prompt:       "prompt",
			SheinStoreID: "870",
		},
		SheinStudioGroupedSelection{
			SheinStoreID: "870",
			Selection: SheinStudioSelection{
				ProductName:     "wallet",
				VariantID:       1,
				ParentProductID: 2,
			},
		},
		StudioMaterializedDesignRecord{
			ID:               "design-1",
			ImageURL:         "https://example.com/design.png",
			TargetGroupLabel: "group-a",
		},
		SheinStudioDesign{},
	)

	if req == nil {
		t.Fatal("request is nil")
	}
	if req.TenantID != "tenant-1" {
		t.Fatalf("TenantID = %q, want tenant-1", req.TenantID)
	}
	if req.UserID != "user-1" {
		t.Fatalf("UserID = %q, want user-1", req.UserID)
	}
}
