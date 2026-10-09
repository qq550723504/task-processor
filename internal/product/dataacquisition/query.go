// Package dataacquisition owns bounded data-query execution, not Product snapshots.
package dataacquisition

import (
	"encoding/json"
	"errors"
	"math"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"task-processor/internal/product/sourcing"
)

var (
	ErrInvalid     = errors.New("invalid data acquisition request")
	ErrForbidden   = errors.New("data acquisition forbidden")
	ErrUnavailable = errors.New("data acquisition dependency unavailable")
	ErrConflict    = errors.New("data acquisition command conflict")
	ErrNotFound    = errors.New("data acquisition not found")
	ErrUnknown     = errors.New("data acquisition outcome unknown")
)

const PriceFen int64 = 5

type Site struct {
	Code   string `json:"code"`
	Domain string `json:"domain"`
	Name   string `json:"name"`
}

var siteList = []Site{
	{"us", "www.amazon.com", "美国"}, {"uk", "www.amazon.co.uk", "英国"}, {"de", "www.amazon.de", "德国"},
	{"fr", "www.amazon.fr", "法国"}, {"it", "www.amazon.it", "意大利"}, {"es", "www.amazon.es", "西班牙"},
	{"ca", "www.amazon.ca", "加拿大"}, {"jp", "www.amazon.co.jp", "日本"}, {"au", "www.amazon.com.au", "澳大利亚"},
	{"mx", "www.amazon.com.mx", "墨西哥"}, {"br", "www.amazon.com.br", "巴西"}, {"in", "www.amazon.in", "印度"},
	{"ae", "www.amazon.ae", "阿联酋"}, {"sa", "www.amazon.sa", "沙特阿拉伯"},
}

func Sites() []Site { return append([]Site(nil), siteList...) }
func ResolveSite(code string) (Site, error) {
	for _, s := range siteList {
		if s.Code == code {
			return s, nil
		}
	}
	return Site{}, ErrInvalid
}

var asinPattern = regexp.MustCompile(`^[A-Z0-9]{10}$`)
var nodePattern = regexp.MustCompile(`^[0-9]{1,20}$`)
var currencyPattern = regexp.MustCompile(`^[A-Z]{3}$`)
var fields = map[string]bool{"title": true, "description": true, "brand": true, "images": true, "price": true, "currency": true, "availability": true, "attributes": true, "variants": true, "rating": true, "reviewCount": true, "sourceUrl": true, "capturedAt": true, "asin": true, "site": true}

func Fields() []string {
	return []string{"asin", "site", "title", "description", "brand", "images", "price", "currency", "availability", "attributes", "variants", "rating", "reviewCount", "sourceUrl", "capturedAt"}
}

type Query struct {
	Site         string   `json:"site"`
	Mode         string   `json:"mode"`
	Keyword      string   `json:"keyword,omitempty"`
	CategoryNode string   `json:"categoryNode,omitempty"`
	ASINs        []string `json:"asins,omitempty"`
	Limit        int      `json:"limit"`
	Fields       []string `json:"fields,omitempty"`
}

func NormalizeASIN(site Site, value string) (string, error) {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "https://") {
		u, err := url.Parse(value)
		if err != nil || u.User != nil || u.Port() != "" || u.Fragment != "" || (u.Hostname() != site.Domain && u.Hostname() != strings.TrimPrefix(site.Domain, "www.")) {
			return "", ErrInvalid
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) != 2 || parts[0] != "dp" {
			return "", ErrInvalid
		}
		value = parts[1]
	}
	value = strings.ToUpper(value)
	if !asinPattern.MatchString(value) {
		return "", ErrInvalid
	}
	return value, nil
}
func NormalizeQuery(q Query) (Query, error) {
	site, err := ResolveSite(q.Site)
	if err != nil || q.Limit < 1 || q.Limit > 200 || len(q.ASINs) > 200 || len(q.Fields) > 32 || !utf8.ValidString(q.Keyword) || len([]rune(q.Keyword)) > 200 || strings.ContainsAny(q.Keyword, "\x00\r\n") {
		return Query{}, ErrInvalid
	}
	q.Keyword = strings.TrimSpace(q.Keyword)
	if q.CategoryNode != "" && !nodePattern.MatchString(q.CategoryNode) {
		return Query{}, ErrInvalid
	}
	switch q.Mode {
	case "asin":
		if len(q.ASINs) == 0 || q.Keyword != "" || q.CategoryNode != "" {
			return Query{}, ErrInvalid
		}
		seen := map[string]bool{}
		normalized := []string{}
		for _, input := range q.ASINs {
			a, e := NormalizeASIN(site, input)
			if e != nil {
				return Query{}, e
			}
			if !seen[a] {
				normalized = append(normalized, a)
				seen[a] = true
			}
		}
		q.ASINs = normalized
	case "keyword":
		if q.Keyword == "" || len(q.ASINs) != 0 {
			return Query{}, ErrInvalid
		}
	case "category":
		if q.CategoryNode == "" || len(q.ASINs) != 0 || q.Keyword != "" {
			return Query{}, ErrInvalid
		}
	default:
		return Query{}, ErrInvalid
	}
	seen := map[string]bool{}
	for _, f := range q.Fields {
		if !fields[f] || seen[f] {
			return Query{}, ErrInvalid
		}
		seen[f] = true
	}
	if len(q.Fields) == 0 {
		q.Fields = Fields()
	}
	return q, nil
}

