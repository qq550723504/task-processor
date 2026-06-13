package amazonlisting

import (
	coreconfig "task-processor/internal/core/config"
	amazonpublishing "task-processor/internal/marketplace/amazon/publishing"
)

func NewSPAPISubmitter(cfg *coreconfig.Config) ListingSubmitter {
	return amazonpublishing.NewSPAPISubmitter(cfg)
}
