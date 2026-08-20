package sourcing

// Alibaba1688ProductSnapshot contains the 1688 facts consumed by product
// sourcing. It deliberately contains no crawler execution or ListingKit types.
type Alibaba1688ProductSnapshot struct {
	ID               string
	Title            string
	URL              string
	Images           []string
	MainImage        string
	Videos           []Alibaba1688VideoSnapshot
	PriceRangeCount  int
	MinPrice         float64
	MaxPrice         float64
	Currency         string
	MinOrderQuantity int
	Unit             string
	Supplier         Alibaba1688SupplierSnapshot
	Specifications   []Alibaba1688SpecificationSnapshot
	ProductDetails   []Alibaba1688ProductDetailSnapshot
	PackInfo         *Alibaba1688PackInfoSnapshot
	VariationValues  []Alibaba1688VariationValueSnapshot
	Variants         []Alibaba1688VariantSnapshot
	SalesVolume      int
	ReviewCount      int
	Rating           float64
	Shipping         Alibaba1688ShippingSnapshot
	Category         string
	Brand            string
	Keywords         []string
	IsCustomized     bool
}

type Alibaba1688VideoSnapshot struct {
	VideoURL string
	CoverURL string
}

type Alibaba1688SupplierSnapshot struct {
	ID              string
	Name            string
	CompanyName     string
	Location        string
	ShopURL         string
	CardType        string
	YearsInBusiness int
	Rating          float64
	ResponseRate    float64
	IsGoldSupplier  bool
	IsVerified      bool
}

type Alibaba1688SpecificationSnapshot struct {
	Name  string
	Value string
}

type Alibaba1688ProductDetailSnapshot struct {
	Content string
	Images  []string
}

type Alibaba1688PackInfoSnapshot struct {
	PackageType   string
	Weight        float64
	PackageImages []string
	Instructions  string
}

type Alibaba1688VariationValueSnapshot struct {
	Name   string
	Values []string
}

type Alibaba1688VariantSnapshot struct {
	Attributes map[string]any
	Name       string
	Image      string
	Stock      int
	Price      float64
}

type Alibaba1688ShippingSnapshot struct {
	ShippingFrom   string
	ProcessingTime string
}
