// Package a1688 reads bounded anonymous public product evidence, never accounts.
package a1688

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/net/html"
	sigjson "sigs.k8s.io/json"
	"task-processor/internal/integration/httpimage"
	"task-processor/internal/product/sourcing"
)

type Client struct{ http *http.Client }

const maxBody = 2 << 20

var ErrUnavailable = errors.New("public acquisition unavailable")
var ErrUnsupported = errors.New("public source structure unsupported")
var contextAssignment = regexp.MustCompile(`^\s*(?:window\.)?context\s*=\s*`)

func New() *Client {
	client := httpimage.NewPublicImageHTTPClient()
	if transport, ok := client.Transport.(*http.Transport); ok {
		transport.DisableCompression = true
	}
	return &Client{http: client}
}

// NewWithTransport is explicit trusted composition injection for isolated HTTP
// fixtures. Runtime configuration never accepts a transport or fixture URL.
func NewWithTransport(transport http.RoundTripper) *Client {
	client := New()
	if transport != nil {
		client.http.Transport = transport
	}
	return client
}

func (c *Client) Acquire(ctx context.Context, source sourcing.AcquisitionSource) (sourcing.AcquisitionEvidence, error) {
	canonical, err := sourcing.Canonical1688Source(source.URL)
	if err != nil || canonical != source {
		return sourcing.AcquisitionEvidence{}, sourcing.ErrInvalidAcquisition
	}
	if c == nil || c.http == nil {
		return sourcing.AcquisitionEvidence{}, ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	client := *c.http
	client.Jar = nil
	client.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		redirect, err := sourcing.Canonical1688Source(request.URL.String())
		if err != nil || redirect != source || request.URL.Scheme != "https" || len(via) > 3 {
			return ErrUnavailable
		}
		request.URL.RawQuery, request.URL.Fragment = "", ""
		request.Header.Del("Cookie")
		request.Header.Del("Authorization")
		return nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return sourcing.AcquisitionEvidence{}, ErrUnavailable
	}
	request.Header.Set("Accept", "text/html, application/json")
	request.Header.Set("Accept-Encoding", "gzip")
	response, err := client.Do(request)
	if err != nil {
		if ctx.Err() != nil {
			return sourcing.AcquisitionEvidence{}, ctx.Err()
		}
		return sourcing.AcquisitionEvidence{}, ErrUnavailable
	}
	defer response.Body.Close()
	media, parameters, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (media != "text/html" && media != "application/json") || parameters["charset"] != "" && !strings.EqualFold(parameters["charset"], "utf-8") || response.StatusCode != http.StatusOK || response.ContentLength > maxBody {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	raw, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil || len(raw) > maxBody {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	switch strings.ToLower(response.Header.Get("Content-Encoding")) {
	case "", "identity":
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(raw))
		if err != nil {
			return sourcing.AcquisitionEvidence{}, ErrUnsupported
		}
		raw, err = io.ReadAll(io.LimitReader(reader, maxBody+1))
		closeErr := reader.Close()
		if err != nil || closeErr != nil || len(raw) > maxBody {
			return sourcing.AcquisitionEvidence{}, ErrUnsupported
		}
	default:
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	if ctx.Err() != nil {
		return sourcing.AcquisitionEvidence{}, ctx.Err()
	}
	return parsePublic(source, raw, media, time.Now().UTC())
}

type exactText string

func (v *exactText) UnmarshalJSON(raw []byte) error {
	if bytes.Equal(raw, []byte("null")) {
		return ErrUnsupported
	}
	if len(raw) > 0 && raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return ErrUnsupported
		}
		*v = exactText(s)
		return nil
	}
	if len(raw) == 0 || raw[0] != '-' && (raw[0] < '0' || raw[0] > '9') {
		return ErrUnsupported
	}
	*v = exactText(raw)
	return nil
}

type rawPrice struct {
	Price       *exactText `json:"price"`
	Currency    *string    `json:"currency"`
	BeginAmount *exactText `json:"beginAmount"`
}

type contextFields struct {
	Title        *string  `json:"title"`
	OfferImgList []string `json:"offerImgList"`
	DataJSON     struct {
		TempModel struct {
			OfferID exactText `json:"offerId"`
		} `json:"tempModel"`
		SKUModel struct {
			Props []struct {
				Name string `json:"prop"`
			} `json:"skuProps"`
			Info map[string]struct {
				ID       *exactText `json:"skuId"`
				Price    *exactText `json:"price"`
				Currency *string    `json:"currency"`
			} `json:"skuInfoMap"`
		} `json:"skuModel"`
	} `json:"dataJson"`
	FinalPriceModel struct {
		Trade struct {
			Ranges []rawPrice `json:"offerPriceRanges"`
		} `json:"tradeWithoutPromotion"`
	} `json:"finalPriceModel"`
}

