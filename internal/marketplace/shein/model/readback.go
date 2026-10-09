package model

type ProductReadback struct {
	CategoryID       int64
	ProductTypeID    int64
	BrandCode        string
	SupplierCode     string
	Names            []LanguageContent
	SKCSupplierCodes map[string]string
	Product          PublishResult
}
