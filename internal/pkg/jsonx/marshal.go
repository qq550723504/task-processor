package jsonx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MarshalWithoutHTMLEscape 序列化JSON但不转义HTML字符
// 避免 & 被转义为 \u0026，< 被转义为 \u003c 等
func MarshalWithoutHTMLEscape(v any) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)

	if err := encoder.Encode(v); err != nil {
		return nil, fmt.Errorf("JSON编码失败: %w", err)
	}

	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}

// MarshalIndentWithoutHTMLEscape 序列化JSON（带缩进）但不转义HTML字符
func MarshalIndentWithoutHTMLEscape(v any, prefix, indent string) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent(prefix, indent)

	if err := encoder.Encode(v); err != nil {
		return nil, fmt.Errorf("JSON编码失败: %w", err)
	}

	result := buf.Bytes()
	if len(result) > 0 && result[len(result)-1] == '\n' {
		result = result[:len(result)-1]
	}

	return result, nil
}

// MarshalPretty 美化输出JSON（带缩进，不转义HTML）
func MarshalPretty(v any) ([]byte, error) {
	return MarshalIndentWithoutHTMLEscape(v, "", "  ")
}

// UnmarshalStrict 严格模式反序列化JSON，包含未知字段时返回错误
func UnmarshalStrict(data []byte, v any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("JSON解码失败: %w", err)
	}

	return nil
}

// ToJSONString 将对象转换为JSON字符串（不转义HTML）
func ToJSONString(v any) (string, error) {
	data, err := MarshalWithoutHTMLEscape(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSONStringPretty 将对象转换为美化的JSON字符串
func ToJSONStringPretty(v any) (string, error) {
	data, err := MarshalPretty(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// IsValidJSON 检查字符串是否为有效的JSON
func IsValidJSON(str string) bool {
	var js json.RawMessage
	return json.Unmarshal([]byte(str), &js) == nil
}

// CompactJSON 压缩JSON（移除空白字符）
func CompactJSON(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, data); err != nil {
		return nil, fmt.Errorf("压缩JSON失败: %w", err)
	}
	return buf.Bytes(), nil
}

// SaveToFile 将 JSON 数据写入 logs/ 目录下的指定文件名
func SaveToFile(filename string, data []byte) error {
	if err := os.MkdirAll("logs", 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %w", err)
	}
	filePath := filepath.Join("logs", filename)
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("写入文件失败: %w", err)
	}
	return nil
}
