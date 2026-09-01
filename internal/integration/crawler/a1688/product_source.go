package a1688

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"task-processor/internal/product/catalog/canonical"
	"task-processor/internal/product/sourcing"
	"task-processor/internal/productenrich"
)

// Alibaba1688ProductSnapshot contains the 1688 facts consumed by product
// sourcing. It deliberately contains no crawler execution or ListingKit types.
type Alibaba1688ProductSnapshot struct {
	ID               string
	Title            string
	URL              string
	Images           []string
	MainImage        string
	Videos           []Alibaba1688VideoSnapshot
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

// Convert1688ProductToScrapedData normalizes a raw 1688 crawler product into
// the product enrichment scraped-data contract.
func Convert1688ProductToScrapedData(product *Alibaba1688ProductSnapshot) *productenrich.ScrapedData {
	if product == nil {
		return nil
	}
	images := normalize1688Images(product.Images)

	specs := make(map[string]string, len(product.Specifications))
	for _, sp := range product.Specifications {
		name := strings.TrimSpace(sp.Name)
		value := strings.TrimSpace(sp.Value)
		if name == "" || value == "" {
			continue
		}
		specs[name] = value
	}

	return &productenrich.ScrapedData{
		Title:             product.Title,
		Category:          product.Category,
		Description:       build1688Description(product),
		Images:            images,
		Price:             product.MinPrice,
		Specs:             specs,
		VariantDimensions: build1688VariantDimensions(product.VariationValues),
		Variants:          build1688ScrapedVariants(product, images),
	}
}

func build1688Description(product *Alibaba1688ProductSnapshot) string {
	if len(product.ProductDetails) == 0 {
		return product.Title
	}
	var sb strings.Builder
	for _, d := range product.ProductDetails {
		content := strings.TrimSpace(d.Content)
		if content == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(content)
	}
	if sb.Len() == 0 {
		return product.Title
	}
	return sb.String()
}

func build1688VariantDimensions(values []Alibaba1688VariationValueSnapshot) []canonical.ScrapedVariantDimension {
	if len(values) == 0 {
		return nil
	}

	dimensions := make([]canonical.ScrapedVariantDimension, 0, len(values))
	for _, item := range values {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}

		dimension := canonical.ScrapedVariantDimension{Name: name}
		seen := make(map[string]struct{}, len(item.Values))
		for _, raw := range item.Values {
			value := strings.TrimSpace(raw)
			if value == "" {
				continue
			}
			if _, exists := seen[value]; exists {
				continue
			}
			seen[value] = struct{}{}
			dimension.Values = append(dimension.Values, value)
		}
		if len(dimension.Values) == 0 {
			continue
		}
		dimensions = append(dimensions, dimension)
	}

	if len(dimensions) == 0 {
		return nil
	}
	return dimensions
}

func build1688ScrapedVariants(product *Alibaba1688ProductSnapshot, fallbackImages []string) []productenrich.ProductVariant {
	if product == nil || len(product.Variants) == 0 {
		return nil
	}

	variants := make([]productenrich.ProductVariant, 0, len(product.Variants))
	for idx, variant := range product.Variants {
		converted := productenrich.ProductVariant{
			Attributes: convert1688VariantAttributes(variant.Attributes),
			Stock:      variant.Stock,
			Images:     collect1688VariantImages(variant, fallbackImages),
			IsDefault:  idx == 0,
		}
		converted.SKU = buildScrapedVariantSKU(idx, converted.Attributes)
		if variant.Price > 0 {
			converted.Price = &canonical.PriceInfo{
				Currency:  default1688Currency(product.Currency),
				Amount:    variant.Price,
				CostPrice: variant.Price,
			}
		}
		variants = append(variants, converted)
	}

	if len(variants) == 0 {
		return nil
	}
	return variants
}

