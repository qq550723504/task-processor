package product

import (
	"fmt"

	"task-processor/internal/model"
	temuapi "task-processor/internal/temu/api"
	models "task-processor/internal/temu/api/product"
	temucontext "task-processor/internal/temu/context"
)

type SavePublishResultInput struct {
	Task           *model.Task
	SubmitResponse *temuapi.SubmitResponse
	Product        *models.Product
}

func buildSavePublishResultInput(temuCtx *temucontext.TemuTaskContext) (*SavePublishResultInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}

	task := temuCtx.GetTask()
	if task == nil {
		return nil, fmt.Errorf("task is not initialized")
	}

	submitResponse, exists := getSubmitResponseFromContext(temuCtx)
	if !exists {
		return nil, fmt.Errorf("submit response is not initialized")
	}

	product := temuCtx.ProductData
	if product == nil {
		product = temuCtx.TemuProduct
	}

	return &SavePublishResultInput{
		Task:           task,
		SubmitResponse: submitResponse,
		Product:        product,
	}, nil
}
