// Package browser 提供Amazon浏览器自动化的邮编设置核心功能
package browser

import (
	"task-processor/internal/core/logger"
	"fmt"
	"time"

	"github.com/playwright-community/playwright-go"
)

// ZipcodeSetter 邮编设置器
type ZipcodeSetter struct {
	browserManager *BrowserManager
	maxRetries     int
	getter         *ZipcodeGetter
	inputHandler   *ZipcodeInputHandler
	validator      *ZipcodeValidator
}

// NewZipcodeSetter 创建邮编设置器实例
func NewZipcodeSetter(browserManager *BrowserManager) *ZipcodeSetter {
	return &ZipcodeSetter{
		browserManager: browserManager,
		maxRetries:     3,
		getter:         NewZipcodeGetter(),
		inputHandler:   NewZipcodeInputHandler(),
		validator:      NewZipcodeValidator(),
	}
}

// SetAndVerifyZipcode 设置并验证邮编（基础方法）
// 第二次重试前会刷新页面
func (zs *ZipcodeSetter) SetAndVerifyZipcode(page playwright.Page, zipcode string) error {
	// 如果邮编为空，跳过设置
	if zipcode == "" {
		logger.GetGlobalLogger("crawler/amazon").Infof("邮编为空，跳过设置")
		return nil
	}

	for attempt := 1; attempt <= zs.maxRetries; attempt++ {
		logger.GetGlobalLogger("crawler/amazon").Infof("尝试设置邮编 (第 %d/%d 次): %s", attempt, zs.maxRetries, zipcode)

		// 检查页面是否仍然有效
		if page.IsClosed() {
			return fmt.Errorf("页面已关闭，无法继续操作")
		}

		// 如果是第二次尝试，先刷新页面
		if attempt == 2 {
			if err := zs.refreshPageForRetry(page); err != nil {
				return fmt.Errorf("刷新页面失败: %w", err)
			}
		}

		// 先验证当前邮编是否正确
		if isValid, err := zs.isZipcodeValid(page, zipcode); err == nil && isValid {
			logger.GetGlobalLogger("crawler/amazon").Infof("当前邮编已经是目标邮编: %s，无需设置", zipcode)
			return nil
		}

		logger.GetGlobalLogger("crawler/amazon").Infof("当前邮编不匹配，需要设置邮编")

		// 设置邮编
		if err := zs.inputHandler.SetZipcode(page, zipcode); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Infof("设置邮编失败: %v", err)
			// 检查是否是页面关闭导致的错误
			if page.IsClosed() {
				return fmt.Errorf("页面已关闭: %w", err)
			}
			if attempt == zs.maxRetries {
				return fmt.Errorf("设置邮编失败，已达到最大重试次数: %w", err)
			}

			// 第一次失败后等待，第二次失败会在下次循环开始时刷新页面
			if attempt == 1 {
				logger.GetGlobalLogger("crawler/amazon").Infof("等待 2 秒后重试")
				time.Sleep(2 * time.Second)
			}
			continue
		}

		// 验证邮编
		if isValid, err := zs.isZipcodeValid(page, zipcode); err != nil || !isValid {
			// 检查是否是页面关闭导致的错误
			if page.IsClosed() {
				return fmt.Errorf("页面已关闭: %w", err)
			}
			if attempt == zs.maxRetries {
				return fmt.Errorf("验证邮编失败，已达到最大重试次数: %w", err)
			}

			// 第一次失败后等待，第二次失败会在下次循环开始时刷新页面
			if attempt == 1 {
				logger.GetGlobalLogger("crawler/amazon").Infof("等待 2 秒后重试")
				time.Sleep(2 * time.Second)
			}
			continue
		}

		logger.GetGlobalLogger("crawler/amazon").Infof("成功设置并验证邮编: %s", zipcode)
		return nil
	}

	return fmt.Errorf("设置并验证邮编失败，已达到最大重试次数")
}

// isZipcodeValid 验证当前邮编是否匹配目标邮编（统一的验证入口）
func (zs *ZipcodeSetter) isZipcodeValid(page playwright.Page, expectedZipcode string) (bool, error) {
	return zs.validator.VerifyZipcode(page, expectedZipcode)
}

// refreshPageForRetry 为重试刷新页面
func (zs *ZipcodeSetter) refreshPageForRetry(page playwright.Page) error {
	logger.GetGlobalLogger("crawler/amazon").Infof("第二次尝试前刷新页面")
	if _, err := page.Reload(playwright.PageReloadOptions{
		Timeout: playwright.Float(15000), // 15秒超时，防止 WebSocket 断连时永久 hang
	}); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("刷新页面失败: %v", err)
		return fmt.Errorf("刷新页面失败: %w", err)
	}

	// 等待页面加载完成，使用 DOMContentLoaded 避免 NetworkIdle 在断连时永久等待
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateDomcontentloaded,
		Timeout: playwright.Float(15000),
	}); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Infof("等待页面加载失败: %v", err)
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("页面已刷新，继续尝试设置邮编")
	return nil
}
