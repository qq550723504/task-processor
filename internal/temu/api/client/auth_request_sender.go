// Package client 提供TEMU平台HTTP请求发送功能
package client

import (
	"encoding/json"
	"fmt"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// RequestSender HTTP请求发送器
type RequestSender struct {
	logger *logrus.Entry
}

// NewRequestSender 创建新的请求发送器
func NewRequestSender(logger *logrus.Entry) *RequestSender {
	return &RequestSender{
		logger: logger,
	}
}

// SendRequest 发送HTTP请求
func (s *RequestSender) SendRequest(client ClientAPI, request map[string]any, result any) error {
	// 提取请求参数
	params, err := s.extractRequestParams(request)
	if err != nil {
		return fmt.Errorf("提取请求参数失败: %w", err)
	}

	// 构造完整URL
	fullURL, err := s.buildFullURL(client, params.URL)
	if err != nil {
		return fmt.Errorf("构造URL失败: %w", err)
	}

	// 发送HTTP请求
	response, err := client.SendHTTPRequest(
		params.Method,
		fullURL,
		params.Headers,
		params.Body,
		params.FormFields,
		params.FileFields,
	)
	if err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"method": params.Method,
			"url":    fullURL,
		}).Error("发送HTTP请求失败")
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}

	// 验证响应
	if err := s.validateResponse(response, params.Method, fullURL); err != nil {
		return err
	}

	// 解析响应
	return s.parseResponse(response, result, params.Method, fullURL)
}

// RequestParams 请求参数
type RequestParams struct {
	Method     string
	URL        string
	Headers    map[string]string
	Body       any
	FormFields map[string]string
	FileFields map[string]any
}

// extractRequestParams 提取请求参数
func (s *RequestSender) extractRequestParams(request map[string]any) (*RequestParams, error) {
	method, ok := request["method"].(string)
	if !ok || method == "" {
		return nil, fmt.Errorf("请求方法不能为空")
	}

	url, ok := request["url"].(string)
	if !ok || url == "" {
		return nil, fmt.Errorf("请求URL不能为空")
	}

	headers, _ := request["headers"].(map[string]string)
	body := request["body"]
	formFields, _ := request["formFields"].(map[string]string)
	fileFields, _ := request["fileFields"].(map[string]any)

	return &RequestParams{
		Method:     method,
		URL:        url,
		Headers:    headers,
		Body:       body,
		FormFields: formFields,
		FileFields: fileFields,
	}, nil
}

// buildFullURL 构造完整URL
func (s *RequestSender) buildFullURL(client ClientAPI, url string) (string, error) {
	configInterface := client.GetConfig()
	config, ok := configInterface.(*Config)
	if !ok || config == nil {
		return "", fmt.Errorf("无法获取客户端配置")
	}

	return config.BaseURL + url, nil
}

// validateResponse 验证响应
func (s *RequestSender) validateResponse(response *req.Response, method, fullURL string) error {
	httpManager := &HTTPManager{}
	if !httpManager.IsSuccess(response) {
		// 尝试读取错误响应体
		if errorBody, err := response.ToBytes(); err == nil {
			s.logger.WithFields(logrus.Fields{
				"statusCode":   response.StatusCode,
				"responseBody": string(errorBody),
				"method":       method,
				"url":          fullURL,
			}).Error("HTTP请求失败")
			return fmt.Errorf("HTTP请求失败，状态码: %d，响应体: %s", response.StatusCode, string(errorBody))
		}
		return fmt.Errorf("HTTP请求失败，状态码: %d", response.StatusCode)
	}
	return nil
}

// parseResponse 解析响应
func (s *RequestSender) parseResponse(response *req.Response, result any, method, fullURL string) error {
	respBody, err := response.ToBytes()
	if err != nil {
		s.logger.WithError(err).Error("读取响应体失败")
		return fmt.Errorf("读取响应体失败: %w", err)
	}

	if err := json.Unmarshal(respBody, result); err != nil {
		s.logger.WithError(err).WithFields(logrus.Fields{
			"responseBody": string(respBody),
			"method":       method,
			"url":          fullURL,
		}).Error("JSON解析失败")
		return fmt.Errorf("JSON解析失败: %w", err)
	}

	return nil
}
