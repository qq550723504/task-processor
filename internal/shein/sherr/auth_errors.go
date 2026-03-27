package sherr

import "strings"

// IsAuthenticationExpiredError checks whether the error semantically indicates
// an expired authentication state even if the concrete type varies.
func IsAuthenticationExpiredError(err error) bool {
	return isAuthenticationExpired(err)
}

func isAuthenticationExpired(err error) bool {
	return isAuthenticationExpiredError(err)
}

func isAuthenticationExpiredError(err error) bool {
	for err != nil {
		msg := err.Error()
		if strings.Contains(msg, "20302") && strings.Contains(msg, "子系统登录重定向") {
			return true
		}
		if strings.Contains(msg, "认证已过期") || strings.Contains(msg, "需要重新登录") {
			return true
		}
		if u, ok := err.(interface{ Unwrap() error }); ok {
			err = u.Unwrap()
		} else {
			break
		}
	}
	return false
}
