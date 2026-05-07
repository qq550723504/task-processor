package shein

import (
	"task-processor/internal/catalog/canonical"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

type CategoryAPI interface {
	GetCategory(categoryID int) (*sheincategory.CategoryInfo, error)
	GetCategoryTree() (*sheincategory.CategoryTreeResponse, error)
	SuggestCategoryByText(productInfo string) (*sheincategory.SuggestCategoryResponse, error)
}

type CategoryResolver interface {
	Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *CategoryResolution
}

type AttributeAPI interface {
	GetAttributeTemplates(categoryID int) (*sheinattribute.AttributeTemplateInfo, error)
	ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*sheinattribute.ValidateAttributeResponse, error)
	AddCustomAttributeValue(req *sheinattribute.AddCustomAttributeValueRequest) (*sheinattribute.AddCustomAttributeValueResponse, error)
}

type AttributeResolver interface {
	Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *AttributeResolution
}

type SaleAttributeResolver interface {
	Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution
}
