// Package quality checks user-supplied facts without model inference or IO.
package quality

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

const RuleVersion = "product-quality-rules-v1"

var ErrInput = errors.New("invalid product quality input")
var lengthUnit = regexp.MustCompile(`(?i)(?:\d|\s)(?:mm|cm|m|inches|inch|in|ft|feet|meter|metre)\b|毫米|厘米|米|英寸|″|"`)

type Specification struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}
type Input struct {
	Name           string          `json:"name"`
	Material       string          `json:"material"`
	Dimensions     string          `json:"dimensions"`
	Description    string          `json:"description"`
	Specifications []Specification `json:"specifications"`
}
type Finding struct {
	Code       string `json:"code"`
	Field      string `json:"field"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
}
type Report struct {
	RuleVersion string    `json:"ruleVersion"`
	Summary     string    `json:"summary"`
	Findings    []Finding `json:"findings"`
}

func valid(v string, n int) bool {
	return utf8.ValidString(v) && strings.TrimSpace(v) == v && len(v) <= 4*n && utf8.RuneCountInString(v) <= n && strings.IndexFunc(v, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' }) < 0
}
func Check(in Input) (Report, error) {
	r := Report{RuleVersion: RuleVersion, Findings: []Finding{}}
	if !valid(in.Name, 120) || !valid(in.Material, 240) || !valid(in.Dimensions, 240) || !valid(in.Description, 4000) || len(in.Specifications) > 20 {
		return r, ErrInput
	}
	nonempty := in.Name != "" || in.Material != "" || in.Dimensions != "" || in.Description != ""
	for _, s := range in.Specifications {
		if !valid(s.Name, 80) || !valid(s.Value, 240) {
			return r, ErrInput
		}
		nonempty = nonempty || s.Name != "" || s.Value != ""
	}
	if !nonempty {
		return r, ErrInput
	}
	add := func(code, field, message, suggestion string) {
		r.Findings = append(r.Findings, Finding{code, field, message, suggestion})
	}
	for _, f := range []struct{ value, code, field, label string }{{in.Name, "MISSING_NAME", "name", "商品名称"}, {in.Material, "MISSING_MATERIAL", "material", "材质"}, {in.Dimensions, "MISSING_DIMENSIONS", "dimensions", "尺寸"}, {in.Description, "MISSING_DESCRIPTION", "description", "描述"}} {
		if f.value == "" {
			add(f.code, f.field, "尚未提供"+f.label, "请依据真实商品资料补充"+f.label+"，不要猜测。")
		}
	}
	if len(in.Specifications) == 0 {
		add("MISSING_SPECIFICATIONS", "specifications", "尚未提供规格", "补充实际颜色、型号或其他适用规格。")
	}
	if in.Dimensions != "" {
		if !lengthUnit.MatchString(in.Dimensions) {
			add("DIMENSION_UNIT_UNSPECIFIED", "dimensions", "尺寸未明确常用长度单位", "确认尺寸数字对应的单位，例如 cm、mm 或英寸。")
		}
	}
	seen := map[string]string{}
	for _, s := range in.Specifications {
		if s.Name == "" || s.Value == "" {
			add("INCOMPLETE_SPECIFICATION", "specifications", "有规格缺少名称或值", "补全每项规格的名称及实际值。")
			continue
		}
		key := strings.ToLower(s.Name)
		if v, ok := seen[key]; ok {
			if v == s.Value {
				add("DUPLICATE_SPECIFICATION", "specifications", "规格“"+s.Name+"”重复", "删除完全重复的规格项。")
			} else {
				add("CONFLICTING_SPECIFICATION", "specifications", "规格“"+s.Name+"”出现不同值", "核对是否为多个变体；请明确变体关系，或修正错误值。")
			}
		} else {
			seen[key] = s.Value
		}
	}
	if in.Name != "" && in.Name == in.Description {
		add("DUPLICATE_DESCRIPTION", "description", "描述与名称完全相同", "补充与名称不同的真实用途或特征。")
	}
	r.Summary = "资料存在需要核实或补充的项目。"
	if len(r.Findings) == 0 {
		r.Summary = "本次确定性规则未发现问题；仍需人工核实真实性与适用要求。"
	}
	return r, nil
}
