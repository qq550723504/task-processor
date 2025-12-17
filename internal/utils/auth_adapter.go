// Package utils 提供工具方法
package utils

import (
	internalAuth "task-processor/internal/auth"
	commonAuth "task-processor/internal/common/auth"
)

// ClientCredentialsAdapter 客户端凭证适配器
type ClientCredentialsAdapter struct {
	client internalAuth.ClientCredentialsClient
}

// NewClientCredentialsAdapter 创建客户端凭证适配器
func NewClientCredentialsAdapter(client internalAuth.ClientCredentialsClient) *ClientCredentialsAdapter {
	return &ClientCredentialsAdapter{
		client: client,
	}
}

// GetAccessToken 获取访问令牌
func (a *ClientCredentialsAdapter) GetAccessToken() (string, error) {
	return a.client.GetAccessToken()
}

// GetTenantID 获取租户ID
func (a *ClientCredentialsAdapter) GetTenantID() string {
	return a.client.GetTenantID()
}

// ToCommonAuth 转换为common包的认证客户端（临时解决方案）
func (a *ClientCredentialsAdapter) ToCommonAuth() *commonAuth.ClientCredentialsAuthClient {
	// 这是一个临时的适配方案，理想情况下应该重构server包使用接口
	// 由于我们不能修改server包的接口，这里使用反射或其他方式
	// 但为了简单起见，我们先返回nil，让调用方处理
	return nil
}
