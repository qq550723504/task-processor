package httpapi

import (
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit"
	sheinpub "task-processor/internal/publishing/shein"
	sheinclient "task-processor/internal/shein/client"
)

type listingKitSheinAPIClientFactory struct {
	client *management.ClientManager
}

func (f listingKitSheinAPIClientFactory) NewSheinAPIClient(storeID int64, storeInfo *listingkit.SheinStoreInfo) *sheinclient.APIClient {
	if storeInfo == nil {
		return sheinclient.NewAPIClient(storeID, f.client)
	}
	return sheinclient.NewAPIClientWithStoreInfo(storeID, f.client, &managementapi.StoreRespDTO{
		ID:       storeInfo.ID,
		TenantID: storeInfo.TenantID,
		StoreID:  storeInfo.StoreID,
		Name:     storeInfo.Name,
		Platform: storeInfo.Platform,
		Region:   storeInfo.Region,
		LoginUrl: storeInfo.LoginURL,
		Proxy:    storeInfo.Proxy,
	})
}

func buildListingKitSheinCategoryResolver(client *management.ClientManager, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.CategoryResolver {
	return sheinpub.NewCachedCategoryResolver(sheinpub.NewManagedCategoryResolver(client, llm), cache)
}

func buildListingKitSheinAttributeResolver(client *management.ClientManager, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.AttributeResolver {
	return sheinpub.NewCachedAttributeResolver(sheinpub.NewManagedAttributeResolver(client, llm), cache)
}

func buildListingKitSheinSaleAttributeResolver(client *management.ClientManager, llm openaiclient.ChatCompleter, cache sheinpub.ResolutionCacheStore) sheinpub.SaleAttributeResolver {
	return sheinpub.NewCachedSaleAttributeResolver(sheinpub.NewManagedSaleAttributeResolver(client, llm), cache)
}

func buildListingKitSheinProductAPIBuilder(client *management.ClientManager) sheinpub.ProductAPIBuilder {
	return sheinpub.NewManagedProductAPIBuilder(client)
}

func buildListingKitSheinImageAPIBuilder(client *management.ClientManager) sheinpub.ImageAPIBuilder {
	return sheinpub.NewManagedImageAPIBuilder(client)
}

func buildListingKitSheinTranslateAPIBuilder(client *management.ClientManager) sheinpub.TranslateAPIBuilder {
	return sheinpub.NewManagedTranslateAPIBuilder(client)
}
