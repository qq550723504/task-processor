// Package productjson 提供 HTTP API 处理器
package productjson

import (
	"time"
)

// ErrorResponse 错误响应
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// TaskResponse 任务响应
type TaskResponse struct {
	TaskID    string    `json:"task_id"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
