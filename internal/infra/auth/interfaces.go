// Package auth 提供认证功能
package auth

// ClientCredentialsClient 客户端凭证认证接口
type ClientCredentialsClient interface {
	GetAccessToken() (string, error)
	GetTenantID() string
}
