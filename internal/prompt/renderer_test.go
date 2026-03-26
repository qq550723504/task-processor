package prompt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender_NilVarsNoPlaceholder(t *testing.T) {
	r := NewTemplateRenderer()
	got, err := r.Render("hello world", nil)
	require.NoError(t, err)
	assert.Equal(t, "hello world", got)
}

func TestRender_NilVarsWithPlaceholder(t *testing.T) {
	r := NewTemplateRenderer()
	// vars 为 nil 但模板含占位符，应返回 error（map 为 nil，访问 key 报错）
	_, err := r.Render("hello {{.Name}}", nil)
	assert.Error(t, err)
}

func TestRender_WithVars(t *testing.T) {
	r := NewTemplateRenderer()
	cases := []struct {
		name string
		tmpl string
		vars map[string]any
		want string
	}{
		{
			name: "单变量替换",
			tmpl: "标题: {{.Title}}",
			vars: map[string]any{"Title": "iPhone 15"},
			want: "标题: iPhone 15",
		},
		{
			name: "多变量替换",
			tmpl: "{{.Brand}} - {{.Title}}",
			vars: map[string]any{"Brand": "Apple", "Title": "iPhone 15"},
			want: "Apple - iPhone 15",
		},
		{
			name: "同一变量多次使用",
			tmpl: "{{.Name}} is {{.Name}}",
			vars: map[string]any{"Name": "Go"},
			want: "Go is Go",
		},
		{
			name: "空 vars 无占位符",
			tmpl: "no placeholder",
			vars: map[string]any{},
			want: "no placeholder",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Render(tc.tmpl, tc.vars)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestRender_MissingVar_ReturnsError(t *testing.T) {
	r := NewTemplateRenderer()
	cases := []struct {
		name        string
		tmpl        string
		vars        map[string]any
		errContains string
	}{
		{
			name:        "缺失变量",
			tmpl:        "hello {{.Missing}}",
			vars:        map[string]any{},
			errContains: "Missing",
		},
		{
			name:        "部分变量缺失",
			tmpl:        "{{.Title}} by {{.Author}}",
			vars:        map[string]any{"Title": "Go"},
			errContains: "Author",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := r.Render(tc.tmpl, tc.vars)
			assert.Error(t, err)
			assert.Empty(t, got)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestRender_InvalidTemplate(t *testing.T) {
	r := NewTemplateRenderer()
	_, err := r.Render("{{.Unclosed", map[string]any{})
	assert.Error(t, err)
}
