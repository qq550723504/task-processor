package design

// UploadRequest 表示设计图上传请求。
type UploadRequest struct {
	FileName    string
	Content     []byte
	ContentType string
	Width       int
	Height      int
	FormFields  map[string]string
}

// CreateMaterialRequest 表示登记素材请求。
type CreateMaterialRequest struct {
	FileCode       string `json:"file_code"`
	Length         int64  `json:"length"`
	Name           string `json:"name"`
	ContentType    string `json:"content_type"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	ParentFolderID int64  `json:"parent_folder_id"`
	RepeatReturnID bool   `json:"repeatReturnId"`
}

// FindMaterialsRequest 表示按 ID 批量查询素材。
type FindMaterialsRequest struct {
	IDs    []int64
	Fields string
}

// ListDesignProductsRequest queries SDS 成品库 for finished rendered images.
type ListDesignProductsRequest struct {
	ProductID       int64
	ParentProductID int64
	Search          string
	DesignType      string
	Page            int
	Size            int
}

// UpdateDesignProductRequest updates SDS finished-product export metadata.
type UpdateDesignProductRequest struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	MaterialImageName string `json:"material_img_name,omitempty"`
	MaterialColor     string `json:"material_color,omitempty"`
	Keyword           string `json:"keyword,omitempty"`
	Attributes        []any  `json:"attributes,omitempty"`
	ParentAttribute   int    `json:"parent_attribute,omitempty"`
}

// SyncDesignRequest 表示 SDS 保存设计请求。
type SyncDesignRequest struct {
	ProductID                    int64                 `json:"product_id"`
	PrototypeGroupID             int64                 `json:"prototypeGroupId"`
	MerchantProductResultGroupID int64                 `json:"merchantProductResultGroupId"`
	DesignType                   string                `json:"designType"`
	Prototypes                   []SyncDesignPrototype `json:"prototypes"`
}

// SyncDesignPrototype 表示单个模板原型设计数据。
type SyncDesignPrototype struct {
	PrototypeID string            `json:"prototype_id"`
	ProductIDs  []int64           `json:"product_ids"`
	PSDIDs      []string          `json:"psd_ids,omitempty"`
	Layers      []SyncDesignLayer `json:"layers"`
	Images      []string          `json:"images,omitempty"`
}

// SyncDesignLayer 表示单个设计图层。
type SyncDesignLayer struct {
	MaterialID         any     `json:"material_id"`
	DesignMaterialID   int64   `json:"design_material_id,omitempty"`
	LayerID            string  `json:"layer_id"`
	Content            string  `json:"content"`
	ImgWidth           int     `json:"img_width"`
	ImgHeight          int     `json:"img_height"`
	ResizeMode         int     `json:"resize_mode"`
	FitLevel           float64 `json:"fit_level"`
	FabricJSON         string  `json:"fabric_json"`
	RelatedMaterialIDs []int64 `json:"related_material_ids,omitempty"`
}

// PrepareSyncDesignInput 表示“上传素材并保存设计”的最小输入。
type PrepareSyncDesignInput struct {
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

// PrepareSyncDesignResult 表示默认构造出的保存设计请求上下文。
type PrepareSyncDesignResult struct {
	Page                       *DesignProductPage
	RelatedPages               map[int64]*DesignProductPage
	Request                    *SyncDesignRequest
	Material                   *UploadedMaterial
	RenderedImageURLs          []string
	RenderedImageURLsByProduct map[int64][]string
	RenderedImageObservations  map[int64]RenderedImageObservation
	RenderedSensitiveWords     map[string][]SensitiveWordHit
}

// RenderedImageObservation captures the latest finished-product-library
// observation for a target variant, even when no usable fused mockup URLs are
// available yet.
type RenderedImageObservation struct {
	ProductID         int64  `json:"product_id,omitempty"`
	Found             bool   `json:"found,omitempty"`
	BuildFinish       bool   `json:"build_finish,omitempty"`
	Status            int    `json:"status,omitempty"`
	MaterialImageName string `json:"material_image_name,omitempty"`
	TaskID            string `json:"task_id,omitempty"`
	DesignTaskID      string `json:"design_task_id,omitempty"`
	ItemID            string `json:"item_id,omitempty"`
	ImageCount        int    `json:"image_count,omitempty"`
	ThumbnailCount    int    `json:"thumbnail_count,omitempty"`
}

// SensitiveWordHit mirrors the SDS sensitive-word response for a rendered
// design product item.
type SensitiveWordHit struct {
	SensitiveWord string `json:"sensitiveWord"`
	Type          int    `json:"type"`
	TypeStrs      string `json:"typeStrs"`
	ImgURL        string `json:"imgUrl"`
	IsParent      int    `json:"isParent"`
	PositionStrs  string `json:"positionStrs"`
}

// SaveDesignRequest mirrors the SDS frontend save payload used by /ps/design/add_and_design.
type SaveDesignRequest struct {
	ProductID        int64                 `json:"product_id"`
	PrototypeGroupID int64                 `json:"prototypeGroupId,omitempty"`
	DesignType       string                `json:"designType,omitempty"`
	Prototypes       []SyncDesignPrototype `json:"prototypes"`
}

// PreviewRequest 表示预览图生成请求。
type PreviewRequest struct {
	Body any
}

// ProductDraftRequest 表示商品草稿创建或更新请求。
type ProductDraftRequest struct {
	Body any
}
