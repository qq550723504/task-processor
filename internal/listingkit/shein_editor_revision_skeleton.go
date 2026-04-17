package listingkit

type SheinEditorRevisionSkeleton struct {
	Platform string              `json:"platform"`
	Actor    string              `json:"actor,omitempty"`
	Reason   string              `json:"reason,omitempty"`
	Shein    *SheinRevisionInput `json:"shein,omitempty"`
}

func buildSheinEditorRevisionSkeleton(pkg *SheinPackage) *SheinEditorRevisionSkeleton {
	if pkg == nil {
		return nil
	}

	return &SheinEditorRevisionSkeleton{
		Platform: "shein",
		Actor:    "desktop-client",
		Reason:   "manual adjustment",
		Shein: &SheinRevisionInput{
			SpuName:                 stringPointerOrNil(pkg.SpuName),
			ProductNameEn:           stringPointerOrNil(pkg.ProductNameEn),
			BrandName:               stringPointerOrNil(pkg.BrandName),
			Description:             stringPointerOrNil(pkg.Description),
			Images:                  clonePlatformImageSetForEditor(pkg.Images),
			ProductAttributes:       append([]PlatformAttribute(nil), pkg.ProductAttributes...),
			ResolvedAttributes:      append([]SheinResolvedAttribute(nil), pkg.ResolvedAttributes...),
			CategoryResolution:      buildSheinCategoryResolutionPatch(pkg),
			AttributeResolution:     buildSheinAttributeResolutionPatch(pkg),
			SaleAttributeResolution: buildSheinSaleAttributeResolutionPatch(pkg),
			SKCPatches:              buildSheinEditorSKCPatches(pkg),
			ReviewNotes:             append([]string(nil), pkg.ReviewNotes...),
		},
	}
}

func stringPointerOrNil(value string) *string {
	if value == "" {
		return nil
	}
	copied := value
	return &copied
}
