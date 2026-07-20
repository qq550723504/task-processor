package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/imroc/req/v3"
)

// MultipartFile 表示 multipart 上传文件。
type MultipartFile struct {
	FieldName string
	FileName  string
	Content   []byte
}

// Do 发送通用 HTTP 请求。
func (c *Client) Do(
	ctx context.Context,
	method string,
	path string,
	query map[string]string,
	body any,
	result any,
) (*req.Response, error) {
	op := fmt.Sprintf("%s %s", method, path)
	for attempt := 0; attempt < 2; attempt++ {
		resp, err := c.send(ctx, method, path, query, body, result)
		if authErr := c.detectAuthError(op, path, resp, err); authErr != nil {
			if attempt == 0 && c.shouldRetryWithFreshAuth(path) {
				refreshed, refreshErr := c.bootstrapAuth(ctx, true)
				if refreshErr != nil {
					return resp, refreshErr
				}
				if refreshed {
					continue
				}
			}
			if !c.shouldRetryWithFreshAuth(path) {
				c.ClearAuthState()
			}
			return resp, authErr
		}
		return resp, err
	}

	return nil, &Error{Op: op, Message: "request retry exhausted"}
}

// UploadFile 发送 multipart 文件上传请求。
func (c *Client) UploadFile(
	ctx context.Context,
	path string,
	form map[string]string,
	file MultipartFile,
	result any,
) (*req.Response, error) {
	url := c.resolveURL(path)
	reqBuilder := c.httpClient.R().
		SetContext(ctx).
		SetFormData(form).
		SetFileBytes(file.FieldName, file.FileName, file.Content)

	if result != nil {
		reqBuilder.SetSuccessResult(result)
	}

	resp, err := reqBuilder.Post(url)
	if err != nil {
		return nil, &Error{
			Kind:    ErrorKindMultipartUpload,
			Op:      fmt.Sprintf("POST %s", path),
			Message: "multipart upload failed",
			Err:     err,
		}
	}

	if !resp.IsSuccessState() {
		return resp, &Error{
			Kind:       ErrorKindMultipartUpload,
			Op:         fmt.Sprintf("POST %s", path),
			StatusCode: resp.StatusCode,
			Message:    resp.String(),
		}
	}

	return resp, nil
}

func (c *Client) resolveURL(path string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path
	}

	baseURL := strings.TrimRight(c.config.BaseURL, "/")
	cleanPath := path
	if cleanPath == "" {
		cleanPath = "/"
	}
	if !strings.HasPrefix(cleanPath, "/") {
		cleanPath = "/" + cleanPath
	}

	return baseURL + cleanPath
}

func (c *Client) send(
	ctx context.Context,
	method string,
	path string,
	query map[string]string,
	body any,
	result any,
) (*req.Response, error) {
	url := c.resolveURL(path)
	reqBuilder := c.httpClient.R().SetContext(ctx)

	if len(query) > 0 {
		reqBuilder.SetQueryParams(query)
	}

	if result != nil {
		reqBuilder.SetSuccessResult(result)
	}

	if body != nil {
		reqBuilder.SetBody(body)
	}

	resp, err := reqBuilder.Send(method, url)
	if err != nil {
		return nil, &Error{
			Op:      fmt.Sprintf("%s %s", method, path),
			Message: "request send failed",
			Err:     err,
		}
	}

	if !resp.IsSuccessState() {
		return resp, &Error{
			Op:         fmt.Sprintf("%s %s", method, path),
			StatusCode: resp.StatusCode,
			Message:    resp.String(),
		}
	}

	if ret, msg, ok := parseBusinessError(resp); ok && ret != 0 {
		return resp, &Error{
			Op:         fmt.Sprintf("%s %s", method, path),
			StatusCode: resp.StatusCode,
			Message:    msg,
		}
	}

	return resp, nil
}

func (c *Client) shouldRetryWithFreshAuth(path string) bool {
	if c == nil || c.config == nil || !c.config.AuthBootstrap.HasSource() {
		return false
	}
	loginPath := strings.TrimSpace(c.config.Endpoints.LoginPath)
	if loginPath == "" {
		return true
	}
	return path != loginPath
}

func (c *Client) detectAuthError(op, path string, resp *req.Response, err error) error {
	if err == nil && resp == nil {
		return nil
	}

	if statusErr, ok := err.(*Error); ok {
		if isHTTPAuthFailure(statusErr.StatusCode) {
			return &AuthRequiredError{Op: op, StatusCode: statusErr.StatusCode, Message: statusErr.Message}
		}
		if isBodyAuthFailure(statusErr.StatusCode, statusErr.Message) {
			return &AuthRequiredError{Op: op, StatusCode: statusErr.StatusCode, Message: statusErr.Message}
		}
		if resp != nil {
			if ret, msg, ok := parseBusinessError(resp); ok && ret == 20001 {
				return &AuthRequiredError{Op: op, StatusCode: statusErr.StatusCode, Message: msg}
			}
			if isBodyAuthFailure(resp.StatusCode, resp.String()) {
				return &AuthRequiredError{Op: op, StatusCode: resp.StatusCode, Message: strings.TrimSpace(resp.String())}
			}
		}
	}

	if resp != nil {
		if ret, msg, ok := parseBusinessError(resp); ok && ret == 20001 {
			return &AuthRequiredError{Op: op, StatusCode: resp.StatusCode, Message: msg}
		}
		if isBodyAuthFailure(resp.StatusCode, resp.String()) {
			return &AuthRequiredError{Op: op, StatusCode: resp.StatusCode, Message: strings.TrimSpace(resp.String())}
		}
	}

	return nil
}

func isHTTPAuthFailure(statusCode int) bool {
	return statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden
}

func isBodyAuthFailure(statusCode int, message string) bool {
	if statusCode != http.StatusBadRequest {
		return false
	}

	normalized := strings.ToLower(strings.TrimSpace(message))
	if normalized == "" {
		return false
	}

	indicators := []string{
		"用户未登录",
		"auth required",
		"login required",
		"not logged in",
		"unauthenticated",
	}
	for _, indicator := range indicators {
		if strings.Contains(normalized, strings.ToLower(indicator)) {
			return true
		}
	}
	return false
}

func parseBusinessError(resp *req.Response) (int, string, bool) {
	if resp == nil {
		return 0, "", false
	}
	body := resp.Bytes()
	if len(body) == 0 || !bytes.Contains(body, []byte(`"ret"`)) {
		return 0, "", false
	}

	var payload struct {
		Ret int    `json:"ret"`
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, "", false
	}
	return payload.Ret, payload.Msg, true
}

func buildDefaultHeaders(config *Config) map[string]string {
	return map[string]string{
		"Accept":             "application/json, text/plain, */*",
		"Accept-Language":    "zh-CN,zh;q=0.9,en;q=0.8",
		"Cache-Control":      "no-cache",
		"Pragma":             "no-cache",
		"Priority":           "u=1, i",
		"Referer":            config.Referer,
		"User-Agent":         config.UserAgent,
		"Sec-Ch-Ua":          `"Chromium";v="140", "Not=A?Brand";v="24", "Google Chrome";v="140"`,
		"Sec-Ch-Ua-Mobile":   "?0",
		"Sec-Ch-Ua-Platform": `"Windows"`,
		"Sec-Fetch-Dest":     "empty",
		"Sec-Fetch-Mode":     "cors",
		"Sec-Fetch-Site":     "same-origin",
	}
}
