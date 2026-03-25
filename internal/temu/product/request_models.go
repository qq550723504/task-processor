package product

import (
	"fmt"

	temuapi "task-processor/internal/temu/api"
	temucontext "task-processor/internal/temu/context"
)

type ProductRequestInput struct {
	APIClient temuapi.APIClientInterface
	Product   *temuapi.Product
	TaskID    string
	ProductID string
}

func buildProductRequestInput(temuCtx *temucontext.TemuTaskContext) (*ProductRequestInput, error) {
	if temuCtx == nil {
		return nil, fmt.Errorf("temu context is nil")
	}
	if temuCtx.APIClient == nil {
		return nil, fmt.Errorf("api client is not initialized")
	}
	if temuCtx.TemuProduct == nil {
		return nil, fmt.Errorf("temu product is nil")
	}

	input := &ProductRequestInput{
		APIClient: temuCtx.APIClient,
		Product:   temuCtx.TemuProduct,
	}
	if task := temuCtx.GetTask(); task != nil {
		input.TaskID = fmt.Sprintf("%d", task.ID)
		input.ProductID = task.ProductID
	}
	return input, nil
}
