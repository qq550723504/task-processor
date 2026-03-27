package sherr

import "strings"

// FilteredError 业务过滤错误（非真正的错误，只是不符合筛选条件）
type FilteredError struct {
	message string
}

func (e *FilteredError) Error() string     { return e.message }
func (e *FilteredError) IsRetryable() bool { return false }

// NewFilteredError 创建业务过滤错误
func NewFilteredError(message string) error {
	return &FilteredError{message: message}
}

// IsFilteredError 检查是否为业务过滤错误
func IsFilteredError(err error) bool {
	if err == nil {
		return false
	}
	if _, ok := err.(*FilteredError); ok {
		return true
	}
	errMsg := err.Error()
	for _, kw := range []string{"低于筛选规则", "高于筛选规则", "超过筛选规则", "筛选规则最低", "筛选规则最高"} {
		if strings.Contains(errMsg, kw) {
			return true
		}
	}
	return false
}
