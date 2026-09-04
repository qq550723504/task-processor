package imageagentworker

import (
	policycatalog "task-processor/internal/integration/policy/productimage"
	imagepolicy "task-processor/internal/marketplace/imagepolicy"
)

func loadEmbeddedImagePolicyResolver() (*imagepolicy.Resolver, error) {
	return policycatalog.LoadEmbeddedResolver()
}

func newImagePolicyResolver(catalog policycatalog.Catalog) (*imagepolicy.Resolver, error) {
	return policycatalog.NewResolver(catalog)
}
