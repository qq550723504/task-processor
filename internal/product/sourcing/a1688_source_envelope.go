package sourcing

import (
	"strconv"
	"strings"

	alibaba1688model "task-processor/internal/crawler/alibaba1688/model"
)

const (
	Alibaba1688SourcePlatform = "1688"

	alibaba1688SourceReferenceType = "1688_product"
	alibaba1688ImageRolePrimary    = "primary"
	alibaba1688ImageRoleGallery    = "gallery"
	alibaba1688ImageRoleDetail     = "detail"
	alibaba1688ImageRoleVariant    = "variant"
	alibaba1688ImageRolePackage    = "package"
)

// Alibaba1688SourceEnvelopeInput is the product-sourcing view of one 1688
// crawler result. It preserves the source request context without making
// ListingKit or marketplace packages consume raw crawler payloads.
type Alibaba1688SourceEnvelopeInput struct {
	Request     Alibaba1688CrawlRequestInput
	Product     *alibaba1688model.Product1688
	RawSnapshot string
	SourceRunID string
	RequestID   string
	Error       error
}

// Alibaba1688SourceEnvelope maps one 1688 crawler product into the neutral
// source envelope used by catalog, asset, and ListingKit handoff code.
func Alibaba1688SourceEnvelope(input Alibaba1688SourceEnvelopeInput) SourceEnvelope {
	product := input.Product
	identity := alibaba1688SourceIdentity(input.Request, product)
	envelope := SourceEnvelope{
		Identity:     identity,
		RawReference: alibaba1688RawReference(input.Request, product, input.RawSnapshot),
		Trace: SourceTrace{
			SourceRunID: strings.TrimSpace(input.SourceRunID),
			RequestID:   strings.TrimSpace(input.RequestID),
		},
	}
	if product == nil {
		envelope.Warnings = append(envelope.Warnings, SourceWarning{Code: "missing_product", Message: "1688 source product is missing"})
		if input.Error != nil {
			envelope.Warnings = append(envelope.Warnings, SourceWarning{Code: "source_error", Message: input.Error.Error()})
		}
		return envelope.Normalize()
	}

	envelope.ProductCandidate = alibaba1688ProductCandidate(product)
	envelope.AssetCandidates = alibaba1688AssetCandidates(product)
	envelope.SupplierOrCostFacts = alibaba1688SupplierOrCostFacts(product)
	envelope.Warnings = alibaba1688SourceWarnings(identity, product, envelope, input.Error)
	return envelope.Normalize()
}

func alibaba1688SourceIdentity(input Alibaba1688CrawlRequestInput, product *alibaba1688model.Product1688) SourceIdentity {
	requestURL := NormalizeAlibaba1688URL(input.URL)
	productID := ExtractAlibaba1688ProductID(requestURL)
	sourceURL := requestURL
	if product != nil {
		if id := strings.TrimSpace(product.ID); id != "" {
			productID = id
		}
		if url := NormalizeAlibaba1688URL(product.URL); url != "" {
			sourceURL = url
			if productID == "" {
				productID = ExtractAlibaba1688ProductID(url)
			}
		}
	}
	if productID == "" {
		productID = sourceURL
	}
	return NormalizeSourceIdentity(SourceIdentity{
		SourceType:     SourceTypeCrawler,
		SourcePlatform: Alibaba1688SourcePlatform,
		SourceID:       productID,
		SourceURL:      sourceURL,
		Platform:       Alibaba1688SourcePlatform,
		Region:         "cn",
		ProductID:      productID,
		StoreID:        input.StoreID,
	})
}

func alibaba1688RawReference(input Alibaba1688CrawlRequestInput, product *alibaba1688model.Product1688, snapshot string) RawSourceReference {
	ref := RawSourceReference{
		ReferenceType: alibaba1688SourceReferenceType,
		SnapshotID:    strings.TrimSpace(snapshot),
		URL:           NormalizeAlibaba1688URL(input.URL),
	}
	if product != nil {
		ref.ReferenceID = strings.TrimSpace(product.ID)
		if url := NormalizeAlibaba1688URL(product.URL); url != "" {
			ref.URL = url
		}
	}
	if ref.ReferenceID == "" {
		ref.ReferenceID = ExtractAlibaba1688ProductID(ref.URL)
	}
	return ref
}

