package mapping

import (
	"strings"
	"testing"
	"time"

	"task-processor/internal/listingruntime"
)

type ownerCapturingMappingGateway struct {
	requests []*listingruntime.ProductImportMappingUpsert
}

func TestSmartRepairStrategyPropagatesStoreOwner(t *testing.T) {
	t.Parallel()

	gateway := &ownerCapturingMappingGateway{}
	strategy := NewSmartRepairStrategy(gateway, nil, nil, nil)
	result, err := strategy.Repair(&MappingRepairContext{
		Request: &MappingRepairRequest{
			TenantID: 1,
			StoreID:  2,
			SkuCode:  "SKU-1",
			Reason:   "repair",
		},
		StoreInfo: &listingruntime.StoreInfo{
			OwnerUserID: "zitadel-sub-1",
			Region:      "US",
		},
		StartTime: time.Now(),
	})
	if err != nil {
		t.Fatalf("Repair() error = %v", err)
	}
	if result == nil || !result.Success {
		t.Fatalf("result = %+v, want success", result)
	}
	if len(gateway.requests) != 1 || gateway.requests[0].OwnerUserID != "zitadel-sub-1" {
		t.Fatalf("requests = %+v, want owner zitadel-sub-1", gateway.requests)
	}
}

func (g *ownerCapturingMappingGateway) CreateMapping(req *listingruntime.ProductImportMappingUpsert) (int64, error) {
	g.requests = append(g.requests, req)
	return 41, nil
}

func (g *ownerCapturingMappingGateway) FindMappingByPlatformProductID(string, int64) (*listingruntime.ProductImportMapping, error) {
	return &listingruntime.ProductImportMapping{ID: 41}, nil
}

func TestMappingBuilderRejectsBlankOwnerBeforeGateway(t *testing.T) {
	t.Parallel()

	gateway := &ownerCapturingMappingGateway{}
	_, err := NewMappingBuilder(gateway).CreateMappingRelation(&MappingCreateOptions{
		TenantID:    1,
		StoreID:     2,
		OwnerUserID: "  ",
		SkuCode:     "SKU-1",
		Region:      "US",
	})
	if err == nil || !strings.Contains(err.Error(), "映射所有者不能为空") {
		t.Fatalf("error = %v, want owner validation error", err)
	}
	if len(gateway.requests) != 0 {
		t.Fatalf("gateway calls = %d, want 0", len(gateway.requests))
	}
}
