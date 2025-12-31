// Package types 提供TEMU平台的接口定义
package types

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

// APIClientInterface TEMU API客户端接口，用于解决循环依赖
type APIClientInterface interface {
	// Cookie相关方法
	SetCookies(cookies []*http.Cookie)
	ReloadCookies() error
	HasCookies() bool
	GetCookieCount() int
	GetCookieValue(name string) string
	GetMallID() string
	SetCookieValue(name, value string)
	SetMallID(mallID string)

	// 请求相关方法
	SendTEMURequest(request map[string]any, result any) error
	SendHTTPRequestInterface(method, url string, headers map[string]string, body any, formFields map[string]string, fileFields map[string]any) (interface{}, error)

	// 基础信息获取方法
	GetTenantID() int64
	GetStoreID() int64
	GetBaseURL() string
	GetLogger() *logrus.Entry

	// 产品相关方法
	ListProductsInterface(pageNo, pageSize int) (interface{}, error)
	ListOnShelfProductsInterface(pageNo, pageSize int) (interface{}, error)
	GetProductInterface(goodsID string) (interface{}, error)
}
