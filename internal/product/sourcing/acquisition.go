package sourcing

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const AcquisitionContractVersion = "src2b-acquisition-v1"

var ErrInvalidAcquisition = errors.New("invalid acquisition evidence")

type AcquisitionSource struct {
	OfferID string
	URL     string
}

type AcquisitionAttribute struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type AcquisitionPrice struct {
	Amount      string  `json:"amount"`
	Currency    *string `json:"currency"`
	MinQuantity *string `json:"minQuantity,omitempty"`
}

type AcquisitionVariant struct {
	SourceID   *string                `json:"sourceID"`
	SKU        *string                `json:"sku"`
	Title      *string                `json:"title"`
	Attributes []AcquisitionAttribute `json:"attributes"`
	Price      *AcquisitionPrice      `json:"price"`
}

type AcquisitionImage struct {
	URL  string `json:"url"`
	Role string `json:"role"`
}

type AcquisitionEvidence struct {
	SchemaVersion int                    `json:"schemaVersion"`
	SourceURL     string                 `json:"sourceURL"`
	OfferID       string                 `json:"offerID"`
	Title         *string                `json:"title"`
	Description   *string                `json:"description"`
	Attributes    []AcquisitionAttribute `json:"attributes"`
	Variants      []AcquisitionVariant   `json:"variants"`
	PriceFacts    []AcquisitionPrice     `json:"priceFacts"`
	Images        []AcquisitionImage     `json:"images"`
	CapturedAt    time.Time              `json:"capturedAt"`
	ContentSHA256 string                 `json:"contentSHA256"`
	ParserVersion string                 `json:"parserVersion"`
	Warnings      []SourceWarning        `json:"warnings"`
	MissingFacts  []MissingFact          `json:"missingFacts"`
}

var acquisitionOfferID = regexp.MustCompile(`^[1-9][0-9]{0,19}$`)
var acquisitionOfferPath = regexp.MustCompile(`^/offer/([1-9][0-9]{0,19})\.html$`)
var acquisitionDigest = regexp.MustCompile(`^[0-9a-f]{64}$`)
var acquisitionDecimal = regexp.MustCompile(`^(0|[1-9][0-9]{0,15})(\.[0-9]{1,8})?$`)
var acquisitionCurrency = regexp.MustCompile(`^[A-Z]{3}$`)

func Canonical1688Source(input string) (AcquisitionSource, error) {
	if len(input) > 2048 || !utf8.ValidString(input) {
		return AcquisitionSource{}, ErrInvalidAcquisition
	}
	input = strings.TrimSpace(input)
	id := input
	if !acquisitionOfferID.MatchString(id) {
		u, err := url.Parse(input)
		if err != nil || u.User != nil || u.Opaque != "" || u.RawPath != "" || (u.Scheme != "https" && u.Scheme != "http") || !strings.EqualFold(u.Hostname(), "detail.1688.com") {
			return AcquisitionSource{}, ErrInvalidAcquisition
		}
		if port := u.Port(); strings.HasSuffix(u.Host, ":") || port != "" && !(u.Scheme == "https" && port == "443") && !(u.Scheme == "http" && port == "80") {
			return AcquisitionSource{}, ErrInvalidAcquisition
		}
		if _, err := url.QueryUnescape(u.RawQuery); err != nil {
			return AcquisitionSource{}, ErrInvalidAcquisition
		}
		parts := acquisitionOfferPath.FindStringSubmatch(u.Path)
		if len(parts) != 2 {
			return AcquisitionSource{}, ErrInvalidAcquisition
		}
		id = parts[1]
	}
	return AcquisitionSource{OfferID: id, URL: "https://detail.1688.com/offer/" + id + ".html"}, nil
}

