package sourcing

import (
	"fmt"
	"strconv"
	"strings"

	"task-processor/internal/model"
)

const (
	AmazonSourcePlatform = "amazon"

	amazonSourceReferenceType = "amazon_product"
	amazonImageRolePrimary    = "primary"
	amazonImageRoleGallery    = "gallery"
)

// AmazonSourceEnvelopeInput is the product-sourcing view of one Amazon crawler
// result. It keeps crawler execution details out of product sourcing while still
// preserving source request context for identity and traceability.
type AmazonSourceEnvelopeInput struct {
	Request     SourceRequest
	Product     *model.Product
	RawSnapshot string
	SourceRunID string
	RequestID   string
}

// AmazonSourceEnvelope maps one Amazon crawler product into the neutral product
// sourcing envelope. The mapper intentionally produces platform-neutral product
// and asset candidates only; target marketplace publish payloads belong in
// marketplace packages.
func AmazonSourceEnvelope(input AmazonSourceEnvelopeInput) SourceEnvelope {
	req := NormalizeSourceRequest(input.Request)
	product := input.Product

	identity := amazonSourceIdentity(req, product)
	envelope := SourceEnvelope{
		Identity:     identity,
		RawReference: amazonRawReference(product, input.RawSnapshot),
		Trace: SourceTrace{
			SourceRunID: strings.TrimSpace(input.SourceRunID),
			RequestID:   strings.TrimSpace(input.RequestID),
		},
	}
	if product == nil {
		envelope.Warnings = append(envelope.Warnings, SourceWarning{
			Code:    "missing_product",
			Message: "Amazon source product is missing",
		})
		return envelope.Normalize()
	}

	envelope.ProductCandidate = amazonProductCandidate(product)
	envelope.AssetCandidates = amazonAssetCandidates(product)
	envelope.SupplierOrCostFacts = amazonSupplierOrCostFacts(product)
	envelope.Warnings = amazonSourceWarnings(identity, product, envelope)
	return envelope.Normalize()
}

func amazonSourceIdentity(req SourceRequest, product *model.Product) SourceIdentity {
	id := SourceIdentity{
		SourceType:     SourceTypeCrawler,
		SourcePlatform: AmazonSourcePlatform,
		Region:         req.Region,
		Platform:       AmazonSourcePlatform,
		StoreID:        req.StoreID,
	}
	if product != nil {
		id.SourceID = strings.TrimSpace(product.Asin)
		id.SourceURL = strings.TrimSpace(product.URL)
		id.ProductID = strings.TrimSpace(product.Asin)
	}
	if id.SourceID == "" {
		id.SourceID = req.ProductID
		id.ProductID = req.ProductID
	}
	return NormalizeSourceIdentity(id)
}

func amazonRawReference(product *model.Product, snapshot string) RawSourceReference {
	ref := RawSourceReference{
		ReferenceType: amazonSourceReferenceType,
		SnapshotID:    strings.TrimSpace(snapshot),
	}
	if product == nil {
		return ref
	}
	ref.ReferenceID = strings.TrimSpace(product.Asin)
	ref.URL = strings.TrimSpace(product.URL)
	return ref
}

func amazonProductCandidate(product *model.Product) ProductCandidate {
	attributes := map[string]string{}
	addStringAttribute(attributes, "asin", product.Asin)
	addStringAttribute(attributes, "parent_asin", product.ParentAsin)
	addStringAttribute(attributes, "availability", product.Availability)
	addStringAttribute(attributes, "bs_category", product.BsCategory)
	addStringAttribute(attributes, "root_bs_category", product.RootBsCategory)
	addStringAttribute(attributes, "product_dimensions", product.ProductDimensions)
	addStringAttribute(attributes, "item_weight", product.ItemWeight)
	addStringAttribute(attributes, "model_number", product.ModelNumber)
	addStringAttribute(attributes, "department", product.Department)
	addStringAttribute(attributes, "manufacturer", product.Manufacturer)
	addStringAttribute(attributes, "country_of_origin", product.CountryOfOrigin)
	addStringAttribute(attributes, "ships_from", product.ShipsFrom)
	addStringAttribute(attributes, "domain", product.Domain)
	if len(product.Categories) > 0 {
		addStringAttribute(attributes, "categories", strings.Join(trimNonEmptyStrings(product.Categories), ">"))
	}
	if len(product.Features) > 0 {
		addStringAttribute(attributes, "features", strings.Join(trimNonEmptyStrings(product.Features), "\n"))
	}

	return ProductCandidate{
		Title:       strings.TrimSpace(product.Title),
		Description: amazonProductDescription(product),
		Brand:       strings.TrimSpace(product.Brand),
		Attributes:  attributes,
		Variants:    amazonVariantCandidates(product.Variations),
	}
}

