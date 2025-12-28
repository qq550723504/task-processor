// Package client 提供TEMU平台的请求发送功能
package client

import (
	"encoding/json"
	"fmt"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

// RequestSender 请求发送器
type RequestSender struct {
	client        *req.Client
	config        *Config
	cookieHandler *CookieHandler
	logger        *logrus.Entry
}

// NewRequestSender 创建请求发送器
func NewRequestSender(client *req.Client, config *Config, cookieHandler *CookieHandler, logger *logrus.Entry) *RequestSender {
	return &RequestSender{
		client:        client,
		config:        config,
		cookieHandler: cookieHandler,
		logger:        logger,
	}
}

// SendTEMURequest 发送TEMU API请求（带Cookie检查和重试逻辑）
func (s *RequestSender) SendTEMURequest(request map[string]interface{}, result interface{}) error {
	// 检查Cookie
	if !s.cookieHandler.HasCookies() {
		s.logger.Warnf("店铺没有Cookie数据，尝试重新加载Cookie")

		// 尝试重新加载Cookie
		if err := s.cookieHandler.ReloadCookies(); err != nil {
			// Cookie加载失败，设置暂停键
			s.logger.Errorf("重新加载Cookie失败: %v", err)
			if pauseErr := s.setPauseKeyForAuthExpired("从管理系统获取Cookie失败: Cookie数据为空"); pauseErr != nil {
				s.logger.Errorf("设置暂停键失败: %v", pauseErr)
			}
			// 返回AuthExpiredError以便任务处理器识别并暂停任务
			return NewAuthExpiredError(
				"没有Cookie数据且重新加载失败，请先在管理系统中设置Cookie",
				err,
			)
		}

		// 再次检查Cookie
		if !s.cookieHandler.HasCookies() {
			// Cookie为空，设置暂停键
			s.logger.Warn("Cookie数据为空")
			if pauseErr := s.setPauseKeyForAuthExpired("Cookie数据为空"); pauseErr != nil {
				s.logger.Errorf("设置暂停键失败: %v", pauseErr)
			}
			// 返回AuthExpiredError以便任务处理器识别并暂停任务
			return NewAuthExpiredError(
				"没有Cookie数据，请先在管理系统中设置Cookie",
				nil,
			)
		}

		s.logger.Infof("成功重新加载Cookie，数量: %d", s.cookieHandler.GetCookieCount())
	} else {
		s.logger.Debugf("Cookie检查通过，数量: %d", s.cookieHandler.GetCookieCount())
	}

	// 使用重试逻辑发送请求
	return s.sendTEMURequestWithRetry(request, result)
}

// sendTEMURequestWithRetry 发送TEMU API请求（带重试逻辑）
func (s *RequestSender) sendTEMURequestWithRetry(request map[string]interface{}, result interface{}) error {
	maxRetries := 3

	for attempt := 1; attempt <= maxRetries; attempt++ {
		s.logger.Debugf("API调用尝试 %d/%d", attempt, maxRetries)

		err := s.sendTEMURequestOnce(request, result)
		if err == nil {
			s.logger.Debugf("API调用成功，尝试次数: %d", attempt)
			return nil
		}

		s.logger.Warnf("API调用失败 (尝试 %d/%d): %v", attempt, maxRetries, err)

		// 如果是认证相关错误，尝试重新加载Cookie
		if s.isAuthenticationError(err) {
			s.logger.Infof("检测到认证错误，尝试重新加载Cookie...")
			if reloadErr := s.cookieHandler.ReloadCookies(); reloadErr != nil {
				s.logger.Warnf("重新加载Cookie失败: %v", reloadErr)
				// 如果是最后一次尝试且Cookie加载失败，设置暂停键
				if attempt == maxRetries {
					s.logger.Error("所有重试均失败，设置认证过期暂停键")
					if pauseErr := s.setPauseKeyForAuthExpired(fmt.Sprintf("认证错误且Cookie重新加载失败: %v", reloadErr)); pauseErr != nil {
						s.logger.Errorf("设置暂停键失败: %v", pauseErr)
					}
				}
			} else {
				s.logger.Infof("成功重新加载Cookie，数量: %d", s.cookieHandler.GetCookieCount())
			}
		}

		// 如果不是最后一次尝试，记录重试信息
		if attempt < maxRetries {
			s.logger.Debugf("准备重试...")
		}
	}

	return fmt.Errorf("API调用失败，已重试%d次", maxRetries)
}

// sendTEMURequestOnce 发送单次TEMU API请求 - 使用req库
func (s *RequestSender) sendTEMURequestOnce(request map[string]interface{}, result interface{}) error {
	// 从request map中提取参数
	method, ok := request["method"].(string)
	if !ok {
		return fmt.Errorf("请求方法不能为空")
	}

	url, ok := request["url"].(string)
	if !ok {
		return fmt.Errorf("请求URL不能为空")
	}

	headers, _ := request["headers"].(map[string]string)
	body := request["body"]
	formFields, _ := request["formFields"].(map[string]string)
	fileFields, _ := request["fileFields"].(map[string]interface{})

	// 验证请求参数
	if err := s.validateRequest(method, url); err != nil {
		return fmt.Errorf("请求参数验证失败: %w", err)
	}

	// 构造完整URL
	fullURL := s.config.BaseURL + url

	// 发送HTTP请求
	response, err := s.sendHTTPRequest(method, fullURL, headers, body, formFields, fileFields)
	if err != nil {
		return fmt.Errorf("发送HTTP请求失败: %w", err)
	}

	// 检查HTTP状态码
	if !s.isSuccess(response) {
		// 尝试读取错误响应体
		if errorBody, err := response.ToBytes(); err == nil {
			s.logger.Errorf("HTTP请求失败，状态码: %d, 响应体: %s", response.StatusCode, string(errorBody))
		}
		return fmt.Errorf("HTTP请求失败，状态码: %d", response.StatusCode)
	}

	// 解析响应体
	respBody, err := response.ToBytes()
	if err != nil {
		return fmt.Errorf("读取响应体失败: %w", err)
	}

	return json.Unmarshal(respBody, result)
}

// sendHTTPRequest 发送HTTP请求的内部方法 - 使用req库
func (s *RequestSender) sendHTTPRequest(method, url string, headers map[string]string, body interface{}, formFields map[string]string, fileFields map[string]interface{}) (*req.Response, error) {
	// 创建动态请求
	r := s.client.R()

	// 设置请求头
	for key, value := range headers {
		r.SetHeader(key, value)
	}

	// 如果有文件字段，则使用multipart/form-data格式
	if len(fileFields) > 0 {
		// 添加表单字段
		if len(formFields) > 0 {
			r.SetFormData(formFields)
		}

		// 添加文件字段
		for fieldName, fileData := range fileFields {
			if fileInfo, ok := fileData.(map[string]interface{}); ok {
				filename, _ := fileInfo["filename"].(string)
				content, _ := fileInfo["content"].([]byte)
				r.SetFileBytes(fieldName, filename, content)
			}
		}

		// 根据方法类型发送请求
		switch method {
		case "GET":
			return r.Get(url)
		case "POST":
			return r.Post(url)
		case "PUT":
			return r.Put(url)
		case "DELETE":
			return r.Delete(url)
		default:
			return r.Send(method, url)
		}
	} else {
		// 否则使用JSON格式
		// 如果没有设置Content-Type，则默认为JSON
		if _, exists := headers["content-type"]; !exists && body != nil {
			r.SetHeader("content-type", "application/json;charset=UTF-8")
		}

		// 根据方法类型发送请求
		switch method {
		case "GET":
			return r.Get(url)
		case "POST":
			return r.SetBody(body).Post(url)
		case "PUT":
			return r.SetBody(body).Put(url)
		case "DELETE":
			return r.SetBody(body).Delete(url)
		default:
			return r.SetBody(body).Send(method, url)
		}
	}
}

// validateRequest 验证请求参数
func (s *RequestSender) validateRequest(method, url string) error {
	if method == "" {
		return fmt.Errorf("请求方法不能为空")
	}

	if url == "" {
		return fmt.Errorf("请求URL不能为空")
	}

	return nil
}

// isSuccess 检查响应是否成功
func (s *RequestSender) isSuccess(response *req.Response) bool {
	if response == nil {
		return false
	}
	return response.StatusCode >= 200 && response.StatusCode < 300
}

// isAuthenticationError 检查是否为认证错误
func (s *RequestSender) isAuthenticationError(err error) bool {
	if err == nil {
		return false
	}

	errorStr := err.Error()
	// 检查常见的认证错误关键词
	authErrorKeywords := []string{
		"401",
		"403",
		"unauthorized",
		"forbidden",
		"authentication",
		"login",
		"cookie",
		"session",
	}

	for _, keyword := range authErrorKeywords {
		if contains(errorStr, keyword) {
			return true
		}
	}

	return false
}

// setPauseKeyForAuthExpired 设置认证过期暂停键
func (s *RequestSender) setPauseKeyForAuthExpired(reason string) error {
	// TODO: 实现暂停键设置逻辑
	s.logger.Warnf("需要设置暂停键，原因: %s", reason)
	return nil
}

// contains 检查字符串是否包含子字符串（忽略大小写）
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			(len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					indexOf(s, substr) >= 0)))
}

// indexOf 查找子字符串位置
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
