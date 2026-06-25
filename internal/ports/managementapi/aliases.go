package managementapi

import (
	original "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/taskstatus"
)

type StoreAPI = original.StoreAPI
type StoreRespDTO = original.StoreRespDTO
type StorePageReqDTO = original.StorePageReqDTO
type StoreIdUpdateReqDTO = original.StoreIdUpdateReqDTO
type StoreStatusUpdateReqDTO = original.StoreStatusUpdateReqDTO
type StorePauseStatusRespDTO = original.StorePauseStatusRespDTO

type FilterRuleAPI = original.FilterRuleAPI
type FilterRuleReqDTO = original.FilterRuleReqDTO
type FilterRuleRespDTO = original.FilterRuleRespDTO

type ProfitRuleAPI = original.ProfitRuleAPI
type ProfitRuleReqDTO = original.ProfitRuleReqDTO
type ProfitRuleRespDTO = original.ProfitRuleRespDTO

type PricingRuleAPI = original.PricingRuleAPI
type PricingRuleReqDTO = original.PricingRuleReqDTO
type PricingRuleRespDTO = original.PricingRuleRespDTO

type ProductImportMappingAPI = original.ProductImportMappingAPI
type ProductImportMappingCreateReqDTO = original.ProductImportMappingCreateReqDTO
type ProductImportMappingCheckReqDTO = original.ProductImportMappingCheckReqDTO
type ProductImportMappingRespDTO = original.ProductImportMappingRespDTO
type ProductImportMappingGetBySkuReqDTO = original.ProductImportMappingGetBySkuReqDTO

type ProductImportTaskRespDTO = original.ProductImportTaskRespDTO
type ProductImportTaskUpdateReqDTO = original.ProductImportTaskUpdateReqDTO

type InventoryRecordAPI = original.InventoryRecordAPI
type ProductDataAPI = original.ProductDataAPI
type ProductDataDTO = original.ProductDataDTO
type ProductDataItemDTO = original.ProductDataItemDTO
type ProductAttributesItemDTO = original.ProductAttributesItemDTO
type ProductDataBatchSaveReqDTO = original.ProductDataBatchSaveReqDTO
type ProductDataBatchUpdateAttributesReqDTO = original.ProductDataBatchUpdateAttributesReqDTO
type ProductDataListByStorePageReqDTO = original.ProductDataListByStorePageReqDTO
type ProductDataRespDTO = original.ProductDataRespDTO
type InventoryRecordCreateReqDTO = original.InventoryRecordCreateReqDTO

type TaskStatusRespDTO = original.TaskStatusRespDTO
type TaskActionRespDTO = original.TaskActionRespDTO

type DailyListingCountAPI = original.DailyListingCountAPI
type DailyListingCountSetReqDTO = original.DailyListingCountSetReqDTO
type TryConsumeDailyQuotaReqDTO = original.TryConsumeDailyQuotaReqDTO
type RollbackDailyQuotaReqDTO = original.RollbackDailyQuotaReqDTO

type PageResult[T any] = original.PageResult[T]
type CommonResult[T any] = original.CommonResult[T]

const (
	ShelfStatusPending   = original.ShelfStatusPending
	ShelfStatusReviewing = original.ShelfStatusReviewing
	ShelfStatusOnShelf   = original.ShelfStatusOnShelf
	ShelfStatusOffShelf  = original.ShelfStatusOffShelf
	ShelfStatusRejected  = original.ShelfStatusRejected
	ShelfStatusDeleted   = original.ShelfStatusDeleted
)

func TaskStatusSnapshotFromDTO(status *TaskStatusRespDTO) *taskstatus.TaskStatusSnapshot {
	if status == nil {
		return nil
	}
	return &taskstatus.TaskStatusSnapshot{
		TaskID:           status.TaskID,
		Status:           status.Status,
		StatusKey:        status.StatusKey,
		StatusName:       status.StatusName,
		CanonicalStatus:  status.CanonicalStatus,
		Platform:         status.Platform,
		Region:           status.Region,
		TaskType:         status.TaskType,
		Priority:         status.Priority,
		RetryCount:       status.RetryCount,
		MaxRetries:       status.MaxRetries,
		ProcessingTimeMs: status.ProcessingTimeMs,
		QueueName:        status.QueueName,
		ProcessingNode:   status.ProcessingNode,
		ProgressPercent:  status.ProgressPercent,
		Result:           status.Result,
		ErrorMessage:     status.ErrorMessage,
		ErrorStack:       status.ErrorStack,
		ExecutionLogs:    status.ExecutionLogs,
		TaskDetails:      status.TaskDetails,
	}
}