// MapAcquisitionEvidence is the sole admission from untrusted acquisition facts.
// Channel describes acquisition, never the source's product identity or authority.
func MapAcquisitionEvidence(source AcquisitionSource, e AcquisitionEvidence, channel, operationID string) (SourceEnvelope, error) {
	canonical, err := Canonical1688Source(source.URL)
	if err != nil || canonical != source || e.SchemaVersion != 1 || e.OfferID != source.OfferID || e.SourceURL != source.URL || (channel != "public_http" && channel != "browser_capture") || operationID == "" || len(operationID) > 128 || !acquisitionDigest.MatchString(e.ContentSHA256) || e.CapturedAt.IsZero() || e.ParserVersion == "" || len(e.ParserVersion) > 128 {
		return SourceEnvelope{}, ErrInvalidAcquisition
	}
	if err := validateAcquisitionEvidenceSize(e); err != nil {
		return SourceEnvelope{}, err
	}
	if e.Title == nil || strings.TrimSpace(*e.Title) == "" {
		return SourceEnvelope{}, ErrInvalidAcquisition
	}
	attributes, err := acquisitionAttributes(e.Attributes)
	if err != nil {
		return SourceEnvelope{}, err
	}
	envelope := SourceEnvelope{
		Identity:         SourceIdentity{SourceType: SourceTypeCrawler, SourcePlatform: "1688", SourceID: source.OfferID, SourceURL: source.URL},
		RawReference:     RawSourceReference{ReferenceType: "acquisition_evidence_v1", ReferenceID: source.OfferID, URL: source.URL, Checksum: "sha256:" + e.ContentSHA256, CapturedAt: e.CapturedAt.UTC(), Metadata: map[string]string{"channel": channel, "parser_version": e.ParserVersion, "contract_version": AcquisitionContractVersion}},
		ProductCandidate: ProductCandidate{Title: strings.TrimSpace(*e.Title), Attributes: attributes},
		Trace:            SourceTrace{SourceRunID: "acquisition:" + operationID},
	}
	missing := func(field, reason string) {
		envelope.MissingFacts = append(envelope.MissingFacts, MissingFact{Field: field, Reason: reason})
		envelope.Warnings = append(envelope.Warnings, SourceWarning{Code: "missing_source_fact", Field: field, Message: reason})
	}
	if e.Description != nil && strings.TrimSpace(*e.Description) != "" {
		envelope.ProductCandidate.Description = *e.Description
	} else {
		missing("description", "source did not supply description text")
	}
	if len(e.Attributes) == 0 {
		missing("attributes", "source did not supply product attributes")
	}
	if len(e.Variants) == 0 {
		missing("variants", "source did not supply variants")
	}
	seenIDs, seenSKUs := map[string]bool{}, map[string]bool{}
	hasPrice := len(e.PriceFacts) > 0
	for i, variant := range e.Variants {
		attrs, err := acquisitionAttributes(variant.Attributes)
		if err != nil {
			return SourceEnvelope{}, err
		}
		candidate := ProductVariantCandidate{SourceID: acquisitionValue(variant.SourceID), SKU: acquisitionValue(variant.SKU), Title: acquisitionValue(variant.Title), Attributes: attrs}
		if candidate.SourceID != "" && seenIDs[candidate.SourceID] || candidate.SKU != "" && seenSKUs[candidate.SKU] {
			return SourceEnvelope{}, ErrInvalidAcquisition
		}
		if candidate.SourceID != "" {
			seenIDs[candidate.SourceID] = true
		} else {
			missing(fmt.Sprintf("variants.%d.sourceID", i), "source variant identifier absent")
		}
		if candidate.SKU != "" {
			seenSKUs[candidate.SKU] = true
		} else {
			missing(fmt.Sprintf("variants.%d.sku", i), "source SKU absent")
		}
		if variant.Price != nil {
			value, err := validateAcquisitionPrice(*variant.Price)
			if err != nil {
				return SourceEnvelope{}, err
			}
			hasPrice = true
			candidate.Attributes["source.price.amount"] = variant.Price.Amount
			priceFact, _ := json.Marshal(variant.Price)
			candidate.Attributes["source.price.fact"] = string(priceFact)
			currency := acquisitionValue(variant.Price.Currency)
			original, originalOK := new(big.Rat).SetString(variant.Price.Amount)
			projected, projectedOK := new(big.Rat).SetString(strconv.FormatFloat(value, 'f', -1, 64))
			if !originalOK || !projectedOK || original.Cmp(projected) != 0 {
				field := fmt.Sprintf("variants.%d.price", i)
				missing(field, "exact source decimal cannot be projected as a Catalog price")
				envelope.Warnings = append(envelope.Warnings, SourceWarning{Code: "source_price_precision", Field: field, Message: "original decimal retained without a rounded Catalog price"})
			} else if currency != "" && value > 0 {
				candidate.Price, candidate.Currency = value, currency
			} else {
				missing(fmt.Sprintf("variants.%d.price", i), "price retained as source fact; missing currency or zero value is not projected as a Catalog price")
			}
		} else {
			missing(fmt.Sprintf("variants.%d.price", i), "source variant price absent")
		}
		envelope.ProductCandidate.Variants = append(envelope.ProductCandidate.Variants, candidate)
	}
	if !hasPrice {
		missing("price", "source did not supply price facts")
	}
	if len(e.PriceFacts) > 0 {
		envelope.SupplierOrCostFacts.Facts = map[string]string{}
	}
	for i, price := range e.PriceFacts {
		if _, err := validateAcquisitionPrice(price); err != nil {
			return SourceEnvelope{}, err
		}
		raw, _ := json.Marshal(price)
		envelope.SupplierOrCostFacts.Facts[fmt.Sprintf("price_fact.%d", i)] = string(raw)
		if price.Currency == nil {
			missing(fmt.Sprintf("priceFacts.%d.currency", i), "source did not identify price currency")
		}
	}
	if len(e.Images) == 0 {
		missing("images", "source did not supply image candidates")
	}
	seenImages := map[string]bool{}
	for _, image := range e.Images {
		if !validAcquisitionImageURL(image.URL) {
			return SourceEnvelope{}, ErrInvalidAcquisition
		}
		if seenImages[image.URL] {
			continue
		}
		seenImages[image.URL] = true
		envelope.AssetCandidates = append(envelope.AssetCandidates, AssetCandidate{URL: image.URL, MediaType: "image", Role: image.Role})
	}
	for _, fact := range e.MissingFacts {
		missing(fact.Field, fact.Reason)
	}
	for _, warning := range e.Warnings {
		envelope.Warnings = append(envelope.Warnings, SourceWarning{Code: warning.Code, Field: warning.Field, Message: "source reported " + warning.Code})
	}
	if err := validateSourceEnvelopePreflight(envelope); err != nil {
		return SourceEnvelope{}, err
	}
	return NormalizePublicationEnvelope(envelope)
}

