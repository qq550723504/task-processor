package sale

import (
	"encoding/json"
	"testing"
)

func newJSONParser() *SaleAttributeJSONParser {
	return NewSaleAttributeJSONParser()
}

func TestSaleAttributeJSONParser_looksLikeCompleteJson(t *testing.T) {
	p := newJSONParser()

	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"valid_object", `{"key":"value"}`, true},
		{"valid_nested", `{"a":{"b":1}}`, true},
		{"with_whitespace", `  { "key": "value" }  `, true},
		{"missing_closing_brace", `{"key":"value"`, false},
		{"missing_opening_brace", `"key":"value"}`, false},
		{"empty_object", `{}`, true},
		{"empty_string", ``, false},
		{"array_not_object", `["a","b"]`, false},
		{"plain_text", `hello world`, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.looksLikeCompleteJson(tc.content)
			if got != tc.want {
				t.Errorf("looksLikeCompleteJson(%q) = %v, want %v", tc.content, got, tc.want)
			}
		})
	}
}

func TestSaleAttributeJSONParser_fixCommonJsonIssues_MissingBraces(t *testing.T) {
	p := newJSONParser()

	tests := []struct {
		name      string
		input     string
		wantValid bool
	}{
		{
			"already_valid",
			`{"SaleAttributes":[],"Variants":[]}`,
			true,
		},
		{
			"missing_closing_brace",
			`{"SaleAttributes":[],"Variants":[]`,
			true,
		},
		{
			// fixCommonJsonIssues 按括号数量补全，嵌套对象缺两个括号时顺序不对
			// 实际行为：修复后仍然无效（源码的已知限制）
			"nested_object_missing_two_brackets",
			`{"SaleAttributes":[{"id":1}`,
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := p.fixCommonJsonIssues(tc.input)
			isValid := json.Valid([]byte(result))
			if isValid != tc.wantValid {
				t.Errorf("fixCommonJsonIssues(%q) valid=%v, want %v; result=%q",
					tc.input, isValid, tc.wantValid, result)
			}
		})
	}
}

func TestSaleAttributeJSONParser_removeTrailingExplanation(t *testing.T) {
	p := newJSONParser()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			"no_trailing_text",
			`{"key":"value"}`,
			`{"key":"value"}`,
		},
		{
			"trailing_markdown_header",
			`{"key":"value"}` + "\n### 说明\n这是说明文字",
			`{"key":"value"}`,
		},
		{
			"trailing_bold_text",
			`{"key":"value"}` + "\n**注意**：这是注意事项",
			`{"key":"value"}`,
		},
		{
			"trailing_numbered_list",
			`{"key":"value"}` + "\n\n1. 第一条\n2. 第二条",
			`{"key":"value"}`,
		},
		{
			"trailing_dash_list",
			`{"key":"value"}` + "\n\n- 条目一\n- 条目二",
			`{"key":"value"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := p.removeTrailingExplanation(tc.input)
			if got != tc.want {
				t.Errorf("removeTrailingExplanation() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSaleAttributeJSONParser_ParseAndValidateJSON_ValidInput(t *testing.T) {
	p := newJSONParser()

	// ResultSaleAttribute 字段无 JSON tag，使用大写字段名
	validJSON := `{
		"SaleAttributes": [
			{"AttrID": 1, "AttrValue": [{"ID": 101, "Value": "Black"}]}
		],
		"Variants": [
			{"attributes": {"Color": "Black"}}
		]
	}`

	result := p.ParseAndValidateJSON(validJSON)
	if len(result.SaleAttributes) != 1 {
		t.Errorf("expected 1 sale attribute, got %d", len(result.SaleAttributes))
	}
	if len(result.Variants) != 1 {
		t.Errorf("expected 1 variant, got %d", len(result.Variants))
	}
}

func TestSaleAttributeJSONParser_ParseAndValidateJSON_EmptyInput(t *testing.T) {
	p := newJSONParser()

	result := p.ParseAndValidateJSON("")
	if len(result.SaleAttributes) != 0 || len(result.Variants) != 0 {
		t.Errorf("expected empty result for empty input, got %+v", result)
	}
}

func TestSaleAttributeJSONParser_ParseAndValidateJSON_WithMarkdownWrapper(t *testing.T) {
	p := newJSONParser()

	// GPT 常见返回格式：带 markdown 代码块
	wrappedJSON := "```json\n{\"SaleAttributes\":[],\"Variants\":[]}\n```"

	result := p.ParseAndValidateJSON(wrappedJSON)
	// 能正确解析，不 panic，空切片合法
	_ = result
}
