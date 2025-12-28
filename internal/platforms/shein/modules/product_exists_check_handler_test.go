// Package modules 提供SHEIN平台产品存在性检查处理器测试
package modules

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProductExistsCheckHandler_Name(t *testing.T) {
	handler := NewProductExistsCheckHandler()
	assert.Equal(t, "产品存在性检查", handler.Name())
}

func TestProductExistsCheckHandler_Handle_NilManagementClient(t *testing.T) {
	handler := NewProductExistsCheckHandler()
	ctx := &TaskContext{
		ManagementClientMgr: nil,
	}

	// 当管理客户端为空时，应该跳过检查并返回nil
	err := handler.Handle(ctx)
	assert.NoError(t, err)
}

func TestProductExistsCheckHandler_Handle_NilTask(t *testing.T) {
	handler := NewProductExistsCheckHandler()
	ctx := &TaskContext{
		Task: nil,
	}

	// 当任务为空时，应该返回不可重试错误
	err := handler.Handle(ctx)
	assert.Error(t, err)

	// 验证错误是不可重试的
	assert.False(t, IsRetryableError(err))
}