func acquisitionValue(value *string) string {
	return strings.TrimSpace(acquisitionRawValue(value))
}

func acquisitionRawValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func acquisitionAttributes(values []AcquisitionAttribute) (map[string]string, error) {
	result := make(map[string]string, len(values))
	for _, value := range values {
		name := strings.TrimSpace(value.Name)
		if name == "" || strings.HasPrefix(name, "source.") {
			return nil, ErrInvalidAcquisition
		}
		if _, exists := result[name]; exists {
			return nil, ErrInvalidAcquisition
		}
		result[name] = value.Value
	}
	return result, nil
}

func validateAcquisitionPrice(price AcquisitionPrice) (float64, error) {
	if !acquisitionDecimal.MatchString(price.Amount) || price.Currency != nil && !acquisitionCurrency.MatchString(*price.Currency) || price.MinQuantity != nil && !acquisitionOfferID.MatchString(*price.MinQuantity) {
		return 0, ErrInvalidAcquisition
	}
	value, err := strconv.ParseFloat(price.Amount, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, ErrInvalidAcquisition
	}
	return value, nil
}

func validAcquisitionImageURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Opaque != "" || u.Fragment != "" || strings.EqualFold(u.Hostname(), "localhost") {
		return false
	}
	if ip := net.ParseIP(u.Hostname()); ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
		return false
	}
	return true
}

func validateAcquisitionEvidenceSize(e AcquisitionEvidence) error {
	// Walk before JSON allocation. The shared budget bounds each string/collection.
	b := sourceEnvelopeBudget{}
	add := func(values ...string) bool {
		for _, value := range values {
			if !utf8.ValidString(value) || strings.ContainsRune(value, 0) {
				return false
			}
		}
		return b.addStrings(values...)
	}
	attrs := func(values []AcquisitionAttribute) bool {
		if !b.addItems(len(values)) {
			return false
		}
		for _, attr := range values {
			if !add(attr.Name, attr.Value) {
				return false
			}
		}
		return true
	}
	if !add(e.SourceURL, e.OfferID, acquisitionRawValue(e.Title), acquisitionRawValue(e.Description), e.ContentSHA256, e.ParserVersion) || !attrs(e.Attributes) || !b.addItems(len(e.Variants)) || !b.addItems(len(e.PriceFacts)) || !b.addItems(len(e.Images)) || !b.addItems(len(e.Warnings)) || !b.addItems(len(e.MissingFacts)) {
		return ErrSourcePublicationTooLarge
	}
	for _, v := range e.Variants {
		if !add(acquisitionRawValue(v.SourceID), acquisitionRawValue(v.SKU), acquisitionRawValue(v.Title)) || !attrs(v.Attributes) {
			return ErrSourcePublicationTooLarge
		}
		if v.Price != nil && !add(v.Price.Amount, acquisitionRawValue(v.Price.Currency), acquisitionRawValue(v.Price.MinQuantity)) {
			return ErrSourcePublicationTooLarge
		}
	}
	for _, p := range e.PriceFacts {
		if !add(p.Amount, acquisitionRawValue(p.Currency), acquisitionRawValue(p.MinQuantity)) {
			return ErrSourcePublicationTooLarge
		}
	}
	for _, image := range e.Images {
		if !add(image.URL, image.Role) {
			return ErrSourcePublicationTooLarge
		}
	}
	for _, w := range e.Warnings {
		if !add(w.Code, w.Field, w.Message) {
			return ErrSourcePublicationTooLarge
		}
	}
	for _, m := range e.MissingFacts {
		if !add(m.Field, m.Reason) {
			return ErrSourcePublicationTooLarge
		}
	}
	raw, err := json.Marshal(e)
	if err != nil || len(raw) > MaxEncodedEnvelopeBytes {
		return ErrSourcePublicationTooLarge
	}
	return nil
}
