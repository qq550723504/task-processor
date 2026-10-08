package main

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/commercial/billing"
	commercialstore "task-processor/internal/integration/persistence/commercialbilling"
	"task-processor/internal/ledger/orgresource"
)

func TestFreshOwnerInstallsRetailPricesAndPreservesOriginalQuote(t *testing.T) {
	ctx := context.Background()
	moneyDB, commercialDB := openOwnerSchemaTestDB(t, "retail-money"), openOwnerSchemaTestDB(t, "retail-commercial")
	require.NoError(t, migrateOwnerSchemas(moneyDB, commercialDB))
	store, err := commercialstore.New(commercialDB)
	require.NoError(t, err)
	for _, want := range []struct {
		id              string
		price, quantity int64
	}{
		{"store-service-30d-v1", 16800, 2}, {"ai-point-v1", 1, 500}, {"data-row-1688-server-v1", 5, 3},
	} {
		offer, err := store.ReadOffer(ctx, want.id)
		require.NoError(t, err)
		require.Equal(t, want.price, offer.UnitPriceMinor)
		quote, err := store.CreateQuote(ctx, billing.QuoteRequest{OrganizationID: "retail-org", OfferID: want.id, Quantity: want.quantity})
		require.NoError(t, err)
		require.Equal(t, want.price*want.quantity, quote.TotalMinor)
		originalVersion := offer.PricingVersion
		offer.UnitPriceMinor++
		offer.PricingVersion = "owner-next"
		offer.Status = billing.OfferDisabled
		require.NoError(t, store.SaveOffer(ctx, offer))
		require.NoError(t, migrateOwnerSchemas(moneyDB, commercialDB))
		var got billing.Offer
		require.NoError(t, commercialDB.Table("commercial_offers").Where("offer_id = ?", want.id).Take(&got).Error)
		require.Equal(t, offer, got, "restart must not overwrite or reactivate owner pricing")
		frozen, err := store.ReadQuote(ctx, "retail-org", quote.QuoteID)
		require.NoError(t, err)
		require.Equal(t, want.price*want.quantity, frozen.TotalMinor)
		require.Equal(t, originalVersion, frozen.PricingVersion)
	}
}

func TestRetailInstallRejectsCompetingPricesBeforeInsertingAnyProduct(t *testing.T) {
	for _, kind := range []billing.ProductKind{billing.ProductStoreRenewalPeriod, billing.ProductAIPoint, billing.ProductDataRow} {
		t.Run(string(kind), func(t *testing.T) {
			ctx := context.Background()
			moneyDB, commercialDB := openOwnerSchemaTestDB(t, "conflict-money"), openOwnerSchemaTestDB(t, "conflict-commercial")
			require.NoError(t, commercialstore.AutoMigrate(commercialDB))
			store, err := commercialstore.New(commercialDB)
			require.NoError(t, err)
			resource, _ := billing.ResourceTypeForProduct(kind)
			future := time.Now().UTC().Add(24 * time.Hour)
			existing := billing.Offer{OfferID: "owner-scheduled", ProductKind: kind, ResourceType: resource, Currency: "CNY", UnitPriceMinor: 7, PricingVersion: "owner-v1", MinQuantity: 1, MaxQuantity: 100, Status: billing.OfferActive, StartsAt: &future}
			require.NoError(t, store.CreateOffer(ctx, existing))
			require.Error(t, migrateOwnerSchemas(moneyDB, commercialDB))
			var count int64
			require.NoError(t, commercialDB.Table("commercial_offers").Count(&count).Error)
			require.Equal(t, int64(1), count)
			var got billing.Offer
			require.NoError(t, commercialDB.Table("commercial_offers").Where("offer_id = ?", existing.OfferID).Take(&got).Error)
			require.Equal(t, int64(7), got.UnitPriceMinor)
		})
	}
}

func TestRetailInstallRollsBackEarlierInsertWhenLaterOfferIDIsOccupied(t *testing.T) {
	ctx := context.Background()
	moneyDB, commercialDB := openOwnerSchemaTestDB(t, "occupied-money"), openOwnerSchemaTestDB(t, "occupied-commercial")
	require.NoError(t, commercialstore.AutoMigrate(commercialDB))
	store, err := commercialstore.New(commercialDB)
	require.NoError(t, err)
	occupied := billing.Offer{OfferID: "ai-point-v1", ProductKind: billing.ProductDataRow, ResourceType: orgresource.ResourceDataRow, Currency: "CNY", UnitPriceMinor: 7, PricingVersion: "unrelated", MinQuantity: 1, MaxQuantity: 100, Status: billing.OfferDisabled}
	require.NoError(t, store.CreateOffer(ctx, occupied))
	require.Error(t, migrateOwnerSchemas(moneyDB, commercialDB))
	_, err = store.ReadOffer(ctx, "store-service-30d-v1")
	require.Error(t, err, "earlier product insertion must be rolled back")
	var got billing.Offer
	require.NoError(t, commercialDB.Table("commercial_offers").Where("offer_id = ?", occupied.OfferID).Take(&got).Error)
	require.Equal(t, occupied, got)
}
