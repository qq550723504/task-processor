package sherr

// RetryableError 可重试错误接口
type RetryableError interface {
	error
	IsRetryable() bool
}

// retryableError 可重试错误实现
type retryableError struct {
	message    string
	retryable  bool
	wrappedErr error
}

func (e *retryableError) Error() string {
	if e.wrappedErr != nil {
		return e.message + ": " + e.wrappedErr.Error()
	}
	return e.message
}

func (e *retryableError) IsRetryable() bool {
	return e.retryable
}

func (e *retryableError) Unwrap() error {
	return e.wrappedErr
}

// NewRetryableError 创建可重试错误
func NewRetryableError(message string, err error) error {
	if isAuthenticationExpiredError(err) {
		return err
	}
	return &retryableError{message: message, retryable: true, wrappedErr: err}
}

// NewNonRetryableError 创建不可重试错误
func NewNonRetryableError(message string, err error) error {
	return &retryableError{message: message, retryable: false, wrappedErr: err}
}

// IsRetryableError 检查错误是否可重试
func IsRetryableError(err error) bool {
	if isAuthenticationExpired(err) {
		return false
	}
	if isNonRetryableError(err) {
		return false
	}
	if re, ok := err.(RetryableError); ok {
		return re.IsRetryable()
	}
	return true
}
