package usecase

import (
	"task-processor/internal/productimage"
	"task-processor/internal/sds/workflow"
)

// SyncInput 表示 SDS 设计同步的统一输入。
type SyncInput = workflow.SyncInput

// ImageSource 表示一个可下载的远程图片源。
type ImageSource = workflow.ImageSource

// RemoteImageInput 表示远程图片同步输入。
type RemoteImageInput struct {
	Sync  SyncInput
	Image workflow.ImageSource
}

// LocalFileInput 表示本地文件同步输入。
type LocalFileInput struct {
	Sync SyncInput
	File workflow.FileSource
}

// ImageResultInput 表示 productimage 结果同步输入。
type ImageResultInput struct {
	Sync        SyncInput
	ImageResult *productimage.ImageProcessResult
}

// ImageRequestInput 表示 productimage 请求并同步输入。
type ImageRequestInput struct {
	Sync         SyncInput
	ImageRequest *productimage.ImageProcessRequest
}
