package listingkit

func buildAmazonExportPayloadFromInput(input amazonExportPayloadInput) *AmazonExportPayload {
	if input.draft == nil {
		return nil
	}
	return &AmazonExportPayload{
		Draft:       input.draft,
		ImageBundle: input.visualBase.imageBundle,
	}
}

func buildTemuExportPayloadFromInput(input reviewableExportPayloadInput, pkg *TemuPackage) *TemuExportPayload {
	if pkg == nil {
		return nil
	}
	return &TemuExportPayload{
		ImageBundle: input.visualBase.imageBundle,
		Package:     pkg,
	}
}

func buildWalmartExportPayloadFromInput(input reviewableExportPayloadInput, pkg *WalmartPackage) *WalmartExportPayload {
	if pkg == nil {
		return nil
	}
	return &WalmartExportPayload{
		ImageBundle: input.visualBase.imageBundle,
		Package:     pkg,
	}
}
