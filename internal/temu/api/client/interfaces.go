// Package client 提供TEMU平台的接口定义
package client

import (
	"net/http"
	"task-processor/internal/listingadmin"

	"github.com/imroc/req/v3"
	"github.com/sirupsen/logrus"
)

type StoreRuntime interface {
	GetStoreAPI() listingadmin.StoreAPI
}

// ClientAPI TEMU API客户端接口，用于解决循环依赖
type ClientAPI interface {
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
	SendHTTPRequest(method, url string, headers map[string]string, body any, formFields map[string]string, fileFields map[string]any) (*req.Response, error)

	// 基础信息获取方法
	GetStoreID() int64
	GetBaseURL() string
	GetLogger() *logrus.Entry

	// 认证管理需要的方法
	GetConfig() any
	GetCookieManager() any
}
