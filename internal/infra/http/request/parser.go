// Package request 提供 HTTP 请求解析工具
package request

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// ParseJSON 解析 JSON 请求体
// 参数:
//   - r: HTTP 请求
//   - v: 目标结构体指针
//
// 返回值:
//   - error: 解析错误，成功时返回 nil
func ParseJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("请求体为空")
	}

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields() // 不允许未知字段

	if err := decoder.Decode(v); err != nil {
		return fmt.Errorf("解析 JSON 失败: %w", err)
	}

	return nil
}

// ParseJSONWithValidation 解析 JSON 并执行验证
// 参数:
//   - r: HTTP 请求
//   - v: 目标结构体指针
//   - validateFn: 验证函数
//
// 返回值:
//   - error: 解析或验证错误
func ParseJSONWithValidation(r *http.Request, v interface{}, validateFn func() error) error {
	if err := ParseJSON(r, v); err != nil {
		return err
	}

	if validateFn != nil {
		if err := validateFn(); err != nil {
			return fmt.Errorf("验证失败: %w", err)
		}
	}

	return nil
}
