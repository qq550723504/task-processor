// package sync 提供TEMU平台调度器相关服务
package sync

import (
	managementapi "task-processor/internal/infra/clients/management/api"
)

// buildProductDataItem 将 ProductDataDTO 转换为 ProductDataItemDTO
func buildProductDataItem(prod *managementapi.ProductDataDTO) managementapi.ProductDataItemDTO {
	return managementapi.NewProductDataItemDTO(prod)
}

// buildBatchSaveReq 构建批量保存请求
func buildBatchSaveReq(prod *managementapi.ProductDataDTO, items []managementapi.ProductDataItemDTO) *managementapi.ProductDataBatchSaveReqDTO {
	return managementapi.NewProductDataBatchSaveReqDTO(prod, items)
}
