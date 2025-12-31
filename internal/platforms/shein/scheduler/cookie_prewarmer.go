// Package scheduler 提供SHEIN平台的Cookie预热功能
package scheduler

import (
	"context"
	"time"

	"task-processor/internal/common/management"
	shops "task-processor/internal/common/shein"
	"task-processor/internal/infra/memory"

	"github.com/sirupsen/logrus"
)

// CookiePrewarmer Cookie预热器，在调度器启动前预热Cookie
type CookiePrewarmer struct {
	managementClient *management.ClientManager
	cookieManager    *memory.CookieManager
	shopClientMgr    *shops.ClientManager
	logger           *logrus.Entry
}

// NewCookiePrewarmer 创建Cookie预热器
func NewCookiePrewarmer(managementClient *management.ClientManager) *CookiePrewarmer {
	cookieManager := memory.NewCookieManager()
	shopClientMgr := shops.NewClientManager(cookieManager, managementClient)

	return &CookiePrewarmer{
		managementClient: managementClient,
		cookieManager:    cookieManager,
		shopClientMgr:    shopClientMgr,
		logger: logrus.WithFields(logrus.Fields{
			"component": "SHEINCookiePrewarmer",
		}),
	}
}

// PrewarmResult Cookie预热结果
type PrewarmResult struct {
	TenantID     int64
	StoreID      int64
	StoreName    string
	Success      bool
	ErrorMessage string
	CookieCount  int
}

// PrewarmCookies 预热指定店铺的Cookie
func (cp *CookiePrewarmer) PrewarmCookies(ctx context.Context, storeIDs []int64) []PrewarmResult {
	cp.logger.Infof("开始预热 %d 个店铺的Cookie", len(storeIDs))

	results := make([]PrewarmResult, 0, len(storeIDs))

	for _, storeID := range storeIDs {
		result := cp.prewarmSingleStore(ctx, storeID)
		results = append(results, result)

		// 避免过于频繁的API调用
		select {
		case <-ctx.Done():
			cp.logger.Info("Cookie预热被取消")
			return results
		case <-time.After(100 * time.Millisecond):
			// 短暂延迟
		}
	}

	// 统计预热结果
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	cp.logger.Infof("Cookie预热完成 - 成功: %d, 失败: %d, 总数: %d",
		successCount, len(results)-successCount, len(results))

	return results
}

// prewarmSingleStore 预热单个店铺的Cookie
func (cp *CookiePrewarmer) prewarmSingleStore(ctx context.Context, storeID int64) PrewarmResult {
	result := PrewarmResult{
		StoreID: storeID,
		Success: false,
	}

	// 获取店铺信息
	storeInfo, err := cp.managementClient.GetStoreClient().GetStore(storeID)
	if err != nil {
		result.ErrorMessage = "获取店铺信息失败: " + err.Error()
		cp.logger.Errorf("获取店铺 %d 信息失败: %v", storeID, err)
		return result
	}

	result.TenantID = storeInfo.TenantID
	result.StoreName = storeInfo.Name

	// 检查平台类型
	if storeInfo.Platform != "SHEIN" && storeInfo.Platform != "shein" {
		result.ErrorMessage = "不是SHEIN平台店铺"
		cp.logger.Debugf("店铺 %s (ID: %d) 不是SHEIN平台，跳过Cookie预热", storeInfo.Name, storeID)
		return result
	}

	// 检查是否启用自动核价
	if storeInfo.EnableAutoPrice != nil && !*storeInfo.EnableAutoPrice {
		result.ErrorMessage = "未启用自动核价"
		cp.logger.Debugf("店铺 %s (ID: %d) 未启用自动核价，跳过Cookie预热", storeInfo.Name, storeID)
		return result
	}

	// 尝试获取API客户端（这会触发Cookie获取）
	client, err := cp.shopClientMgr.GetClient(storeInfo.TenantID, storeID, storeInfo)
	if err != nil {
		result.ErrorMessage = "获取API客户端失败: " + err.Error()
		cp.logger.Errorf("预热店铺 %s (ID: %d) Cookie失败: %v", storeInfo.Name, storeID, err)
		return result
	}

	// 检查Cookie数量
	if apiClient, ok := client.(interface{ GetCookieCount() int }); ok {
		result.CookieCount = apiClient.GetCookieCount()
	}

	result.Success = true
	cp.logger.Infof("✅ 成功预热店铺 %s (ID: %d) Cookie，数量: %d",
		storeInfo.Name, storeID, result.CookieCount)

	return result
}

// GetShopClientManager 获取店铺客户端管理器（供调度器使用）
func (cp *CookiePrewarmer) GetShopClientManager() *shops.ClientManager {
	return cp.shopClientMgr
}

// GetCookieManager 获取Cookie管理器（供调度器使用）
func (cp *CookiePrewarmer) GetCookieManager() *memory.CookieManager {
	return cp.cookieManager
}
