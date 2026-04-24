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
	DesignType      string
	Page            int
	Size            int
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
	VariantID        int64
	ParentProductID  int64
	PrototypeGroupID int64
	MerchantResultID int64
	DesignType       string
	LayerID          string
	FitLevel         float64
	ResizeMode       int
}

// PrepareSyncDesignResult 表示默认构造出的保存设计请求上下文。
type PrepareSyncDesignResult struct {
	Page              *DesignProductPage
	Request           *SyncDesignRequest
	Material          *UploadedMaterial
	RenderedImageURLs []string
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
