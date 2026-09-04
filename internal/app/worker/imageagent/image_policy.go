package imageagentworker

import (
	policyassembly "task-processor/internal/app/imageagentpolicy"
	policycatalog "task-processor/internal/integration/policy/productimage"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
)

func loadEmbeddedImagePolicyResolver() (*imagepolicy.Resolver, error) {
	return policyassembly.LoadEmbeddedResolver()
}

func newImagePolicyResolver(catalog policycatalog.Catalog) (*imagepolicy.Resolver, error) {
	return policyassembly.NewResolver(catalog)
}
