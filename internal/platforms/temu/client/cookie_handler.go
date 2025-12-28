// Package client 提供TEMU平台的Cookie处理功能
package client

import (
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

// CookieHandler Cookie处理器
type CookieHandler struct {
	cookies       []*http.Cookie
	cookieManager *CookieManager
	storeID       int64
	logger        *logrus.Entry
}

// NewCookieHandler 创建Cookie处理器
func NewCookieHandler(storeID int64, cookieManager *CookieManager, logger *logrus.Entry) *CookieHandler {
	return &CookieHandler{
		cookies:       make([]*http.Cookie, 0),
		cookieManager: cookieManager,
		storeID:       storeID,
		logger:        logger,
	}
}

// SetCookies 设置Cookie
func (h *CookieHandler) SetCookies(cookies []*http.Cookie) {
	h.cookies = cookies
	h.logger.WithField("cookieNum", len(cookies)).Info("设置Cookie")
}

// GetCookies 获取Cookie
func (h *CookieHandler) GetCookies() []*http.Cookie {
	return h.cookies
}

// ReloadCookies 重新加载Cookie
func (h *CookieHandler) ReloadCookies() error {
	cookies, err := h.cookieManager.LoadCookies()
	if err != nil {
		h.logger.WithError(err).Error("重新加载Cookie失败")
		return fmt.Errorf("重新加载Cookie失败: %w", err)
	}

	if cookies != nil {
		h.SetCookies(cookies)
		h.logger.Info("成功重新加载Cookie")
	} else {
		h.logger.Info("未找到Cookie数据")
	}

	return nil
}

// HasCookies 检查是否有Cookie
func (h *CookieHandler) HasCookies() bool {
	return len(h.cookies) > 0
}

// GetCookieCount 获取Cookie数量
func (h *CookieHandler) GetCookieCount() int {
	return len(h.cookies)
}

// InitializeCookies 初始化Cookie（在客户端创建时调用）
func (h *CookieHandler) InitializeCookies() {
	// 在初始化时测试管理系统连接
	if err := h.cookieManager.TestConnection(); err != nil {
		h.logger.WithError(err).Error("管理系统连接测试失败，跳过Cookie加载")
	} else {
		// 连接正常，尝试加载Cookie
		if cookies, err := h.cookieManager.LoadCookies(); err != nil {
			h.logger.WithError(err).Error("初始化时加载Cookie失败")
		} else if cookies != nil {
			h.SetCookies(cookies)
			h.logger.Info("成功在初始化时加载Cookie")
		} else {
			h.logger.Info("初始化时未找到Cookie数据")
		}
	}
}
