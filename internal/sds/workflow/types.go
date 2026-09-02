package workflow

import "task-processor/internal/sds/design"

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

// SyncResult 组合 workflow 使用到的上传请求和 SDS 返回结果。
type SyncResult struct {
	UploadRequest design.UploadRequest
	DesignResult  *design.PrepareSyncDesignResult
}
