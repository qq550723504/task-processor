// Package auth 提供认证功能
package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// ClientCredentialsTokenResponse 客户端凭证令牌响应
type ClientCredentialsTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

// BusinessTokenResponse 业务包装的令牌响应
type BusinessTokenResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		TokenType    string `json:"token_type"`
		Scope        string `json:"scope"`
	} `json:"data"`
}

// fetchAccessToken 获取新的访问令牌
func (c *ClientCredentialsAuthClient) fetchAccessToken() (string, error) {
	tokenURL := c.baseURL + "/admin-api/system/oauth2/token"

	// 构建请求
	req, err := c.buildTokenRequest(tokenURL)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	c.logger.Infof("请求客户端凭证令牌: URL=%s, ClientID=%s, TenantID=%s", tokenURL, c.clientID, c.tenantID)

	// 发送请求
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("获取令牌失败 (状态码: %d): %s", resp.StatusCode, string(body))
	}

	// 解析响应并保存token
	return c.parseAndSaveToken(body)
}

// buildTokenRequest 构建token请求
func (c *ClientCredentialsAuthClient) buildTokenRequest(tokenURL string) (*http.Request, error) {
	// 构建表单数据
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", c.clientID)
	data.Set("client_secret", c.clientSecret)
	data.Set("scope", "user.read")

	req, err := http.NewRequest("POST", tokenURL, bytes.NewBufferString(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("tenant-id", c.tenantID)

	return req, nil
}

// parseAndSaveToken 解析响应并保存token
func (c *ClientCredentialsAuthClient) parseAndSaveToken(body []byte) (string, error) {
	accessToken, expiresIn, err := c.parseTokenResponse(body)
	if err != nil {
		return "", err
	}

	if accessToken == "" {
		return "", fmt.Errorf("响应中没有access_token，响应内容: %s", string(body))
	}

	// 保存token和过期时间
	c.accessToken = accessToken
	c.expiresAt = c.calculateExpiresAt(expiresIn)

	c.logger.Infof("成功获取客户端凭证令牌，过期时间: %s", c.expiresAt.Format("2006-01-02 15:04:05"))

	return c.accessToken, nil
}

// parseTokenResponse 解析token响应
func (c *ClientCredentialsAuthClient) parseTokenResponse(body []byte) (string, int64, error) {
	// 先尝试解析为业务包装响应
	var businessResp BusinessTokenResponse
	if err := json.Unmarshal(body, &businessResp); err == nil && businessResp.Data.AccessToken != "" {
		c.logger.Infof("解析为业务响应格式: code=%d, msg=%s", businessResp.Code, businessResp.Msg)
		if businessResp.Code != 0 {
			return "", 0, fmt.Errorf("获取令牌失败: %s (code: %d)", businessResp.Msg, businessResp.Code)
		}
		c.logger.Info("成功从业务响应中提取access_token")
		return businessResp.Data.AccessToken, businessResp.Data.ExpiresIn, nil
	}

	// 尝试解析为标准 OAuth2 响应
	var tokenResp ClientCredentialsTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("解析响应失败: %w，响应内容: %s", err, string(body))
	}

	c.logger.Info("成功从标准OAuth2响应中提取access_token")
	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}

// calculateExpiresAt 计算过期时间
func (c *ClientCredentialsAuthClient) calculateExpiresAt(expiresIn int64) time.Time {
	if expiresIn > 0 {
		// 提前5分钟过期，避免边界情况
		return time.Now().Add(time.Duration(expiresIn)*time.Second - 5*time.Minute)
	}
	// 默认1个月过期
	return time.Now().Add(30 * 24 * time.Hour)
}
