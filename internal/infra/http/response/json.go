// Package response 提供 HTTP 响应工具
package response

import (
	"encoding/json"
	"net/http"
)

// JSON 统一 JSON 响应结构
type JSON struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Success 发送成功响应
func Success(w http.ResponseWriter, message string, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(JSON{
		Success: true,
		Message: message,
		Data:    data,
	})
}

// Error 发送错误响应
func Error(w http.ResponseWriter, statusCode int, errMsg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(JSON{
		Success: false,
		Error:   errMsg,
	})
}

// BadRequest 发送 400 错误
func BadRequest(w http.ResponseWriter, errMsg string) {
	Error(w, http.StatusBadRequest, errMsg)
}

// NotFound 发送 404 错误
func NotFound(w http.ResponseWriter, errMsg string) {
	Error(w, http.StatusNotFound, errMsg)
}

// MethodNotAllowed 发送 405 错误
func MethodNotAllowed(w http.ResponseWriter, errMsg string) {
	Error(w, http.StatusMethodNotAllowed, errMsg)
}

// ServiceUnavailable 发送 503 错误
func ServiceUnavailable(w http.ResponseWriter, errMsg string) {
	Error(w, http.StatusServiceUnavailable, errMsg)
}

// InternalServerError 发送 500 错误
func InternalServerError(w http.ResponseWriter, errMsg string) {
	Error(w, http.StatusInternalServerError, errMsg)
}
