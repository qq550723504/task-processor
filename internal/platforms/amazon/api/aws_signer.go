// Package api 提供Amazon SP-API的AWS签名v4实现
package api

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AWSSigner AWS签名v4实现
type AWSSigner struct {
	accessKeyID     string
	secretAccessKey string
	region          string
	service         string
}

// NewAWSSigner 创建AWS签名器
func NewAWSSigner(accessKeyID, secretAccessKey, region string) *AWSSigner {
	return &AWSSigner{
		accessKeyID:     accessKeyID,
		secretAccessKey: secretAccessKey,
		region:          region,
		service:         "execute-api",
	}
}

// SignRequest 对HTTP请求进行AWS签名v4签名
func (s *AWSSigner) SignRequest(req *http.Request, payload []byte) error {
	if s.accessKeyID == "" || s.secretAccessKey == "" {
		// 如果没有AWS凭证，跳过签名（使用LWA令牌）
		return nil
	}

	now := time.Now().UTC()

	// 设置必需的头部
	req.Header.Set("X-Amz-Date", now.Format("20060102T150405Z"))
	req.Header.Set("Host", req.URL.Host)

	// 计算payload哈希
	payloadHash := s.calculatePayloadHash(payload)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	// 创建签名
	signature := s.createSignature(req, payloadHash, now)

	// 设置Authorization头部
	authHeader := fmt.Sprintf("AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.accessKeyID,
		s.createCredentialScope(now),
		s.getSignedHeaders(req),
		signature)

	req.Header.Set("Authorization", authHeader)

	return nil
}

// calculatePayloadHash 计算请求体的SHA256哈希
func (s *AWSSigner) calculatePayloadHash(payload []byte) string {
	if payload == nil {
		payload = []byte{}
	}
	hash := sha256.Sum256(payload)
	return fmt.Sprintf("%x", hash)
}

// createSignature 创建签名
func (s *AWSSigner) createSignature(req *http.Request, payloadHash string, timestamp time.Time) string {
	// 步骤1: 创建规范请求
	canonicalRequest := s.createCanonicalRequest(req, payloadHash)

	// 步骤2: 创建待签名字符串
	stringToSign := s.createStringToSign(canonicalRequest, timestamp)

	// 步骤3: 计算签名
	signingKey := s.createSigningKey(timestamp)
	signature := s.calculateSignature(signingKey, stringToSign)

	return signature
}

// createCanonicalRequest 创建规范请求
func (s *AWSSigner) createCanonicalRequest(req *http.Request, payloadHash string) string {
	// HTTP方法
	method := req.Method

	// 规范URI
	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}

	// 规范查询字符串
	canonicalQueryString := s.createCanonicalQueryString(req.URL.Query())

	// 规范头部
	canonicalHeaders := s.createCanonicalHeaders(req)

	// 签名头部
	signedHeaders := s.getSignedHeaders(req)

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		method,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		payloadHash)
}

// createCanonicalQueryString 创建规范查询字符串
func (s *AWSSigner) createCanonicalQueryString(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	var parts []string
	for key, vals := range values {
		for _, val := range vals {
			parts = append(parts, fmt.Sprintf("%s=%s",
				url.QueryEscape(key),
				url.QueryEscape(val)))
		}
	}

	sort.Strings(parts)
	return strings.Join(parts, "&")
}

// createCanonicalHeaders 创建规范头部
func (s *AWSSigner) createCanonicalHeaders(req *http.Request) string {
	var headers []string

	for name := range req.Header {
		lowerName := strings.ToLower(name)
		if s.shouldSignHeader(lowerName) {
			value := strings.TrimSpace(req.Header.Get(name))
			headers = append(headers, fmt.Sprintf("%s:%s", lowerName, value))
		}
	}

	sort.Strings(headers)
	return strings.Join(headers, "\n") + "\n"
}

// getSignedHeaders 获取签名头部列表
func (s *AWSSigner) getSignedHeaders(req *http.Request) string {
	var headers []string

	for name := range req.Header {
		lowerName := strings.ToLower(name)
		if s.shouldSignHeader(lowerName) {
			headers = append(headers, lowerName)
		}
	}

	sort.Strings(headers)
	return strings.Join(headers, ";")
}

// shouldSignHeader 判断是否应该签名该头部
func (s *AWSSigner) shouldSignHeader(name string) bool {
	switch name {
	case "authorization", "user-agent":
		return false
	default:
		return true
	}
}

// createStringToSign 创建待签名字符串
func (s *AWSSigner) createStringToSign(canonicalRequest string, timestamp time.Time) string {
	algorithm := "AWS4-HMAC-SHA256"
	requestDateTime := timestamp.Format("20060102T150405Z")
	credentialScope := s.createCredentialScope(timestamp)

	hasher := sha256.New()
	hasher.Write([]byte(canonicalRequest))
	canonicalRequestHash := fmt.Sprintf("%x", hasher.Sum(nil))

	return fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		requestDateTime,
		credentialScope,
		canonicalRequestHash)
}

// createCredentialScope 创建凭证范围
func (s *AWSSigner) createCredentialScope(timestamp time.Time) string {
	return fmt.Sprintf("%s/%s/%s/aws4_request",
		timestamp.Format("20060102"),
		s.region,
		s.service)
}

// createSigningKey 创建签名密钥
func (s *AWSSigner) createSigningKey(timestamp time.Time) []byte {
	dateKey := s.hmacSHA256([]byte("AWS4"+s.secretAccessKey), timestamp.Format("20060102"))
	regionKey := s.hmacSHA256(dateKey, s.region)
	serviceKey := s.hmacSHA256(regionKey, s.service)
	signingKey := s.hmacSHA256(serviceKey, "aws4_request")

	return signingKey
}

// calculateSignature 计算最终签名
func (s *AWSSigner) calculateSignature(signingKey []byte, stringToSign string) string {
	signature := s.hmacSHA256(signingKey, stringToSign)
	return fmt.Sprintf("%x", signature)
}

// hmacSHA256 计算HMAC-SHA256
func (s *AWSSigner) hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}
