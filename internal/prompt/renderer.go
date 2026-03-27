package prompt

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

// TemplateRenderer 模板渲染器，使用 text/template 将占位符替换为运行时变量。
type TemplateRenderer interface {
	// Render 渲染模板字符串，vars 为变量映射。
	// vars 为 nil 且模板不含 {{ 时直接返回原字符串（零分配快速路径）。
	// 模板中存在未定义变量时返回 "", error，错误信息包含缺失变量名。
	Render(tmpl string, vars map[string]any) (string, error)
}

// templateRenderer 是 TemplateRenderer 的具体实现。
type templateRenderer struct{}

// NewTemplateRenderer 创建一个新的 TemplateRenderer 实例。
func NewTemplateRenderer() TemplateRenderer {
	return &templateRenderer{}
}

// Render 渲染模板字符串。
func (r *templateRenderer) Render(tmpl string, vars map[string]any) (string, error) {
	// 快速路径：vars 为 nil 且模板不含占位符，零分配直接返回
	if vars == nil && !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}

	// 使用 Option("missingkey=error") 确保未定义变量时返回错误
	t, err := template.New("").Option("missingkey=error").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("解析模板失败: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", fmt.Errorf("渲染模板失败: %w", err)
	}
	return buf.String(), nil
}