func alibaba1688ProductCandidate(product *alibaba1688model.Product1688) ProductCandidate {
	attributes := map[string]string{}
	addStringAttribute(attributes, "source_product_id", product.ID)
	addStringAttribute(attributes, "category", product.Category)
	addStringAttribute(attributes, "brand", product.Brand)
	addStringAttribute(attributes, "currency", default1688Currency(product.Currency))
	addStringAttribute(attributes, "unit", product.Unit)
	addStringAttribute(attributes, "shipping_from", product.ShippingInfo.ShippingFrom)
	addStringAttribute(attributes, "processing_time", product.ShippingInfo.ProcessingTime)
	addBoolAttribute(attributes, "is_customized", product.IsCustomized)
	addIntAttribute(attributes, "min_order_quantity", product.MinOrderQuantity)
	addIntAttribute(attributes, "sales_volume", product.SalesVolume)
	addIntAttribute(attributes, "review_count", product.ReviewCount)
	addFloatAttribute(attributes, "rating", product.Rating)
	addFloatAttribute(attributes, "min_price", product.MinPrice)
	addFloatAttribute(attributes, "max_price", product.MaxPrice)
	if len(product.Keywords) > 0 {
		addStringAttribute(attributes, "keywords", strings.Join(trimNonEmptyStrings(product.Keywords), ","))
	}
	for _, spec := range product.Specifications {
		name := strings.TrimSpace(spec.Name)
		value := strings.TrimSpace(spec.Value)
		if name == "" || value == "" {
			continue
		}
		attributes["spec:"+name] = value
	}
	if product.PackInfo != nil {
		addStringAttribute(attributes, "package_type", product.PackInfo.PackageType)
		addFloatAttribute(attributes, "package_weight_grams", product.PackInfo.Weight)
		addStringAttribute(attributes, "package_instructions", product.PackInfo.Instructions)
	}

	return ProductCandidate{
		Title:       strings.TrimSpace(product.Title),
		Description: build1688Description(product),
		Brand:       strings.TrimSpace(product.Brand),
		Attributes:  attributes,
		Variants:    alibaba1688VariantCandidates(product.Variants),
	}
}

