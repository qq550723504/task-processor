// Package client 提供TEMU平台认证重试处理功能
package client

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// RetryHandler 重试处理器接口
type RetryHandler interface {
	SendRequestWithRetry(client ClientAPI, request map[string]any, result any) error
}

// TemuRetryHandler TEMU平台重试处理器
type TemuRetryHandler struct {
	config        *AuthConfig
	errorDetector ErrorDetector
	pauseHandler  PauseHandler
	logger        *logrus.Entry
}

// NewTemuRetryHandler 创建新的TEMU重试处理器
func NewTemuRetryHandler(
	config *AuthConfig,
	errorDetector ErrorDetector,
	pauseHandler PauseHandler,
	logger *logrus.Entry,
) *TemuRetryHandler {
	return &TemuRetryHandler{
		config:        config,
		errorDetector: errorDetector,
		pauseHandler:  pauseHandler,
		logger:        logger,
	}
}

// SendRequestWithRetry 发送请求（带重试逻辑）
func (h *TemuRetryHandler) SendRequestWithRetry(client ClientAPI, request map[string]any, result any) error {
	ctx := NewAuthContext(client.GetStoreID())

	for ctx.AttemptCount < h.config.MaxRetries {
		ctx.IncrementAttempt()
		h.logger.Debugf("API调用尝试 %d/%d", ctx.AttemptCount, h.config.MaxRetries)

		err := h.sendRequestOnce(client, request, result)
		if err == nil {
			h.logger.Debugf("API调用成功，尝试次数: %d", ctx.AttemptCount)
			return nil
		}

		h.logger.Warnf("API调用失败 (尝试 %d/%d): %v", ctx.AttemptCount, h.config.MaxRetries, err)

		// 处理认证错误
		if h.errorDetector.IsAuthenticationError(err) {
			if authErr := h.handleAuthError(client, ctx, err); authErr != nil {
				return authErr
			}
		} else {
			ctx.ResetAuthError()
		}

		// 如果不是最后一次尝试，等待后重试
		if ctx.AttemptCount < h.config.MaxRetries {
			h.logger.Debugf("等待 %v 后重试...", h.config.RetryDelay)
			time.Sleep(h.config.RetryDelay)
		}
	}

	return fmt.Errorf("API调用失败，已重试%d次", h.config.MaxRetries)
}

// handleAuthError 处理认证错误
func (h *TemuRetryHandler) handleAuthError(client ClientAPI, ctx *AuthContext, err error) error {
	ctx.IncrementAuthError(err)
	h.logger.Infof("检测到认证错误 (连续第%d次)，尝试重新加载Cookie...", ctx.ConsecutiveAuthErrors)

	// 尝试重新加载Cookie
	if reloadErr := client.ReloadCookies(); reloadErr != nil {
		h.logger.Warnf("重新加载Cookie失败: %v", reloadErr)

		// 如果是最后一次尝试且Cookie加载失败，设置暂停键并返回
		if ctx.IsMaxRetryReached(h.config) {
			return h.handleFinalAuthFailure(client, ctx, fmt.Sprintf("认证错误且Cookie重新加载失败: %v", reloadErr))
		}
		return nil
	}

	h.logger.Infof("成功重新加载Cookie，数量: %d", client.GetCookieCount())

	// 如果连续多次认证错误且已经是最后一次尝试，即使Cookie加载成功也设置暂停键
	if ctx.ShouldPause(h.config) && ctx.IsMaxRetryReached(h.config) {
		return h.handleFinalAuthFailure(client, ctx, fmt.Sprintf("连续%d次认证错误，Cookie可能已失效", ctx.ConsecutiveAuthErrors))
	}

	return nil
}

// handleFinalAuthFailure 处理最终认证失败
func (h *TemuRetryHandler) handleFinalAuthFailure(client ClientAPI, ctx *AuthContext, reason string) error {
	h.logger.Error("认证失败，设置认证过期暂停键")

	if pauseErr := h.pauseHandler.SetPauseKeyForAuthExpired(client, reason); pauseErr != nil {
		h.logger.Errorf("设置暂停键失败: %v", pauseErr)
	}

	return NewAuthExpiredError(
		fmt.Sprintf("店铺ID=%d认证过期，已设置暂停键", ctx.StoreID),
		ctx.LastError,
	)
}

// sendRequestOnce 发送单次请求
func (h *TemuRetryHandler) sendRequestOnce(client ClientAPI, request map[string]any, result any) error {
	requestSender := NewRequestSender(h.logger)
	return requestSender.SendRequest(client, request, result)
}
