package client

import "fmt"

// Error 表示 SDS 客户端统一错误。
type Error struct {
	Op         string
	StatusCode int
	Message    string
	Err        error
}

func (e *Error) Error() string {
	switch {
	case e.Err != nil && e.StatusCode > 0:
		return fmt.Sprintf("sds %s failed with status %d: %s: %v", e.Op, e.StatusCode, e.Message, e.Err)
	case e.Err != nil:
		return fmt.Sprintf("sds %s failed: %s: %v", e.Op, e.Message, e.Err)
	case e.StatusCode > 0:
		return fmt.Sprintf("sds %s failed with status %d: %s", e.Op, e.StatusCode, e.Message)
	default:
		return fmt.Sprintf("sds %s failed: %s", e.Op, e.Message)
	}
}

func (e *Error) Unwrap() error {
	return e.Err
}