func alibaba1688VariantCandidates(variants []alibaba1688model.Variant) []ProductVariantCandidate {
	if len(variants) == 0 {
		return nil
	}
	candidates := make([]ProductVariantCandidate, 0, len(variants))
	for idx, variant := range variants {
		attributes := convert1688VariantAttributes(variant.Attributes)
		candidate := ProductVariantCandidate{
			SourceID:   buildScrapedVariantSKU(idx, attributes),
			Title:      strings.TrimSpace(variant.Name),
			SKU:        buildScrapedVariantSKU(idx, attributes),
			Attributes: attributes,
		}
		if candidate.Title == "" && len(candidate.Attributes) == 0 && strings.TrimSpace(variant.Image) == "" && variant.Price <= 0 && variant.Stock == 0 {
			continue
		}
		if variant.Stock > 0 {
			candidate.Attributes["stock"] = strconv.Itoa(variant.Stock)
		}
		if variant.Price > 0 {
			candidate.Attributes["price"] = formatOptionalPrice(variant.Price)
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func alibaba1688AssetCandidates(product *alibaba1688model.Product1688) []AssetCandidate {
	seen := map[string]struct{}{}
	assets := make([]AssetCandidate, 0, len(product.Images)+len(product.ProductDetails)+len(product.Variants)+len(product.Videos)+2)
	appendAsset := func(url, role, mediaType string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		assets = append(assets, AssetCandidate{SourceID: url, URL: url, MediaType: mediaType, Role: role})
	}
	appendAsset(product.MainImage, alibaba1688ImageRolePrimary, "image")
	for _, image := range product.Images {
		appendAsset(image, alibaba1688ImageRoleGallery, "image")
	}
	for _, detail := range product.ProductDetails {
		for _, image := range detail.Images {
			appendAsset(image, alibaba1688ImageRoleDetail, "image")
		}
	}
	for _, variant := range product.Variants {
		appendAsset(variant.Image, alibaba1688ImageRoleVariant, "image")
	}
	if product.PackInfo != nil {
		for _, image := range product.PackInfo.PackageImages {
			appendAsset(image, alibaba1688ImageRolePackage, "image")
		}
	}
	for _, video := range product.Videos {
		appendAsset(video.CoverURL, "video_cover", "image")
		appendAsset(video.VideoURL, "video", "video")
	}
	return assets
}

func alibaba1688SupplierOrCostFacts(product *alibaba1688model.Product1688) SupplierOrCostFacts {
	facts := map[string]string{}
	addStringAttribute(facts, "company_name", product.Supplier.CompanyName)
	addStringAttribute(facts, "location", product.Supplier.Location)
	addStringAttribute(facts, "shop_url", product.Supplier.ShopURL)
	addStringAttribute(facts, "card_type", product.Supplier.CardType)
	addIntAttribute(facts, "years_in_business", product.Supplier.YearsInBusiness)
	addFloatAttribute(facts, "supplier_rating", product.Supplier.Rating)
	addFloatAttribute(facts, "response_rate", product.Supplier.ResponseRate)
	addBoolAttribute(facts, "is_gold_supplier", product.Supplier.IsGoldSupplier)
	addBoolAttribute(facts, "is_verified", product.Supplier.IsVerified)
	addIntAttribute(facts, "min_order_quantity", product.MinOrderQuantity)
	addStringAttribute(facts, "unit", product.Unit)
	if len(product.PriceRanges) > 0 {
		facts["price_range_count"] = strconv.Itoa(len(product.PriceRanges))
	}
	return SupplierOrCostFacts{
		SupplierID:   strings.TrimSpace(product.Supplier.ID),
		SupplierName: strings.TrimSpace(product.Supplier.Name),
		Currency:     default1688Currency(product.Currency),
		Cost:         formatOptionalPrice(product.MinPrice),
		Price:        formatOptionalPrice(product.MinPrice),
		Facts:        facts,
	}
}

func alibaba1688SourceWarnings(identity SourceIdentity, product *alibaba1688model.Product1688, envelope SourceEnvelope, err error) []SourceWarning {
	warnings := []SourceWarning{}
	if err != nil {
		warnings = append(warnings, SourceWarning{Code: "source_error", Message: err.Error()})
	}
	if identity.Validation().MissingSourceID {
		warnings = append(warnings, SourceWarning{Code: "missing_source_id", Field: "id", Message: "1688 source product is missing product id"})
	}
	if strings.TrimSpace(product.Title) == "" {
		warnings = append(warnings, SourceWarning{Code: "missing_title", Field: "title", Message: "1688 source product is missing title"})
	}
	if len(envelope.AssetCandidates) == 0 {
		warnings = append(warnings, SourceWarning{Code: "missing_assets", Field: "images", Message: "1688 source product has no image assets"})
	}
	if product.MinPrice <= 0 {
		warnings = append(warnings, SourceWarning{Code: "missing_cost", Field: "min_price", Message: "1688 source product is missing minimum price"})
	}
	return warnings
}

func addIntAttribute(attributes map[string]string, key string, value int) {
	if value != 0 {
		attributes[key] = strconv.Itoa(value)
	}
}

func addFloatAttribute(attributes map[string]string, key string, value float64) {
	if value > 0 {
		attributes[key] = strconv.FormatFloat(value, 'f', -1, 64)
	}
}

func addBoolAttribute(attributes map[string]string, key string, value bool) {
	if value {
		attributes[key] = strconv.FormatBool(value)
	}
}
