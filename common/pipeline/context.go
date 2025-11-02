package pipeline

import (
	"context"
	"task-processor/common/amazon"
	"task-processor/common/management/api"
	"task-processor/common/temu"
	"task-processor/common/types"
	temutypes "task-processor/platforms/temu/types"
)

// TaskContext 任务上下文
type TaskContext struct {
	Context         context.Context
	Task            *types.Task
	AmazonProcessor *amazon.AmazonProcessor
	APIClient       *temu.APIClient // TEMU API客户端

	// 强类型字段 - 参考SHEIN的设计
	AmazonProduct   *amazon.Product         // Amazon产品数据
	TemuProduct     *temutypes.Product      // TEMU产品数据
	StoreInfo       *api.StoreRespDTO       // 店铺信息
	RawJsonData     *api.RawJsonDataRespDTO // 原始JSON数据
	DataSource      string                  // 数据源标识
	NeedsAmazonData bool                    // 是否需要Amazon数据

	// 处理结果
	SubmitResult  interface{} // 提交结果
	SaveResult    interface{} // 保存结果
	PublishResult interface{} // 发布结果

	Data map[string]interface{} // 用于在处理器之间传递数据（保留兼容性）
}

// NewTaskContext 创建新的任务上下文
func NewTaskContext(ctx context.Context, task *types.Task) *TaskContext {
	return &TaskContext{
		Context: ctx,
		Task:    task,
		Data:    make(map[string]interface{}),
	}
}

// GetTask 获取任务信息
func (tc *TaskContext) GetTask() *types.Task {
	return tc.Task
}

// GetContext 获取上下文
func (tc *TaskContext) GetContext() context.Context {
	return tc.Context
}

// SetAmazonProcessor 设置Amazon处理器
func (tc *TaskContext) SetAmazonProcessor(processor *amazon.AmazonProcessor) {
	tc.AmazonProcessor = processor
}

// GetAmazonProcessor 获取Amazon处理器
func (tc *TaskContext) GetAmazonProcessor() *amazon.AmazonProcessor {
	return tc.AmazonProcessor
}

// SetData 设置数据
func (tc *TaskContext) SetData(key string, value interface{}) {
	tc.Data[key] = value
}

// GetData 获取数据
func (tc *TaskContext) GetData(key string) (interface{}, bool) {
	value, exists := tc.Data[key]
	return value, exists
}

// GetStringData 获取字符串数据
func (tc *TaskContext) GetStringData(key string) (string, bool) {
	if value, exists := tc.Data[key]; exists {
		if str, ok := value.(string); ok {
			return str, true
		}
	}
	return "", false
}

// GetIntData 获取整数数据
func (tc *TaskContext) GetIntData(key string) (int, bool) {
	if value, exists := tc.Data[key]; exists {
		if i, ok := value.(int); ok {
			return i, true
		}
	}
	return 0, false
}

// SetAPIClient 设置API客户端
func (tc *TaskContext) SetAPIClient(client *temu.APIClient) {
	tc.APIClient = client
}

// GetAPIClient 获取API客户端
func (tc *TaskContext) GetAPIClient() *temu.APIClient {
	return tc.APIClient
}
