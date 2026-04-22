package shein

import (
	"task-processor/internal/productenrich"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

type CategoryAPI interface {
	GetCategory(categoryID int) (*sheincategory.CategoryInfo, error)
	GetCategoryTree() (*sheincategory.CategoryTreeResponse, error)
	SuggestCategoryByText(productInfo string) (*sheincategory.SuggestCategoryResponse, error)
}

type CategoryResolver interface {
	Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *CategoryResolution
}

type AttributeAPI interface {
	GetAttributeTemplates(categoryID int) (*sheinattribute.AttributeTemplateInfo, error)
}

type AttributeResolver interface {
	Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *AttributeResolution
}

type SaleAttributeResolver interface {
	Resolve(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) *SaleAttributeResolution
}
