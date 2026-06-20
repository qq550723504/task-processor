package pricing

import (
	"context"
	"testing"

	"task-processor/internal/listingadmin"
	managementapi "task-processor/internal/ports/managementapi"

	"github.com/sirupsen/logrus"
)

func TestStoreConfigDTOFromListingStore(t *testing.T) {
	enableRebargain := true
	store := &listingadmin.Store{
		ID:                      7,
		TenantID:                9,
		PriceType:               "original",
		TemuPriceRejectStrategy: "REJECT",
		EnableRebargain:         &enableRebargain,
	}

	dto := storeConfigDTOFromListingStore(store)
	if dto == nil {
		t.Fatal("storeConfigDTOFromListingStore() = nil")
	}
	if dto.ID != 7 || dto.TenantID != 9 || dto.PriceType != "original" || dto.TemuPriceRejectStrategy != "REJECT" {
		t.Fatalf("unexpected dto: %+v", dto)
	}
	if dto.EnableRebargain == nil || !*dto.EnableRebargain {
		t.Fatalf("EnableRebargain = %v, want true", dto.EnableRebargain)
	}
}

func TestStoreConfigHelpersUseDefaults(t *testing.T) {
	service := &StoreConfigService{
		logger:      logrus.NewEntry(logrus.New()),
		storeConfig: &managementapi.StoreRespDTO{},
	}

	if service.IsRebargainEnabled() {
		t.Fatal("IsRebargainEnabled() = true, want false")
	}
	if got := service.GetPriceType(); got != "special" {
		t.Fatalf("GetPriceType() = %q, want special", got)
	}
	if got := service.GetPriceRejectStrategy(); got != "KEEP_ONLINE" {
		t.Fatalf("GetPriceRejectStrategy() = %q, want KEEP_ONLINE", got)
	}
}

var _ = context.Background
