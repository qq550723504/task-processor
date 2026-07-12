package listingkit

import (
	"context"
	"testing"
	"time"

	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sizeAttributeFallbackStore struct {
	submitResolutionCacheStore
	fallback *sheinpub.SheinResolutionCacheEntry
}

func (s *sizeAttributeFallbackStore) GetManualResolutionCacheBySourceIdentity(context.Context, string, string, string) (*sheinpub.SheinResolutionCacheEntry, error) {
	return nil, nil
}

func (s *sizeAttributeFallbackStore) GetManualResolutionCacheByProductIdentity(context.Context, string, string, int, []string) (*sheinpub.SheinResolutionCacheEntry, error) {
	return s.fallback, nil
}

func TestLoadSheinSizeAttributeCacheFallsBackToPublishedProductIdentity(t *testing.T) {
	t.Parallel()

	sID, fiveXLID := 568, 1430561
	review := &sheinpub.SizeAttributeReview{Ready: true, Attributes: []sheinproduct.SizeAttribute{
		{AttributeID: 55, RelateSaleAttributeID: 87, RelateSaleAttributeValueID: sID, AttributeExtraValue: "69.5"},
		{AttributeID: 55, RelateSaleAttributeID: 87, RelateSaleAttributeValueID: fiveXLID, AttributeExtraValue: "80"},
	}}
	store := &sizeAttributeFallbackStore{fallback: &sheinpub.SheinResolutionCacheEntry{
		CacheKind:      sheinpub.ResolutionCacheKindSizeAttribute,
		CacheKey:       "legacy-key",
		Source:         "manual_cache",
		Manual:         true,
		ResolutionJSON: mustMarshalSheinSizeAttributeReview(review),
		UpdatedAt:      time.Now(),
	}}
	svc, err := NewService(newTestServiceConfig(&stubSubmitRepo{}, withTestConfig(func(cfg *ServiceConfig) {
		cfg.Shein.SheinResolutionCacheStore = store
	})))
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	pkg := &sheinpub.Package{
		CategoryID: 1860,
		DraftPayload: &sheinpub.RequestDraft{SizeAttributeList: []sheinproduct.SizeAttribute{
			{AttributeID: 55, RelateSaleAttributeID: 87, RelateSaleAttributeValueID: sID, AttributeExtraValue: "69.5"},
		}},
		SaleAttributeResolution: &sheinpub.SaleAttributeResolution{
			SecondaryAttributeID: 87,
			SKUValueAssignments: map[string]sheinpub.ResolvedSaleAttribute{
				"s":   {AttributeID: 87, AttributeValueID: &sID},
				"5xl": {AttributeID: 87, AttributeValueID: &fiveXLID},
			},
		},
	}

	got := svc.(*service).loadSheinSizeAttributeCache(&GenerateRequest{SheinStoreID: 1043}, pkg)
	if got == nil || len(got.Attributes) != 2 {
		t.Fatalf("cached review = %+v, want two published rows", got)
	}
}