type Evidence struct {
	Site          string            `json:"site"`
	ASIN          string            `json:"asin"`
	Title         string            `json:"title"`
	Description   string            `json:"description,omitempty"`
	Brand         string            `json:"brand,omitempty"`
	MainImage     string            `json:"mainImage"`
	Images        []string          `json:"images,omitempty"`
	Availability  string            `json:"availability"`
	Currency      string            `json:"currency,omitempty"`
	Price         float64           `json:"price,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	CapturedAt    string            `json:"capturedAt"`
	ParserVersion string            `json:"parserVersion"`
	Missing       []string          `json:"missing,omitempty"`
}

func (e Evidence) Validate() error {
	site, err := ResolveSite(e.Site)
	if err != nil {
		return err
	}
	a, err := NormalizeASIN(site, e.ASIN)
	if err != nil || a != e.ASIN || strings.TrimSpace(e.Title) == "" || len(e.Title) > 2000 || !utf8.ValidString(e.Title) || len(e.Description) > 1<<20 || len(e.Images) > 40 || len(e.Attributes) > 256 || e.ParserVersion != "amazon-v1" {
		return ErrInvalid
	}
	if _, err := time.Parse(time.RFC3339Nano, e.CapturedAt); err != nil {
		return ErrInvalid
	}
	if len(e.Missing) > 32 {
		return ErrInvalid
	}
	for _, field := range e.Missing {
		if !fields[field] {
			return ErrInvalid
		}
	}
	if e.Availability != "available" && e.Availability != "unavailable" {
		return ErrInvalid
	}
	if e.Price < 0 || math.IsNaN(e.Price) || math.IsInf(e.Price, 0) || (e.Currency != "" && !currencyPattern.MatchString(e.Currency)) {
		return ErrInvalid
	}
	if e.Availability == "available" && (!currencyPattern.MatchString(e.Currency) || e.Price <= 0) {
		return ErrInvalid
	}
	for _, value := range append([]string{e.MainImage}, e.Images...) {
		u, err := url.Parse(value)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || u.Fragment != "" || len(value) > 2048 || (u.Hostname() != "m.media-amazon.com" && u.Hostname() != "images-na.ssl-images-amazon.com" && u.Hostname() != "images-eu.ssl-images-amazon.com" && u.Hostname() != "images-fe.ssl-images-amazon.com") {
			return ErrInvalid
		}
	}
	raw, err := json.Marshal(e)
	if err != nil || len(raw) > sourcing.MaxEncodedEnvelopeBytes {
		return ErrInvalid
	}
	return nil
}
func (e Evidence) Envelope(operation string) (sourcing.SourceEnvelope, error) {
	if e.Validate() != nil {
		return sourcing.SourceEnvelope{}, ErrInvalid
	}
	raw, _ := json.Marshal(e)
	site, _ := ResolveSite(e.Site)
	envelope := sourcing.SourceEnvelope{Identity: sourcing.SourceIdentity{SourceType: "public_web", SourcePlatform: "amazon", SourceID: e.Site + ":" + e.ASIN}, RawReference: sourcing.RawSourceReference{ReferenceType: "captured_evidence", ReferenceID: operation, Checksum: sourcing.RawSnapshotChecksum(string(raw))}, ProductCandidate: sourcing.ProductCandidate{Title: e.Title, Description: e.Description, Brand: e.Brand, Attributes: e.Attributes}}
	envelope.Trace.SourceRunID = "amazon_data:" + operation
	for _, field := range e.Missing {
		envelope.Warnings = append(envelope.Warnings, sourcing.SourceWarning{Code: "missing_source_fact", Field: field, Message: "字段未在本次公开页面中取得"})
	}
	envelope.ProductCandidate.Attributes = map[string]string{}
	for k, v := range e.Attributes {
		envelope.ProductCandidate.Attributes[k] = v
	}
	envelope.ProductCandidate.Attributes["sourceUrl"] = "https://" + site.Domain + "/dp/" + e.ASIN
	envelope.ProductCandidate.Attributes["availability"] = e.Availability
	envelope.ProductCandidate.Attributes["capturedAt"] = e.CapturedAt
	if e.Availability == "available" {
		envelope.ProductCandidate.Variants = []sourcing.ProductVariantCandidate{{SourceID: e.ASIN, Title: e.Title, Currency: e.Currency, Price: e.Price}}
	}
	seen := map[string]bool{}
	for _, u := range append([]string{e.MainImage}, e.Images...) {
		if seen[u] {
			continue
		}
		seen[u] = true
		role := "detail"
		if len(envelope.AssetCandidates) == 0 {
			role = "main"
		}
		envelope.AssetCandidates = append(envelope.AssetCandidates, sourcing.AssetCandidate{SourceID: sourcing.RawSnapshotChecksum(u), URL: u, MediaType: "image", Role: role})
	}
	return sourcing.NormalizePublicationEnvelope(envelope)
}
