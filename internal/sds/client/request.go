package client

import (
	"context"
	"fmt"
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

	return resp, nil
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
			Op:      fmt.Sprintf("POST %s", path),
			Message: "multipart upload failed",
			Err:     err,
		}
	}

	if !resp.IsSuccessState() {
		return resp, &Error{
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
