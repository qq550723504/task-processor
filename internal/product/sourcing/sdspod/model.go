package sdspod

type CanonicalMetadata struct {
	ProductName   string
	ProductSKU    string
	VariantSKU    string
	VariantSize   string
	VariantColor  string
	StyleName     string
	Attributes    map[string]string
	MockupURLs    []string
	Variants      []VariantMetadata
	VariantLookup []VariantLookup
}

type VariantMetadata struct {
	SKU        string
	Color      string
	Status     string
	MockupURLs []string
}

type VariantLookup struct {
	CanonicalVariantIndex int
	Keys                  []string
}
