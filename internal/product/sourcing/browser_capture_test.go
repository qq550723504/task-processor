package sourcing

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// The fixture is copied, not reimplemented, from PR #400 at
// 7d689dc19461b2b6b60ffc972020e6c06283fca0. In particular, SKU is unknown.
func browserFixture(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("testdata/browser-capture-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	return body
}

const browserGoldenCanonical = `{"captureVersion":1,"evidence":{"schemaVersion":1,"sourceURL":"https://detail.1688.com/offer/981645030344.html","offerID":"981645030344","title":"棉质收纳袋","description":"可重复使用的棉质收纳袋。","attributes":[{"name":"材质","value":"棉"}],"variants":[{"sourceID":"99999999999999999999","sku":null,"title":null,"attributes":[{"name":"颜色","value":"蓝色"}],"price":{"amount":"1.23000001","currency":null,"minQuantity":null}}],"priceFacts":[{"amount":"12.34000001","currency":null,"minQuantity":"2"}],"images":[{"url":"https://cbu01.alicdn.com/img/ibank/fixture.jpg","role":"source"}],"capturedAt":"2026-09-12T00:00:00Z","contentSHA256":"69ea755f45b92880d8041c706dbd3fa77cff5b8143e021987faf0f1bdacdb6db","parserVersion":"1688-browser-dom/v1","warnings":[{"code":"MISSING_FACT","field":"priceFacts[0].currency"},{"code":"MISSING_FACT","field":"variants[0].sku"},{"code":"MISSING_FACT","field":"variants[0].price.currency"}],"missingFacts":[{"field":"priceFacts[0].currency","reason":"not_observed"},{"field":"variants[0].sku","reason":"not_observed"},{"field":"variants[0].price.currency","reason":"not_observed"}]}}`

func TestBrowserCaptureCanonicalGolden(t *testing.T) {
	capture, err := ParseBrowserCapture(browserFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	if string(capture.CanonicalPayload) != browserGoldenCanonical {
		t.Fatalf("canonical bytes mismatch:\n%s", capture.CanonicalPayload)
	}
	// Independent oracle: .NET SHA256 over the frozen UTF-8 canonical string.
	if capture.PayloadSHA256 != "503afdca81714f7b48638b81f1c6d720e3b29a1f2d2ba4da65c88a849def34c7" {
		t.Fatalf("server intent SHA mismatch: %s", capture.PayloadSHA256)
	}
	if capture.PayloadSHA256 == "69ea755f45b92880d8041c706dbd3fa77cff5b8143e021987faf0f1bdacdb6db" {
		t.Fatal("client projection claim was trusted as server intent hash")
	}
	if len(capture.PayloadSHA256) != 64 {
		t.Fatal("missing server SHA256")
	}
}

func TestBrowserCaptureCanonicalEquivalence(t *testing.T) {
	body := browserFixture(t)
	original, err := ParseBrowserCapture(body)
	if err != nil {
		t.Fatal(err)
	}
	var shuffled map[string]any
	if err := json.Unmarshal(body, &shuffled); err != nil {
		t.Fatal(err)
	}
	evidence := shuffled["evidence"].(map[string]any)
	evidence["sourceURL"] = "http://detail.1688.com/offer/981645030344.html?track=ignored#ignored"
	evidence["capturedAt"] = "2026-09-12T08:00:00+08:00"
	evidence["variants"].([]any)[0].(map[string]any)["price"].(map[string]any)["minQuantity"] = nil
	reordered, _ := json.MarshalIndent(shuffled, "", "  ")
	equivalent, err := ParseBrowserCapture(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original.CanonicalPayload, equivalent.CanonicalPayload) || original.PayloadSHA256 != equivalent.PayloadSHA256 {
		t.Fatal("wire field order, equivalent source/time or optional null changed intent")
	}
}

func TestBrowserCaptureRejectsInvalidWire(t *testing.T) {
	fixture := string(browserFixture(t))
	for name, mutate := range map[string]func(string) string{
		"unknown root":     func(s string) string { return strings.Replace(s, "{", `{"token":"secret",`, 1) },
		"duplicate root":   func(s string) string { return strings.Replace(s, "{", `{"captureVersion":1,`, 1) },
		"duplicate nested": func(s string) string { return strings.Replace(s, `"sku": null`, `"sku":null,"sku":null`, 1) },
		"case alias":       func(s string) string { return strings.Replace(s, `"captureVersion"`, `"CaptureVersion"`, 1) },
		"unknown nested":   func(s string) string { return strings.Replace(s, `"sku": null`, `"sku":null,"cookie":"secret"`, 1) },
		"null attributes": func(s string) string {
			return strings.Replace(s, `"attributes": [`, `"attributes": null,"discarded": [`, 1)
		},
		"wrong capture version": func(s string) string { return strings.Replace(s, `"captureVersion": 1`, `"captureVersion":2`, 1) },
		"wrong schema version":  func(s string) string { return strings.Replace(s, `"schemaVersion": 1`, `"schemaVersion":2`, 1) },
		"wrong parser":          func(s string) string { return strings.Replace(s, "1688-browser-dom/v1", "1688-public/v1", 1) },
		"unaligned offer": func(s string) string {
			return strings.Replace(s, `"offerID": "981645030344"`, `"offerID":"981645030345"`, 1)
		},
		"foreign source": func(s string) string {
			return strings.Replace(s, "https://detail.1688.com/offer/", "https://evil.test/offer/", 1)
		},
		"bad date": func(s string) string { return strings.Replace(s, "2026-09-12T00:00:00.000Z", "not-a-date", 1) },
		"bad claim": func(s string) string {
			return strings.Replace(s, "69ea755f45b92880d8041c706dbd3fa77cff5b8143e021987faf0f1bdacdb6db", "not-a-hash", 1)
		},
		"numeric decimal": func(s string) string { return strings.Replace(s, `"1.23000001"`, `1.23000001`, 1) },
		"trailing value":  func(s string) string { return s + "{}" },
		"string budget":   func(s string) string { return strings.Replace(s, "not_observed", strings.Repeat("x", 8193), 1) },
		"body budget":     func(s string) string { return strings.Repeat(" ", 2*1024*1024) + s },
		"invalid utf8":    func(s string) string { return strings.Replace(s, "not_observed", string([]byte{0xff}), 1) },
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseBrowserCapture([]byte(mutate(fixture))); err == nil {
				t.Fatal("invalid wire was accepted")
			}
		})
	}
}

