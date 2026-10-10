// Package amazon extracts anonymous fixed-site evidence. Legacy extractor
// selectors, multilingual stock signals and price formats are selectively
// extracted here without its model, logger, config or task dependencies.
package amazon

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
	"task-processor/internal/product/dataacquisition"
)

var ErrChallenge = dataacquisition.ErrSourceChallenge
var ErrUnsupported = dataacquisition.ErrSourceUnsupported

const maxHTMLBytes = 4 << 20

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}
func walk(n *html.Node, visit func(*html.Node) bool) *html.Node {
	if visit(n) {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := walk(c, visit); found != nil {
			return found
		}
	}
	return nil
}
func textOf(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	walk(n, func(v *html.Node) bool {
		if v.Type == html.TextNode {
			b.WriteString(v.Data)
			b.WriteByte(' ')
		}
		return false
	})
	return strings.Join(strings.Fields(b.String()), " ")
}
func byID(n *html.Node, id string) *html.Node {
	return walk(n, func(v *html.Node) bool { return attr(v, "id") == id })
}
func hasClass(n *html.Node, class string) bool {
	for _, v := range strings.Fields(attr(n, "class")) {
		if v == class {
			return true
		}
	}
	return false
}
func document(raw string) (*html.Node, error) {
	if len(raw) > maxHTMLBytes {
		return nil, dataacquisition.ErrInvalid
	}
	n, err := html.Parse(strings.NewReader(raw))
	if err != nil {
		return nil, ErrUnsupported
	}
	if walk(n, func(v *html.Node) bool {
		return strings.Contains(attr(v, "action"), "validateCaptcha") || attr(v, "id") == "captchacharacters" || attr(v, "id") == "captchaImage"
	}) != nil {
		return nil, ErrChallenge
	}
	if title := walk(n, func(v *html.Node) bool { return v.Type == html.ElementNode && v.Data == "title" }); title != nil && strings.Contains(strings.ToLower(textOf(title)), "robot check") {
		return nil, ErrChallenge
	}
	return n, nil
}
func ParseDiscovery(raw string, limit int) ([]string, error) {
	if limit < 1 || limit > 200 {
		return nil, dataacquisition.ErrInvalid
	}
	doc, err := document(raw)
	if err != nil {
		return nil, err
	}
	ids := []string{}
	seen := map[string]bool{}
	supported := walk(doc, func(n *html.Node) bool {
		return hasClass(n, "s-main-slot") || attr(n, "data-component-type") == "s-search-result"
	}) != nil
	if !supported {
		return nil, ErrUnsupported
	}
	site, _ := dataacquisition.ResolveSite("us")
	walk(doc, func(n *html.Node) bool {
		if len(ids) >= limit || attr(n, "data-component-type") != "s-search-result" {
			return false
		}
		advert := false
		walk(n, func(v *html.Node) bool {
			t := ""
			if v.Type == html.ElementNode && v.Data == "span" {
				t = strings.ToLower(strings.TrimSpace(textOf(v)))
			}
			if attr(v, "data-component-type") == "sp-sponsored-result" || hasClass(v, "puis-sponsored-label-text") || (v.Type == html.ElementNode && v.Data == "span" && (t == "sponsored" || t == "gesponsert" || t == "sponsorisé" || t == "patrocinado" || t == "sponsorizzato" || t == "スポンサー")) {
				advert = true
			}
			return false
		})
		a, e := dataacquisition.NormalizeASIN(site, attr(n, "data-asin"))
		if e == nil && !advert && !seen[a] {
			ids = append(ids, a)
			seen[a] = true
		}
		return false
	})
	return ids, nil
}
func stockState(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	for _, k := range []string{"currently unavailable", "out of stock", "not available", "discontinued", "sold out", "no disponible", "agotado", "sin stock", "在庫切れ", "取り扱い終了", "現在お取り扱いできません", "nicht auf lager", "ausverkauft", "derzeit nicht verfügbar", "rupture de stock", "épuisé", "indisponible", "non disponibile", "esaurito", "indisponível", "esgotado", "غير متوفر", "نفذت الكمية"} {
		if strings.Contains(text, k) {
			return "unavailable"
		}
	}
	for _, k := range []string{"in stock", "available", "usually ships", "pre-order", "en stock", "disponible", "在庫あり", "予約注文", "入荷予定", "auf lager", "verfügbar", "disponibile", "em estoque", "disponível", "متوفر"} {
		if strings.Contains(text, k) {
			return "available"
		}
	}
	return "unknown"
}

var priceNumber = regexp.MustCompile(`[0-9][0-9.,\x{00a0} ]*`)
var dotGroupedPrice = regexp.MustCompile(`^[0-9]{1,3}(\.[0-9]{3})+$`)

