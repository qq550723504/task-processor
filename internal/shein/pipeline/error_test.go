package pipeline

import (
	"testing"

	"task-processor/internal/model"
	shein "task-processor/internal/shein"
)

func TestHandleTaskFailureFilteredErrorReturnsEarly(t *testing.T) {
	handler := &TaskErrorHandler{}

	handler.HandleTaskFailure(model.Task{ID: 1, Priority: 5}, shein.NewFilteredError("低于筛选规则最低价格"), "validate_rules")
}
