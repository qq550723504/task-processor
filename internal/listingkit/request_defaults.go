package listingkit

import "strings"

type generateRequestDefaults struct {
	sheinDefaultStoreID int64
}

func applyGenerateRequestDefaults(req *GenerateRequest, defaults generateRequestDefaults) {
	if req == nil {
		return
	}
	normalizeGenerateRequest(req)
	if req.SheinStoreID > 0 {
		return
	}
	if defaults.sheinDefaultStoreID <= 0 {
		return
	}
	for _, platform := range req.Platforms {
		if strings.EqualFold(strings.TrimSpace(platform), "shein") {
			req.SheinStoreID = defaults.sheinDefaultStoreID
			return
		}
	}
}
