package build

import (
	"github.com/google/uuid"

	shein "task-processor/internal/shein"
)

type BuildSpuHandler struct{}

func NewBuildSpuHandler() *BuildSpuHandler {
	return &BuildSpuHandler{}
}

func (h *BuildSpuHandler) Name() string {
	return "build_spu"
}

func (h *BuildSpuHandler) Handle(ctx *shein.TaskContext) error {
	input, err := buildSpuInput(ctx)
	if err != nil {
		return err
	}
	buildSpuData(input)
	return nil
}

func buildSpuData(input *BuildSpuInput) {
	supplierCode := input.AsinSkuMap[input.TaskID]
	input.ProductData.SupplierCode = supplierCode
	input.ProductData.PointKey = uuid.New().String()
}
