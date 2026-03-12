// Package errors 提供错误处理功能
package errors

import (
	"errors"
)

// NewError 创建新的错误
func NewError(message string) error {
	return errors.New(message)
}

// WrapError 包装错误
func WrapError(err error, message string) error {
	if err == nil {
		return nil
	}
	return errors.New(message + ": " + err.Error())
}