func normalize1688Images(images []string) []string {
	if len(images) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(images))
	seen := make(map[string]struct{}, len(images))
	for _, raw := range images {
		image := strings.TrimSpace(raw)
		if image == "" {
			continue
		}
		if _, exists := seen[image]; exists {
			continue
		}
		seen[image] = struct{}{}
		normalized = append(normalized, image)
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func convert1688VariantAttributes(attributes map[string]any) map[string]string {
	if len(attributes) == 0 {
		return map[string]string{}
	}

	converted := make(map[string]string, len(attributes))
	for key, raw := range attributes {
		name := strings.TrimSpace(key)
		value := strings.TrimSpace(stringify1688VariantValue(raw))
		if name == "" || value == "" {
			continue
		}
		converted[name] = value
	}
	return converted
}

func stringify1688VariantValue(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int8:
		return strconv.FormatInt(int64(v), 10)
	case int16:
		return strconv.FormatInt(int64(v), 10)
	case int32:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint:
		return strconv.FormatUint(uint64(v), 10)
	case uint8:
		return strconv.FormatUint(uint64(v), 10)
	case uint16:
		return strconv.FormatUint(uint64(v), 10)
	case uint32:
		return strconv.FormatUint(uint64(v), 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case bool:
		return strconv.FormatBool(v)
	default:
		return fmt.Sprint(v)
	}
}

func collect1688VariantImages(variant Alibaba1688VariantSnapshot, fallback []string) []string {
	images := make([]string, 0, 2)
	if image := strings.TrimSpace(variant.Image); image != "" {
		images = append(images, image)
	}
	if len(images) == 0 && len(fallback) > 0 {
		if image := strings.TrimSpace(fallback[0]); image != "" {
			images = append(images, image)
		}
	}
	if len(images) == 0 {
		return nil
	}
	return images
}

func default1688Currency(currency string) string {
	currency = strings.TrimSpace(currency)
	if currency == "" {
		return "CNY"
	}
	return currency
}

func buildScrapedVariantSKU(index int, attributes map[string]string) string {
	if len(attributes) == 0 {
		return fmt.Sprintf("SCRAPED-%03d", index+1)
	}

	keys := make([]string, 0, len(attributes))
	for key := range attributes {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	parts := []string{"SCRAPED"}
	replacer := strings.NewReplacer(" ", "-", "/", "-", "\\", "-", ",", "-", "|", "-", ";", "-")
	for _, key := range keys {
		token := strings.ToUpper(strings.TrimSpace(attributes[key]))
		token = replacer.Replace(token)
		if token == "" {
			continue
		}
		parts = append(parts, token)
	}
	if len(parts) == 1 {
		return fmt.Sprintf("SCRAPED-%03d", index+1)
	}
	parts = append(parts, fmt.Sprintf("%03d", index+1))
	return strings.Join(parts, "-")
}

var alibaba1688OfferIDPattern = regexp.MustCompile(`(?i)(?:/offer/|offer[/=])(\d+)`)

// Alibaba1688CrawlRequestInput is the source-side request context for a 1688 product URL.
type Alibaba1688CrawlRequestInput struct {
	URL       string `json:"url"`
	AccountID int64  `json:"account_id,omitempty"`
	// StoreID is reserved for neutral source identity only. It is not accepted
	// as the 1688 login-account identifier at the HTTP/application boundary.
	StoreID int64 `json:"store_id,omitempty"`
}

// Alibaba1688SourceProductResult is a normalized 1688 crawler result with its
// source identity.
type Alibaba1688SourceProductResult struct {
	Identity sourcing.SourceIdentity
	Product  *Alibaba1688ProductSnapshot
	Error    error
}

// Alibaba1688CrawlResultInput is the neutral result shape passed from a 1688
// integration adapter into product sourcing.
type Alibaba1688CrawlResultInput struct {
	Product *Alibaba1688ProductSnapshot
	Error   error
}

// Alibaba1688SourceRequest builds the stable source request identity for a
// 1688 product URL. Standard offer URLs use the numeric offer ID; non-standard
// URLs fall back to the cleaned URL so the source remains traceable.
func Alibaba1688SourceRequest(input Alibaba1688CrawlRequestInput) sourcing.SourceRequest {
	cleanURL := NormalizeAlibaba1688URL(input.URL)
	productID := ExtractAlibaba1688ProductID(cleanURL)
	if productID == "" {
		productID = cleanURL
	}
	return sourcing.SourceRequest{
		Platform:  "1688",
		Region:    "cn",
		ProductID: productID,
		StoreID:   input.StoreID,
	}
}

// NormalizeAlibaba1688SourceResult attaches a stable source identity to one
// raw 1688 crawler result.
func NormalizeAlibaba1688SourceResult(input Alibaba1688CrawlRequestInput, product *Alibaba1688ProductSnapshot, err error) Alibaba1688SourceProductResult {
	return Alibaba1688SourceProductResult{
		Identity: Alibaba1688SourceRequest(input).Identity(),
		Product:  product,
		Error:    err,
	}
}

// NormalizeAlibaba1688BatchResults aligns 1688 batch results with the
// requested source identities. Missing trailing results become empty source
// results, preserving request/result accounting without guessing failures.
func NormalizeAlibaba1688BatchResults(requests []Alibaba1688CrawlRequestInput, results []Alibaba1688CrawlResultInput) []Alibaba1688SourceProductResult {
	normalized := make([]Alibaba1688SourceProductResult, 0, len(requests))
	for index, req := range requests {
		item := NormalizeAlibaba1688SourceResult(req, nil, nil)
		if index < len(results) {
			item.Product = results[index].Product
			item.Error = results[index].Error
		}
		normalized = append(normalized, item)
	}
	return normalized
}

// NormalizeAlibaba1688URL trims and lightly normalizes a 1688 source URL for
// identity fallback. It intentionally does not validate crawler reachability.
func NormalizeAlibaba1688URL(rawURL string) string {
	cleanURL := strings.TrimSpace(rawURL)
	if cleanURL == "" {
		return ""
	}
	if !strings.HasPrefix(strings.ToLower(cleanURL), "http://") && !strings.HasPrefix(strings.ToLower(cleanURL), "https://") {
		cleanURL = "https://" + cleanURL
	}
	parsed, err := url.Parse(cleanURL)
	if err != nil {
		return cleanURL
	}
	parsed.Fragment = ""
	parsed.RawQuery = ""
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return parsed.String()
}

// ExtractAlibaba1688ProductID extracts a 1688 offer ID from common detail URLs.
func ExtractAlibaba1688ProductID(rawURL string) string {
	if matches := alibaba1688OfferIDPattern.FindStringSubmatch(strings.TrimSpace(rawURL)); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

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
	Product     *Alibaba1688ProductSnapshot
	RawSnapshot string
	SourceRunID string
	RequestID   string
	Error       error
}

// Alibaba1688SourceEnvelope maps one 1688 crawler product into the neutral
// source envelope used by catalog, asset, and ListingKit handoff code.
func Alibaba1688SourceEnvelope(input Alibaba1688SourceEnvelopeInput) sourcing.SourceEnvelope {
	product := input.Product
	identity := alibaba1688SourceIdentity(input.Request, product)
	envelope := sourcing.SourceEnvelope{
		Identity:     identity,
		RawReference: alibaba1688RawReference(input.Request, product, input.RawSnapshot),
		Trace: sourcing.SourceTrace{
			SourceRunID: strings.TrimSpace(input.SourceRunID),
			RequestID:   strings.TrimSpace(input.RequestID),
		},
	}
	if product == nil {
		envelope.Warnings = append(envelope.Warnings, sourcing.SourceWarning{Code: "missing_product", Message: "1688 source product is missing"})
		if input.Error != nil {
			envelope.Warnings = append(envelope.Warnings, sourcing.SourceWarning{Code: "source_error", Message: input.Error.Error()})
		}
		return envelope.Normalize()
	}

	envelope.ProductCandidate = alibaba1688ProductCandidate(product)
	envelope.AssetCandidates = alibaba1688AssetCandidates(product)
	envelope.SupplierOrCostFacts = alibaba1688SupplierOrCostFacts(product)
	envelope.Warnings = alibaba1688SourceWarnings(identity, product, envelope, input.Error)
	return envelope.Normalize()
}

func alibaba1688SourceIdentity(input Alibaba1688CrawlRequestInput, product *Alibaba1688ProductSnapshot) sourcing.SourceIdentity {
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
	return sourcing.NormalizeSourceIdentity(sourcing.SourceIdentity{
		SourceType:     sourcing.SourceTypeCrawler,
		SourcePlatform: Alibaba1688SourcePlatform,
		SourceID:       productID,
		SourceURL:      sourceURL,
		Platform:       Alibaba1688SourcePlatform,
		Region:         "cn",
		ProductID:      productID,
		StoreID:        input.StoreID,
	})
}

func alibaba1688RawReference(input Alibaba1688CrawlRequestInput, product *Alibaba1688ProductSnapshot, snapshot string) sourcing.RawSourceReference {
	ref := sourcing.RawSourceReference{
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

func alibaba1688ProductCandidate(product *Alibaba1688ProductSnapshot) sourcing.ProductCandidate {
	attributes := map[string]string{}
	addStringAttribute(attributes, "source_product_id", product.ID)
	addStringAttribute(attributes, "category", product.Category)
	addStringAttribute(attributes, "brand", product.Brand)
	addStringAttribute(attributes, "currency", default1688Currency(product.Currency))
	addStringAttribute(attributes, "unit", product.Unit)
	addStringAttribute(attributes, "shipping_from", product.Shipping.ShippingFrom)
	addStringAttribute(attributes, "processing_time", product.Shipping.ProcessingTime)
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

	return sourcing.ProductCandidate{
		Title:       strings.TrimSpace(product.Title),
		Description: build1688Description(product),
		Brand:       strings.TrimSpace(product.Brand),
		Attributes:  attributes,
		Variants:    alibaba1688VariantCandidates(product.Variants),
	}
}

func alibaba1688VariantCandidates(variants []Alibaba1688VariantSnapshot) []sourcing.ProductVariantCandidate {
	if len(variants) == 0 {
		return nil
	}
	candidates := make([]sourcing.ProductVariantCandidate, 0, len(variants))
	for idx, variant := range variants {
		attributes := convert1688VariantAttributes(variant.Attributes)
		candidate := sourcing.ProductVariantCandidate{
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

func alibaba1688AssetCandidates(product *Alibaba1688ProductSnapshot) []sourcing.AssetCandidate {
	seen := map[string]struct{}{}
	assets := make([]sourcing.AssetCandidate, 0, len(product.Images)+len(product.ProductDetails)+len(product.Variants)+len(product.Videos)+2)
	appendAsset := func(url, role, mediaType string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		assets = append(assets, sourcing.AssetCandidate{SourceID: url, URL: url, MediaType: mediaType, Role: role})
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

func alibaba1688SupplierOrCostFacts(product *Alibaba1688ProductSnapshot) sourcing.SupplierOrCostFacts {
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
	return sourcing.SupplierOrCostFacts{
		SupplierID:   strings.TrimSpace(product.Supplier.ID),
		SupplierName: strings.TrimSpace(product.Supplier.Name),
		Currency:     default1688Currency(product.Currency),
		Cost:         formatOptionalPrice(product.MinPrice),
		Price:        formatOptionalPrice(product.MinPrice),
		Facts:        facts,
	}
}

func alibaba1688SourceWarnings(identity sourcing.SourceIdentity, product *Alibaba1688ProductSnapshot, envelope sourcing.SourceEnvelope, err error) []sourcing.SourceWarning {
	warnings := []sourcing.SourceWarning{}
	if err != nil {
		warnings = append(warnings, sourcing.SourceWarning{Code: "source_error", Message: err.Error()})
	}
	if identity.Validation().MissingSourceID {
		warnings = append(warnings, sourcing.SourceWarning{Code: "missing_source_id", Field: "id", Message: "1688 source product is missing product id"})
	}
	if strings.TrimSpace(product.Title) == "" {
		warnings = append(warnings, sourcing.SourceWarning{Code: "missing_title", Field: "title", Message: "1688 source product is missing title"})
	}
	if len(envelope.AssetCandidates) == 0 {
		warnings = append(warnings, sourcing.SourceWarning{Code: "missing_assets", Field: "images", Message: "1688 source product has no image assets"})
	}
	if product.MinPrice <= 0 {
		warnings = append(warnings, sourcing.SourceWarning{Code: "missing_cost", Field: "min_price", Message: "1688 source product is missing minimum price"})
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

func addStringAttribute(attributes map[string]string, key, value string) {
	value = strings.TrimSpace(value)
	if value != "" {
		attributes[key] = value
	}
}

func trimNonEmptyStrings(values []string) []string {
	trimmed := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			trimmed = append(trimmed, value)
		}
	}
	return trimmed
}

func formatOptionalPrice(price float64) string {
	if price <= 0 {
		return ""
	}
	return strconv.FormatFloat(price, 'f', -1, 64)
}