func parsePrice(text, site string) (float64, string) {
	code := map[string]string{"us": "USD", "uk": "GBP", "de": "EUR", "fr": "EUR", "it": "EUR", "es": "EUR", "ca": "CAD", "jp": "JPY", "au": "AUD", "mx": "MXN", "br": "BRL", "in": "INR", "ae": "AED", "sa": "SAR"}[site]
	explicit := false
	for _, currency := range []string{"USD", "GBP", "EUR", "CAD", "JPY", "AUD", "MXN", "BRL", "INR", "AED", "SAR"} {
		if regexp.MustCompile(`\b` + currency + `\b`).MatchString(strings.ToUpper(text)) {
			code = currency
			explicit = true
			break
		}
	}
	// Currency is accepted only when the actual price text carries a recognized
	// code/symbol for this fixed marketplace. Missing symbols are not defaulted.
	symbol := map[string]string{"USD": "$", "GBP": "£", "EUR": "€", "CAD": "$", "JPY": "¥", "AUD": "$", "MXN": "$", "BRL": "R$", "INR": "₹", "AED": "د.إ", "SAR": "ريال"}[code]
	if !explicit && !strings.Contains(text, symbol) && !(code == "JPY" && strings.Contains(text, "￥")) {
		return 0, ""
	}
	value := strings.TrimSpace(priceNumber.FindString(text))
	value = strings.ReplaceAll(strings.ReplaceAll(value, " ", ""), "\u00a0", "")
	comma, dot := strings.LastIndex(value, ","), strings.LastIndex(value, ".")
	european := comma > dot && comma >= 0 && len(value)-comma-1 == 2
	if comma < 0 {
		// These marketplaces use dots for integer grouping even when the
		// displayed price omits its decimal comma. Keep ordinary dot decimals
		// and other marketplaces' interpretation unchanged.
		switch site {
		case "de", "it", "es", "br":
			european = dotGroupedPrice.MatchString(value)
		}
	}
	if european {
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
	} else {
		value = strings.ReplaceAll(value, ",", "")
	}
	price, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, ""
	}
	return price, code
}
func ParseProduct(raw, site, asin string, at time.Time) (dataacquisition.Evidence, error) {
	doc, err := document(raw)
	if err != nil {
		return dataacquisition.Evidence{}, err
	}
	market, err := dataacquisition.ResolveSite(site)
	if err != nil {
		return dataacquisition.Evidence{}, err
	}
	canonical, err := dataacquisition.NormalizeASIN(market, asin)
	if err != nil || canonical != asin {
		return dataacquisition.Evidence{}, dataacquisition.ErrInvalid
	}
	n := byID(doc, "ASIN")
	if n == nil || attr(n, "value") != asin {
		return dataacquisition.Evidence{}, ErrUnsupported
	}
	e := dataacquisition.Evidence{Site: site, ASIN: asin, Title: textOf(byID(doc, "productTitle")), Description: textOf(byID(doc, "productDescription")), CapturedAt: at.UTC().Format(time.RFC3339Nano), ParserVersion: "amazon-v1", Attributes: map[string]string{}}
	image := byID(doc, "landingImage")
	if image == nil {
		image = byID(doc, "imgBlkFront")
	}
	if image != nil {
		e.MainImage = attr(image, "data-old-hires")
		if e.MainImage == "" {
			e.MainImage = attr(image, "src")
		}
	}
	availability := byID(doc, "availability")
	if availability == nil {
		availability = byID(doc, "availability-feature")
	}
	e.Availability = stockState(textOf(availability))
	var priceText string
	for _, id := range []string{"corePriceDisplay_desktop_feature_div", "corePrice_feature_div", "apex_desktop", "priceblock_ourprice", "priceblock_dealprice"} {
		root := byID(doc, id)
		if root == nil {
			continue
		}
		p := walk(root, func(v *html.Node) bool {
			return hasClass(v, "a-offscreen") && v.Parent != nil && hasClass(v.Parent, "a-price") && !hasClass(v.Parent, "a-text-price")
		})
		if p != nil {
			priceText = textOf(p)
		} else if strings.HasPrefix(id, "priceblock_") {
			priceText = textOf(root)
		}
		if priceText != "" {
			break
		}
	}
	if priceText == "" {
		p := walk(doc, func(v *html.Node) bool {
			return hasClass(v, "a-offscreen") && v.Parent != nil && hasClass(v.Parent, "a-price") && !hasClass(v.Parent, "a-text-price")
		})
		priceText = textOf(p)
	}
	e.Price, e.Currency = parsePrice(priceText, site)
	for _, id := range []string{"feature-bullets", "productDetails_techSpec_section_1", "productDetails_detailBullets_sections1"} {
		if value := textOf(byID(doc, id)); value != "" && len(value) < 16000 {
			e.Attributes[id] = value
		}
	}
	brand := textOf(byID(doc, "bylineInfo"))
	if len(brand) <= 200 {
		e.Brand = brand
	}
	e.Missing = []string{"variants", "rating", "reviewCount"}
	if e.Description == "" {
		e.Missing = append(e.Missing, "description")
	}
	if e.Brand == "" {
		e.Missing = append(e.Missing, "brand")
	}
	if e.Price <= 0 {
		e.Missing = append(e.Missing, "price", "currency")
	}
	if err := e.Validate(); err != nil {
		return dataacquisition.Evidence{}, err
	}
	return e, nil
}