func amazonProductDescription(product *model.Product) string {
	if description := strings.TrimSpace(product.Description); description != "" {
		return description
	}
	parts := make([]string, 0, len(product.ProductDescription))
	for _, description := range product.ProductDescription {
		if text := strings.TrimSpace(description.Text); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n")
}

func amazonVariantCandidates(variations []model.Variation) []ProductVariantCandidate {
	if len(variations) == 0 {
		return nil
	}
	candidates := make([]ProductVariantCandidate, 0, len(variations))
	for _, variation := range variations {
		candidate := ProductVariantCandidate{
			SourceID:   strings.TrimSpace(variation.Asin),
			Title:      strings.TrimSpace(variation.Name),
			Attributes: stringifyAttributes(variation.Attributes),
		}
		if candidate.SourceID == "" && candidate.Title == "" && len(candidate.Attributes) == 0 {
			continue
		}
		candidates = append(candidates, candidate)
	}
	return candidates
}

func amazonAssetCandidates(product *model.Product) []AssetCandidate {
	seen := map[string]struct{}{}
	assets := make([]AssetCandidate, 0, len(product.Images)+1)
	appendAsset := func(url, role string) {
		url = strings.TrimSpace(url)
		if url == "" {
			return
		}
		if _, ok := seen[url]; ok {
			return
		}
		seen[url] = struct{}{}
		assets = append(assets, AssetCandidate{
			SourceID:  url,
			URL:       url,
			MediaType: "image",
			Role:      role,
		})
	}
	appendAsset(product.ImageURL, amazonImageRolePrimary)
	for _, image := range product.Images {
		appendAsset(image, amazonImageRoleGallery)
	}
	return assets
}

func amazonSupplierOrCostFacts(product *model.Product) SupplierOrCostFacts {
	facts := map[string]string{}
	addStringAttribute(facts, "buybox_seller", product.BuyboxSeller)
	if product.NumberOfSellers > 0 {
		facts["number_of_sellers"] = strconv.Itoa(product.NumberOfSellers)
	}
	return SupplierOrCostFacts{
		SupplierID:   strings.TrimSpace(product.SellerID),
		SupplierName: strings.TrimSpace(product.SellerName),
		Currency:     strings.TrimSpace(product.Currency),
		Cost:         formatOptionalPrice(product.FinalPrice),
		Price:        formatOptionalPrice(product.FinalPrice),
		Facts:        facts,
	}
}

func amazonSourceWarnings(identity SourceIdentity, product *model.Product, envelope SourceEnvelope) []SourceWarning {
	warnings := []SourceWarning{}
	validation := identity.Validation()
	if validation.MissingSourceID {
		warnings = append(warnings, SourceWarning{Code: "missing_source_id", Field: "asin", Message: "Amazon source product is missing ASIN"})
	}
	if strings.TrimSpace(product.Title) == "" {
		warnings = append(warnings, SourceWarning{Code: "missing_title", Field: "title", Message: "Amazon source product is missing title"})
	}
	if len(envelope.AssetCandidates) == 0 {
		warnings = append(warnings, SourceWarning{Code: "missing_assets", Field: "images", Message: "Amazon source product has no image assets"})
	}
	return warnings
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

func stringifyAttributes(attributes map[string]any) map[string]string {
	if len(attributes) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, value := range attributes {
		key = strings.TrimSpace(key)
		if key == "" || value == nil {
			continue
		}
		out[key] = strings.TrimSpace(fmt.Sprint(value))
	}
	return out
}

func formatOptionalPrice(price float64) string {
	if price <= 0 {
		return ""
	}
	return strconv.FormatFloat(price, 'f', -1, 64)
}
