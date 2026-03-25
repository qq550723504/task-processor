package publish

import (
	"fmt"

	"task-processor/internal/app/state"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/model"
	shein "task-processor/internal/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type PublishProductInput struct {
	Task        *model.Task
	ProductData *sheinproduct.Product
	ProductAPI  sheinproduct.ProductAPI
}

func buildPublishProductInput(ctx *shein.TaskContext) (*PublishProductInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.ProductData == nil {
		return nil, fmt.Errorf("product data is nil")
	}
	if ctx.ProductAPI == nil {
		return nil, fmt.Errorf("product api is nil")
	}

	return &PublishProductInput{
		Task:        ctx.Task,
		ProductData: ctx.ProductData,
		ProductAPI:  ctx.ProductAPI,
	}, nil
}

type ValidationInput struct {
	Task        *model.Task
	ProductData *sheinproduct.Product
}

func buildValidationInput(ctx *shein.TaskContext) (*ValidationInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.ProductData == nil {
		return nil, fmt.Errorf("product data is nil")
	}

	return &ValidationInput{
		Task:        ctx.Task,
		ProductData: ctx.ProductData,
	}, nil
}

type ExistenceCheckInput struct {
	Task                 *model.Task
	ManagementClientMgr  *management.ClientManager
	Variants             *[]model.Product
	SetVariantFilteredFn func(asin string, filteredOut bool, reason string)
}

func buildExistenceCheckInput(ctx *shein.TaskContext) (*ExistenceCheckInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}

	return &ExistenceCheckInput{
		Task:                 ctx.Task,
		ManagementClientMgr:  ctx.ManagementClientMgr,
		Variants:             ctx.Variants,
		SetVariantFilteredFn: ctx.SetVariantFiltered,
	}, nil
}

type MappingRequestInput struct {
	Task               *model.Task
	Variants           *[]model.Product
	UnfilteredVariants *[]model.Product
	StoreInfo          *managementapi.StoreRespDTO
	AmazonProduct      *model.Product
	ProductData        *sheinproduct.Product
	FilterRule         *managementapi.FilterRuleRespDTO
	ProfitRule         *managementapi.ProfitRuleRespDTO
}

func buildMappingRequestInput(ctx *shein.TaskContext) (*MappingRequestInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.Task == nil {
		return nil, fmt.Errorf("task is nil")
	}

	return &MappingRequestInput{
		Task:               ctx.Task,
		Variants:           ctx.Variants,
		UnfilteredVariants: ctx.UnFilteredVariants,
		StoreInfo:          ctx.StoreInfo,
		AmazonProduct:      ctx.AmazonProduct,
		ProductData:        ctx.ProductData,
		FilterRule:         ctx.FilterRule,
		ProfitRule:         ctx.ProfitRule,
	}, nil
}

type SavePublishStateInput struct {
	ProductData         *sheinproduct.Product
	SheinResponse       *sheinproduct.SheinResponse
	SetSupplierSkuMapFn func(platformSKU, supplierSKU string)
}

func buildSavePublishStateInput(ctx *shein.TaskContext, response *sheinproduct.SheinResponse) (*SavePublishStateInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}
	if ctx.ProductData == nil {
		return nil, fmt.Errorf("product data is nil")
	}
	if response == nil {
		return nil, fmt.Errorf("shein response is nil")
	}

	return &SavePublishStateInput{
		ProductData:         ctx.ProductData,
		SheinResponse:       response,
		SetSupplierSkuMapFn: ctx.SetSupplierSkuMapping,
	}, nil
}

type TaskStatusUpdateInput struct {
	Task                *model.Task
	ManagementClientMgr *management.ClientManager
}

func buildTaskStatusUpdateInput(ctx *shein.TaskContext) (*TaskStatusUpdateInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}

	return &TaskStatusUpdateInput{
		Task:                ctx.Task,
		ManagementClientMgr: ctx.ManagementClientMgr,
	}, nil
}

type PublishResultInput struct {
	Task                *model.Task
	ManagementClientMgr *management.ClientManager
	MemoryManager       *state.MemoryManager
	StoreInfo           *managementapi.StoreRespDTO
	SheinResponse       *sheinproduct.SheinResponse
	AsinSkuMap          map[string]string
	MappingInput        *MappingRequestInput
}

func buildPublishResultInput(ctx *shein.TaskContext) (*PublishResultInput, error) {
	if ctx == nil {
		return nil, fmt.Errorf("task context is nil")
	}

	mappingInput, err := buildMappingRequestInput(ctx)
	if err != nil {
		return nil, err
	}

	return &PublishResultInput{
		Task:                ctx.Task,
		ManagementClientMgr: ctx.ManagementClientMgr,
		MemoryManager:       ctx.MemoryManager,
		StoreInfo:           ctx.StoreInfo,
		SheinResponse:       ctx.SheinResponse,
		AsinSkuMap:          ctx.AsinSkuMap,
		MappingInput:        mappingInput,
	}, nil
}
