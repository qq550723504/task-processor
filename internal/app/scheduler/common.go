package scheduler

import (
	"task-processor/internal/pkg/management"
	"task-processor/internal/pkg/management/api"
	"task-processor/internal/platforms/shein"
)

// createSheinSyncService 创建 SHEIN 同步服务（公共方法）
func createSheinSyncService(clientManager *management.ClientManager) *shein.SyncService {
	// 创建 repository 工厂函数
	repositoryFactory := func(storeID, tenantID int64) api.ProductDataAPI {
		return clientManager.GetProductDataClientWithTenant(storeID, tenantID)
	}

	// 创建同步服务
	syncService := shein.NewSyncService(repositoryFactory)

	// 设置映射客户端，用于查询 ASIN
	syncService.SetMappingClient(clientManager.GetProductImportMappingClient())

	return syncService
}
