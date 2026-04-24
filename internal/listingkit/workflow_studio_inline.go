package listingkit

func shouldRunStudioInline(req *GenerateRequest) bool {
	if req == nil || req.Options == nil {
		return false
	}
	if !shouldSyncSDS(req) || req.Options.ProcessImages {
		return false
	}
	if len(req.ImageURLs) == 0 || len(req.Platforms) != 1 {
		return false
	}
	return req.Platforms[0] == "shein"
}
