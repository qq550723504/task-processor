package sourcing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"time"
	"unicode/utf8"
)

const (
	BrowserCaptureVersion        = 1
	BrowserCaptureParserVersion  = "1688-browser-dom/v1"
	BrowserCaptureMaxBytes       = 2 * 1024 * 1024
	browserCaptureMaxStringBytes = 8 * 1024
	browserCaptureMaxCollection  = 256
	browserCaptureMaxAggregate   = 1024
)

// ErrInvalidBrowserCapture deliberately contains no untrusted input.
var ErrInvalidBrowserCapture = errors.New("invalid browser capture")

// BrowserCapture carries a transport receipt and the existing domain evidence.
// PayloadSHA256 is server-computed; the client's contentSHA256 is only a claim.
type BrowserCapture struct {
	Source           AcquisitionSource
	Evidence         AcquisitionEvidence
	CanonicalPayload []byte
	PayloadSHA256    string
}

// ParseBrowserCapture validates and canonicalizes the frozen Browser wire.
func ParseBrowserCapture(body []byte) (BrowserCapture, error) {
	if len(body) == 0 || len(body) > BrowserCaptureMaxBytes || !utf8.Valid(body) {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	aggregate := 0
	if err := checkBrowserWire(decoder, reflect.TypeFor[browserCaptureWire](), &aggregate); err != nil {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	if _, err := decoder.Token(); err != io.EOF {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	var wire browserCaptureWire
	if err := json.Unmarshal(body, &wire); err != nil {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	e := &wire.Evidence
	if wire.CaptureVersion != BrowserCaptureVersion || e.SchemaVersion != 1 || e.ParserVersion != BrowserCaptureParserVersion {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	source, err := Canonical1688Source(e.SourceURL)
	if err != nil || source.OfferID != e.OfferID {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	capturedAt, err := time.Parse(time.RFC3339Nano, e.CapturedAt)
	if err != nil || capturedAt.IsZero() {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	claim, err := hex.DecodeString(e.ContentSHA256)
	if err != nil || len(claim) != sha256.Size {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	e.SourceURL = source.URL
	e.CapturedAt = capturedAt.UTC().Format(time.RFC3339Nano)
	canonical, err := json.Marshal(wire)
	if err != nil || len(canonical) > BrowserCaptureMaxBytes {
		return BrowserCapture{}, ErrInvalidBrowserCapture
	}
	digest := sha256.Sum256(canonical)
	return BrowserCapture{
		Source:           source,
		Evidence:         e.domain(capturedAt.UTC()),
		CanonicalPayload: canonical,
		PayloadSHA256:    hex.EncodeToString(digest[:]),
	}, nil
}

// These private structs are the frozen transport shape, not a second domain
// evidence model. Declaration order fixes canonical bytes. Nothing is omitted.
type browserCaptureWire struct {
	CaptureVersion int                 `json:"captureVersion"`
	Evidence       browserEvidenceWire `json:"evidence"`
}

type browserEvidenceWire struct {
	SchemaVersion int                    `json:"schemaVersion"`
	SourceURL     string                 `json:"sourceURL"`
	OfferID       string                 `json:"offerID"`
	Title         *string                `json:"title"`
	Description   *string                `json:"description"`
	Attributes    []browserAttributeWire `json:"attributes"`
	Variants      []browserVariantWire   `json:"variants"`
	PriceFacts    []browserPriceWire     `json:"priceFacts"`
	Images        []browserImageWire     `json:"images"`
	CapturedAt    string                 `json:"capturedAt"`
	ContentSHA256 string                 `json:"contentSHA256"`
	ParserVersion string                 `json:"parserVersion"`
	Warnings      []browserWarningWire   `json:"warnings"`
	MissingFacts  []browserMissingWire   `json:"missingFacts"`
}

type browserAttributeWire struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type browserVariantWire struct {
	SourceID   *string                `json:"sourceID"`
	SKU        *string                `json:"sku"`
	Title      *string                `json:"title"`
	Attributes []browserAttributeWire `json:"attributes"`
	Price      *browserPriceWire      `json:"price"`
}

type browserPriceWire struct {
	Amount      string  `json:"amount"`
	Currency    *string `json:"currency"`
	MinQuantity *string `json:"minQuantity" browser:"optional"`
}

type browserImageWire struct {
	URL  string `json:"url"`
	Role string `json:"role"`
}

type browserWarningWire struct {
	Code  string `json:"code"`
	Field string `json:"field"`
}

type browserMissingWire struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func (e browserEvidenceWire) domain(capturedAt time.Time) AcquisitionEvidence {
	result := AcquisitionEvidence{
		SchemaVersion: e.SchemaVersion, SourceURL: e.SourceURL, OfferID: e.OfferID,
		Title: e.Title, Description: e.Description,
		Attributes: browserDomainAttributes(e.Attributes),
		Variants:   make([]AcquisitionVariant, len(e.Variants)),
		PriceFacts: make([]AcquisitionPrice, len(e.PriceFacts)),
		Images:     make([]AcquisitionImage, len(e.Images)),
		CapturedAt: capturedAt, ContentSHA256: e.ContentSHA256, ParserVersion: e.ParserVersion,
		Warnings:     make([]SourceWarning, len(e.Warnings)),
		MissingFacts: make([]MissingFact, len(e.MissingFacts)),
	}
	for i, v := range e.Variants {
		result.Variants[i] = AcquisitionVariant{
			SourceID: v.SourceID, SKU: v.SKU, Title: v.Title,
			Attributes: browserDomainAttributes(v.Attributes),
		}
		if v.Price != nil {
			price := v.Price.domain()
			result.Variants[i].Price = &price
		}
	}
	for i, p := range e.PriceFacts {
		result.PriceFacts[i] = p.domain()
	}
	for i, v := range e.Images {
		result.Images[i] = AcquisitionImage{URL: v.URL, Role: v.Role}
	}
	for i, v := range e.Warnings {
		result.Warnings[i] = SourceWarning{Code: v.Code, Field: v.Field}
	}
	for i, v := range e.MissingFacts {
		result.MissingFacts[i] = MissingFact{Field: v.Field, Reason: v.Reason}
	}
	return result
}

func browserDomainAttributes(wire []browserAttributeWire) []AcquisitionAttribute {
	result := make([]AcquisitionAttribute, len(wire))
	for i, v := range wire {
		result[i] = AcquisitionAttribute{Name: v.Name, Value: v.Value}
	}
	return result
}

func (p browserPriceWire) domain() AcquisitionPrice {
	return AcquisitionPrice{Amount: p.Amount, Currency: p.Currency, MinQuantity: p.MinQuantity}
}

// encoding/json owns syntax and typed decoding. This narrow token pass adds
// exact-case/duplicate/required-field checks and budgets that Unmarshal lacks.
// Recursion follows only the finite private wire types, never arbitrary JSON.
func checkBrowserWire(decoder *json.Decoder, shape reflect.Type, aggregate *int) error {
	token, err := decoder.Token()
	if err != nil {
		return ErrInvalidBrowserCapture
	}
	if shape.Kind() == reflect.Pointer {
		if token == nil {
			return nil
		}
		shape = shape.Elem()
	}
	switch shape.Kind() {
	case reflect.Struct:
		if token != json.Delim('{') {
			return ErrInvalidBrowserCapture
		}
		fields := make(map[string]reflect.StructField, shape.NumField())
		for i := 0; i < shape.NumField(); i++ {
			field := shape.Field(i)
			fields[field.Tag.Get("json")] = field
		}
		seen := make(map[string]bool, len(fields))
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return ErrInvalidBrowserCapture
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return ErrInvalidBrowserCapture
			}
			field, ok := fields[key]
			if !ok {
				return ErrInvalidBrowserCapture
			}
			seen[key] = true
			if err := checkBrowserWire(decoder, field.Type, aggregate); err != nil {
				return err
			}
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim('}') {
			return ErrInvalidBrowserCapture
		}
		for key, field := range fields {
			if !seen[key] && field.Tag.Get("browser") != "optional" {
				return ErrInvalidBrowserCapture
			}
		}
	case reflect.Slice:
		if token != json.Delim('[') {
			return ErrInvalidBrowserCapture
		}
		count := 0
		for decoder.More() {
			count++
			*aggregate += 1
			if count > browserCaptureMaxCollection || *aggregate > browserCaptureMaxAggregate {
				return ErrInvalidBrowserCapture
			}
			if err := checkBrowserWire(decoder, shape.Elem(), aggregate); err != nil {
				return err
			}
		}
		if end, err := decoder.Token(); err != nil || end != json.Delim(']') {
			return ErrInvalidBrowserCapture
		}
	case reflect.String:
		value, ok := token.(string)
		if !ok || len(value) > browserCaptureMaxStringBytes {
			return ErrInvalidBrowserCapture
		}
	case reflect.Int:
		if _, ok := token.(json.Number); !ok {
			return ErrInvalidBrowserCapture
		}
	default:
		return ErrInvalidBrowserCapture
	}
	return nil
}
