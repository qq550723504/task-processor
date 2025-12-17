package task

import (
	"task-processor/internal/common/management"
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/management/impl"
)

// ManagementClientAdapter 管理客户端适配器
type ManagementClientAdapter struct {
	clientManager *management.ClientManager
}

// NewManagementClientAdapter 创建管理客户端适配器
func NewManagementClientAdapter(clientManager *management.ClientManager) ManagementClientProvider {
	return &ManagementClientAdapter{
		clientManager: clientManager,
	}
}

// GetImportTaskClient 获取导入任务API客户端
func (a *ManagementClientAdapter) GetImportTaskClient() ImportTaskClient {
	return &ImportTaskClientAdapter{
		client: a.clientManager.GetImportTaskClient(),
	}
}

// GetStoreClient 获取店铺API客户端
func (a *ManagementClientAdapter) GetStoreClient() StoreClient {
	return &StoreClientAdapter{
		client: a.clientManager.GetStoreClient(),
	}
}

// ImportTaskClientAdapter 导入任务客户端适配器
type ImportTaskClientAdapter struct {
	client *impl.ImportTaskAPIClientImpl
}

// GetPendingAndRetryTasks 获取待处理和重试任务
func (a *ImportTaskClientAdapter) GetPendingAndRetryTasks(maxTasks int, userID int64, storeIDs []int64) ([]TaskDTO, error) {
	tasks, err := a.client.GetPendingAndRetryTasks(maxTasks, userID, storeIDs)
	if err != nil {
		return nil, err
	}

	// 转换为DTO
	result := make([]TaskDTO, len(tasks))
	for i, task := range tasks {
		result[i] = TaskDTO{
			ID:         task.ID,
			TenantID:   task.TenantID,
			ProductID:  task.ProductID,
			Platform:   task.Platform,
			Region:     task.Region,
			StoreID:    task.StoreID,
			CategoryID: task.CategoryID,
			CreateTime: task.CreateTime,
			RetryCount: task.RetryCount,
			Priority:   task.Priority,
			Creator:    task.Creator,
		}
	}
	return result, nil
}

// UpdateTaskStatus 更新任务状态
func (a *ImportTaskClientAdapter) UpdateTaskStatus(taskID int64, status int16, errorMessage string) error {
	req := &api.ProductImportTaskUpdateReqDTO{
		ID:           taskID,
		Status:       status,
		ErrorMessage: errorMessage,
	}
	return a.client.UpdateTaskStatus(req)
}

// StoreClientAdapter 店铺客户端适配器
type StoreClientAdapter struct {
	client *impl.StoreAPIClientImpl
}

// GetStore 获取店铺信息
func (a *StoreClientAdapter) GetStore(storeID int64) (*StoreDTO, error) {
	store, err := a.client.GetStore(storeID)
	if err != nil {
		return nil, err
	}

	return &StoreDTO{
		ID:       store.ID,
		Platform: store.Platform,
		Name:     store.Name,
	}, nil
}

// DirectManagementClientProvider 直接使用ClientManager的提供者（用于向后兼容）
type DirectManagementClientProvider struct {
	*management.ClientManager
}

// GetImportTaskClient 获取导入任务API客户端
func (p *DirectManagementClientProvider) GetImportTaskClient() ImportTaskClient {
	return &ImportTaskClientAdapter{
		client: p.ClientManager.GetImportTaskClient(),
	}
}

// GetStoreClient 获取店铺API客户端
func (p *DirectManagementClientProvider) GetStoreClient() StoreClient {
	return &StoreClientAdapter{
		client: p.ClientManager.GetStoreClient(),
	}
}

// WrapManagementClient 包装ClientManager为接口
func WrapManagementClient(clientManager *management.ClientManager) ManagementClientProvider {
	if clientManager == nil {
		return nil
	}
	return &DirectManagementClientProvider{
		ClientManager: clientManager,
	}
}

// UnwrapToAPITask 将TaskDTO转换为API任务类型（如果需要）
func UnwrapToAPITask(dto TaskDTO) *api.ProductImportTaskRespDTO {
	return &api.ProductImportTaskRespDTO{
		ID:         dto.ID,
		TenantID:   dto.TenantID,
		ProductID:  dto.ProductID,
		Platform:   dto.Platform,
		Region:     dto.Region,
		StoreID:    dto.StoreID,
		CategoryID: dto.CategoryID,
		CreateTime: dto.CreateTime,
		RetryCount: dto.RetryCount,
		Priority:   dto.Priority,
		Creator:    dto.Creator,
	}
}
