package workflow

import (
	"task-processor/internal/productimage"
	"task-processor/internal/sds/design"
)

// SyncInput 表示保存 SDS 设计所需的业务参数。
type SyncInput struct {
	VariantID              int64
	RelatedVariantIDs      []int64
	RelatedVariantLayerIDs map[int64]string
	ParentProductID        int64
	PrototypeGroupID       int64
	MerchantResultID       int64
	DesignType             string
	LayerID                string
	FitLevel               float64
	ResizeMode             int
	BlankDesignURL         string
}

// ImageSource 表示一个可下载的远程图片源。
type ImageSource struct {
	URL         string
	FileName    string
	ContentType string
	Width       int
	Height      int
}

// FileSource 表示一个本地图片文件源。
type FileSource struct {
	Path        string
	FileName    string
	ContentType string
	Width       int
	Height      int
}

// AssetSource 表示一个 productimage 产出的图片资产源。
type AssetSource struct {
	Asset *productimage.ImageAsset
}

// SyncResult 组合 workflow 使用到的上传请求和 SDS 返回结果。
type SyncResult struct {
	UploadRequest design.UploadRequest
	DesignResult  *design.PrepareSyncDesignResult
}