func TestBrowserCaptureCollectionsRequiredAndBounded(t *testing.T) {
	for _, field := range []string{"attributes", "variants", "priceFacts", "images", "warnings", "missingFacts"} {
		for _, mode := range []string{"missing", "null", "oversized"} {
			t.Run(field+"/"+mode, func(t *testing.T) {
				var payload map[string]any
				_ = json.Unmarshal(browserFixture(t), &payload)
				evidence := payload["evidence"].(map[string]any)
				switch mode {
				case "missing":
					delete(evidence, field)
				case "null":
					evidence[field] = nil
				case "oversized":
					item := evidence[field].([]any)[0]
					items := make([]any, 257)
					for i := range items {
						items[i] = item
					}
					evidence[field] = items
				}
				body, _ := json.Marshal(payload)
				if _, err := ParseBrowserCapture(body); err == nil {
					t.Fatal("missing/null/oversized collection accepted")
				}
			})
		}
	}
}

func TestBrowserCapturePreservesIntentAndHTMLEscaping(t *testing.T) {
	var payload map[string]any
	_ = json.Unmarshal(browserFixture(t), &payload)
	evidence := payload["evidence"].(map[string]any)
	evidence["title"] = "  <tag>& original  "
	body, _ := json.Marshal(payload)
	capture, err := ParseBrowserCapture(body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(capture.CanonicalPayload, []byte(`"title":"  \u003ctag\u003e\u0026 original  "`)) {
		t.Fatal("original string or Go JSON HTML escaping not preserved")
	}
	if !bytes.Contains(capture.CanonicalPayload, []byte(`"amount":"1.23000001"`)) || !bytes.Contains(capture.CanonicalPayload, []byte(`"sku":null`)) {
		t.Fatal("decimal or unknown SKU changed")
	}
}

func TestBrowserCaptureAggregateBudgetAndNestedShape(t *testing.T) {
	for _, mode := range []string{"aggregate", "nested null array", "missing nullable", "null array item"} {
		t.Run(mode, func(t *testing.T) {
			var payload map[string]any
			_ = json.Unmarshal(browserFixture(t), &payload)
			evidence := payload["evidence"].(map[string]any)
			variant := evidence["variants"].([]any)[0].(map[string]any)
			switch mode {
			case "aggregate":
				for _, field := range []string{"attributes", "priceFacts", "images", "warnings", "missingFacts"} {
					item := evidence[field].([]any)[0]
					items := make([]any, 256)
					for i := range items {
						items[i] = item
					}
					evidence[field] = items
				}
			case "nested null array":
				variant["attributes"] = nil
			case "missing nullable":
				delete(variant, "sku")
			case "null array item":
				evidence["images"] = []any{nil}
			}
			body, _ := json.Marshal(payload)
			if _, err := ParseBrowserCapture(body); err == nil {
				t.Fatal("invalid shape/budget accepted")
			}
		})
	}
}

func TestBrowserCaptureOrderAndLexemesChangeDigest(t *testing.T) {
	var payload map[string]any
	_ = json.Unmarshal(browserFixture(t), &payload)
	evidence := payload["evidence"].(map[string]any)
	attributes := evidence["attributes"].([]any)
	evidence["attributes"] = append(attributes, map[string]any{"name": "second", "value": "fact"})
	body, _ := json.Marshal(payload)
	original, err := ParseBrowserCapture(body)
	if err != nil {
		t.Fatal(err)
	}
	attributes = evidence["attributes"].([]any)
	attributes[0], attributes[1] = attributes[1], attributes[0]
	body, _ = json.Marshal(payload)
	reordered, err := ParseBrowserCapture(body)
	if err != nil {
		t.Fatal(err)
	}
	if original.PayloadSHA256 == reordered.PayloadSHA256 {
		t.Fatal("array order lost from intent")
	}
	evidence["priceFacts"].([]any)[0].(map[string]any)["amount"] = "12.340000010"
	body, _ = json.Marshal(payload)
	lexeme, err := ParseBrowserCapture(body)
	if err != nil {
		t.Fatal(err)
	}
	if reordered.PayloadSHA256 == lexeme.PayloadSHA256 {
		t.Fatal("decimal lexical precision lost from intent")
	}
}

func TestBrowserCaptureRejectsUnpairedSurrogates(t *testing.T) {
	var payload map[string]any
	_ = json.Unmarshal(browserFixture(t), &payload)
	payload["evidence"].(map[string]any)["title"] = "UNICODE_SENTINEL"
	body, _ := json.Marshal(payload)
	for _, escaped := range []string{`"\ud800"`, `"\udbff"`, `"\udc00"`, `"\udfff"`, `"\ud800\u0061"`, `"\ud800\ud800"`, `"\ud800x"`} {
		t.Run(escaped, func(t *testing.T) {
			input := bytes.Replace(body, []byte(`"UNICODE_SENTINEL"`), []byte(escaped), 1)
			if _, err := ParseBrowserCapture(input); err == nil {
				t.Fatal("unpaired surrogate was silently replaced and admitted")
			}
		})
	}
}

func TestBrowserCapturePreservesValidUnicode(t *testing.T) {
	var payload map[string]any
	_ = json.Unmarshal(browserFixture(t), &payload)
	payload["evidence"].(map[string]any)["title"] = "UNICODE_SENTINEL"
	body, _ := json.Marshal(payload)
	var pair BrowserCapture
	for _, tc := range []struct{ wire, want string }{
		{`"\ud83d\ude00"`, "\U0001f600"},
		{"\"\U0001f600\"", "\U0001f600"},
		{`"\ufffd"`, "\ufffd"},
		{`"\\ud800"`, `\ud800`},
	} {
		t.Run(tc.wire, func(t *testing.T) {
			input := bytes.Replace(body, []byte(`"UNICODE_SENTINEL"`), []byte(tc.wire), 1)
			capture, err := ParseBrowserCapture(input)
			if err != nil {
				t.Fatal(err)
			}
			if capture.Evidence.Title == nil || *capture.Evidence.Title != tc.want {
				t.Fatal("valid Unicode fact changed")
			}
			if tc.want == "\U0001f600" {
				if pair.PayloadSHA256 != "" && pair.PayloadSHA256 != capture.PayloadSHA256 {
					t.Fatal("raw/paired-escape Unicode intents diverged")
				}
				pair = capture
			}
		})
	}
}
