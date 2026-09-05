package listingkit

import (
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog"
	"task-processor/internal/product/catalog/canonical"
)

func canonicalProductFromApprovedAssets(snapshot catalog.ProductSnapshot, inventory *productasset.ApprovedAssetInventory) *canonical.Product {
	product := catalog.ProjectCanonical(snapshot)
	if product == nil {
		return nil
	}
	product.Images = canonicalImagesFromApprovedAssets(inventory)
	for index := range product.Variants {
		product.Variants[index].Images = nil
	}
	return product
}

func canonicalImagesFromApprovedAssets(inventory *productasset.ApprovedAssetInventory) []canonical.Image {
	if inventory == nil {
		return nil
	}
	result := make([]canonical.Image, 0, len(inventory.Assets))
	appendRole := func(role productasset.Role) {
		for _, approved := range inventory.Assets {
			if approved.Role == role {
				result = append(result, canonical.Image{URL: approved.URL, Role: string(approved.Role)})
			}
		}
	}
	appendRole(productasset.RoleMain)
	appendRole(productasset.RoleWhiteBackground)
	appendRole(productasset.RoleDesign)
	appendRole(productasset.RoleGallery)
	return result
}
