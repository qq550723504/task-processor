package listingkit

import sheinpub "task-processor/internal/publishing/shein"

func (a *assembler) buildSheinAssemblerConfig() sheinpub.AssemblerConfig {
	return sheinpub.AssemblerConfig{
		CategoryResolver:      a.sheinCategoryResolver,
		AttributeResolver:     a.sheinAttributeResolver,
		SaleAttributeResolver: a.sheinSaleAttributeResolver,
		PricingPolicy:         a.sheinPricingPolicy,
		TitleOptimizer:        a.sheinTitleOptimizer,
	}
}
