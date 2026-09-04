package listingkit

func shouldSyncSDS(req *GenerateRequest) bool {
	return req != nil && req.Options != nil && req.Options.SDS != nil &&
		(req.Options.SDS.VariantID > 0 || len(req.Options.SDS.Variants) > 0)
}

func shouldRunSDSDesignSync(req *GenerateRequest) bool {
	return shouldSyncSDS(req)
}