type contextDocument struct {
	Result struct {
		Data map[string]struct {
			Fields contextFields `json:"fields"`
		} `json:"data"`
		Global struct {
			Data struct {
				Model struct {
					Detail struct {
						Attributes []sourcing.AcquisitionAttribute `json:"featureAttributes"`
					} `json:"offerDetail"`
				} `json:"model"`
			} `json:"globalData"`
		} `json:"global"`
	} `json:"result"`
}

func parsePublic(source sourcing.AcquisitionSource, raw []byte, media string, captured time.Time) (sourcing.AcquisitionEvidence, error) {
	if !utf8.Valid(raw) || len(raw) == 0 {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	data, description := raw, (*string)(nil)
	if media == "text/html" {
		var err error
		data, description, err = publicJSON(raw)
		if err != nil {
			return sourcing.AcquisitionEvidence{}, err
		}
	}
	if !boundedJSONDepth(data) {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	var document contextDocument
	violations, err := sigjson.UnmarshalStrict(data, &document, sigjson.DisallowDuplicateFields)
	if err != nil || len(violations) != 0 {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	root := document.Result.Data["Root"].Fields.DataJSON
	if string(root.TempModel.OfferID) != source.OfferID {
		return sourcing.AcquisitionEvidence{}, ErrUnsupported
	}
	digest := sha256.Sum256(raw)
	e := sourcing.AcquisitionEvidence{SchemaVersion: 1, SourceURL: source.URL, OfferID: source.OfferID, Title: document.Result.Data["productTitle"].Fields.Title, Description: description, Attributes: document.Result.Global.Data.Model.Detail.Attributes, CapturedAt: captured, ContentSHA256: hex.EncodeToString(digest[:]), ParserVersion: "1688-context-v1"}
	for _, imageURL := range document.Result.Data["gallery"].Fields.OfferImgList {
		e.Images = append(e.Images, sourcing.AcquisitionImage{URL: imageURL, Role: "source"})
	}
	keys := make([]string, 0, len(root.SKUModel.Info))
	for key := range root.SKUModel.Info {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		sku := root.SKUModel.Info[key]
		variant := sourcing.AcquisitionVariant{SourceID: textPointer(sku.ID)}
		if sku.Price != nil {
			variant.Price = &sourcing.AcquisitionPrice{Amount: string(*sku.Price), Currency: sku.Currency}
		}
		values := strings.Split(key, "&gt;")
		if len(values) == len(root.SKUModel.Props) {
			for i, value := range values {
				variant.Attributes = append(variant.Attributes, sourcing.AcquisitionAttribute{Name: root.SKUModel.Props[i].Name, Value: value})
			}
		} else {
			e.MissingFacts = append(e.MissingFacts, sourcing.MissingFact{Field: "variants.attributes", Reason: "source did not identify variant attribute names"})
		}
		e.Variants = append(e.Variants, variant)
	}
	keys = keys[:0]
	for key := range document.Result.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, price := range document.Result.Data[key].Fields.FinalPriceModel.Trade.Ranges {
			if price.Price == nil {
				e.MissingFacts = append(e.MissingFacts, sourcing.MissingFact{Field: "price", Reason: "source price range has no amount"})
				continue
			}
			e.PriceFacts = append(e.PriceFacts, sourcing.AcquisitionPrice{Amount: string(*price.Price), Currency: price.Currency, MinQuantity: textPointer(price.BeginAmount)})
		}
	}
	return e, nil
}

func textPointer(v *exactText) *string {
	if v == nil {
		return nil
	}
	text := string(*v)
	return &text
}

func publicJSON(raw []byte) ([]byte, *string, error) {
	tokenizer := html.NewTokenizer(bytes.NewReader(raw))
	tokenizer.SetMaxBuf(maxBody)
	var result []byte
	var description *string
	inScript := false
	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			if tokenizer.Err() != io.EOF || len(result) == 0 {
				return nil, nil, ErrUnsupported
			}
			return result, description, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			inScript = token.Data == "script"
			if token.Data == "meta" {
				var name, content string
				for _, attr := range token.Attr {
					if attr.Key == "name" {
						name = attr.Val
					}
					if attr.Key == "content" {
						content = attr.Val
					}
				}
				if strings.EqualFold(name, "description") {
					value := content
					description = &value
				}
			}
		case html.EndTagToken:
			inScript = false
		case html.TextToken:
			if !inScript {
				continue
			}
			text := tokenizer.Text()
			match := contextAssignment.FindIndex(text)
			if match == nil {
				continue
			}
			if result != nil {
				return nil, nil, ErrUnsupported
			}
			candidate := strings.TrimSpace(string(text[match[1]:]))
			candidate = strings.TrimSpace(strings.TrimSuffix(candidate, ";"))
			result = []byte(candidate)
		}
	}
}

func boundedJSONDepth(data []byte) bool {
	depth, nodes := 0, 0
	inString, escaped := false, false
	for _, b := range data {
		if inString {
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
			nodes++
		case '}', ']':
			depth--
		}
		if depth > 32 || nodes > 4096 || depth < 0 {
			return false
		}
	}
	return depth == 0 && !inString
}
