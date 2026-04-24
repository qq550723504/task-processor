package adapter

import (
	"task-processor/internal/productimage"
	"task-processor/internal/sds/workflow"
)

// SyncFromImageRequestInput 表示“创建图片任务并同步 SDS”的输入。
type SyncFromImageRequestInput struct {
	SyncInput    workflow.SyncInput
	ImageRequest *productimage.ImageProcessRequest
}

// SyncResult 表示适配层返回的完整上下文。
type SyncResult struct {
	ImageTask   *productimage.Task
	ImageResult *productimage.ImageProcessResult
	DesignSync  *workflow.SyncResult
}
