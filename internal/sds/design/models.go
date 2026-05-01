package design

import (
	"encoding/json"
	"strconv"
	"strings"
)

// FlexibleInt accepts integer-like SDS JSON values that sometimes arrive as
// 1024.0 instead of 1024.
type FlexibleInt int

func (v *FlexibleInt) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "" || raw == "null" {
		*v = 0
		return nil
	}
	var number float64
	if err := json.Unmarshal(data, &number); err == nil {
		*v = FlexibleInt(number)
		return nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return err
	}
	if text == "" {
		*v = 0
		return nil
	}
	parsed, err := strconv.ParseFloat(text, 64)
	if err != nil {
		return err
	}
	*v = FlexibleInt(parsed)
	return nil
}

// UploadSignature 表示 OSS 直传签名。
type UploadSignature struct {
	Dir            string `json:"dir"`
	Policy         string `json:"policy"`
	OSSAccessKeyID string `json:"ossAccessKeyId"`
	Signature      string `json:"signature"`
	Host           string `json:"host"`
}

// UploadedImage 描述上传后的图片信息。
type UploadedImage struct {
	Key         string `json:"key"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	MD5Name     string `json:"md5Name"`
	ImageURL    string `json:"imageUrl"`
	ContentType string `json:"contentType"`
}

// Material 是 SDS 素材库记录。
type Material struct {
	ID          int64       `json:"id"`
	Name        string      `json:"name"`
	FileCode    string      `json:"file_code"`
	ContentType string      `json:"content_type"`
	Width       FlexibleInt `json:"width"`
	Height      FlexibleInt `json:"height"`
	ImageURL    string      `json:"img_url"`
	ImageURLAlt string      `json:"imgUrl"`
}

// CreateMaterialResponse 表示素材登记接口响应。
type CreateMaterialResponse struct {
	Ret  int        `json:"ret"`
	Msg  string     `json:"msg"`
	Data []Material `json:"data"`
}

// UploadedMaterial 组合上传后的 OSS 文件和素材库记录。
type UploadedMaterial struct {
	Image    *UploadedImage `json:"image"`
	Material *Material      `json:"material"`
}

// DesignProductListResponse is the SDS finished-product library response.
type DesignProductListResponse struct {
	Items      []DesignProductListItem `json:"items"`
	TotalCount int                     `json:"total_count"`
}

// DesignProductListItem contains the rendered output images from 成品库.
type DesignProductListItem struct {
	ID                 string                  `json:"id"`
	ProductID          int64                   `json:"product_id"`
	ProductParentID    int64                   `json:"product_parent_id"`
	DesignTaskID       string                  `json:"design_task_id"`
	TaskID             string                  `json:"task_id"`
	PrototypeID        string                  `json:"prototype_id"`
	BuildFinish        bool                    `json:"buildFinish"`
	Status             int                     `json:"status"`
	ExportName         string                  `json:"export_name"`
	MaterialImageName  string                  `json:"material_img_name"`
	MaterialColor      string                  `json:"material_color"`
	Keyword            string                  `json:"keyword"`
	ParentAttribute    int                     `json:"parent_attribute"`
	Attributes         []any                   `json:"attributes"`
	MaterialVariant    []DesignProductListItem `json:"material_variant"`
	FinishTime         int64                   `json:"finish_time"`
	GMTFinish          string                  `json:"gmt_finish"`
	ImageURLs          []string                `json:"img_urls"`
	ThumbnailImageURLs []string                `json:"thumb_img_urls"`
}

// DesignProductPage 表示设计页初始化接口 `/ps/design/products/{variantId}`。
type DesignProductPage struct {
	MerchantProduct              MerchantProductStub `json:"merchantProduct"`
	Product                      DesignProduct       `json:"product"`
	PrototypeGroup               PrototypeGroup      `json:"prototypeGroup"`
	MerchantProductResultGroupID int64               `json:"merchantProductResultGroupId"`
	MerchantProductParentID      int64               `json:"merchant_product_parent_id"`
	DesignStatus                 int                 `json:"design_status"`
	Layers                       []DesignLayer       `json:"layers"`
	PSDs                         []PSDDocument       `json:"psds"`
}

// MerchantProductStub 保留商家商品最小结构。
type MerchantProductStub struct {
	DelFlag    string `json:"delFlag"`
	Type       int    `json:"type"`
	GMTCreated string `json:"gmt_created"`
}

// DesignProduct 表示设计页里的 SKU 商品。
type DesignProduct struct {
	ID             int64     `json:"id"`
	ParentID       int64     `json:"parent_id"`
	Name           string    `json:"name"`
	SKU            string    `json:"sku"`
	ParentSKU      string    `json:"parentSku"`
	PrototypeID    string    `json:"prototypeId"`
	PrototypeType  string    `json:"prototypeType"`
	Size           string    `json:"size"`
	SizeID         int64     `json:"sizeId"`
	ColorID        int64     `json:"colorId"`
	ColorName      string    `json:"color_name"`
	ImgURL         string    `json:"img_url"`
	PSDImgURL      string    `json:"psd_img_url"`
	ProductDetails any       `json:"product_details"`
	Color          ColorInfo `json:"color"`
	SizeDTO        SizeDTO   `json:"sizeDto"`
}

// ColorInfo 表示颜色信息。
type ColorInfo struct {
	ColorSort   int    `json:"colorSort"`
	ChineseName string `json:"chineseName"`
	Color       string `json:"color"`
	Opacity     int    `json:"opacity"`
	ColorName   string `json:"color_name"`
	ColorID     int64  `json:"colorId"`
}

// SizeDTO 表示尺码信息。
type SizeDTO struct {
	ID       int64  `json:"id"`
	SizeName string `json:"sizeName"`
}

// PrototypeGroup 表示模板组。
type PrototypeGroup struct {
	ID                 int64  `json:"id"`
	TenantID           int64  `json:"tenantId"`
	ProductID          int64  `json:"productId"`
	PrototypeGroupName string `json:"prototypeGroupName"`
	PrototypeType      string `json:"prototypeType"`
	Scope              string `json:"scope"`
	Visible            string `json:"visible"`
	DesignLayerNum     int    `json:"designLayerNum"`
}

// DesignLayer 表示设计图层。
type DesignLayer struct {
	ID             string   `json:"id"`
	PrototypeID    string   `json:"prototypeId"`
	Name           string   `json:"name"`
	Type           int      `json:"type"`
	Height         float64  `json:"height"`
	Width          float64  `json:"width"`
	PrintHeight    float64  `json:"printHeight"`
	PrintWidth     float64  `json:"printWidth"`
	PrintHeightAlt float64  `json:"print_height"`
	PrintWidthAlt  float64  `json:"print_width"`
	IsMasterMap    int      `json:"isMasterMap"`
	IsMasterMapAlt int      `json:"is_master_map"`
	MaskURL        string   `json:"maskUrl"`
	MaskShowURL    string   `json:"maskShowUrl"`
	MaskURLAlt     string   `json:"mask_url"`
	MaskShowURLAlt string   `json:"mask_show_url"`
	ThumbnailURL   string   `json:"thumbnailUrl"`
	ThumbnailURLs  []string `json:"thumbnail_urls"`
	PageType       string   `json:"pageType"`
}

// PSDDocument 表示 PSD 模板文件。
type PSDDocument struct {
	ID           string `json:"id"`
	PrototypeID  string `json:"prototypeId"`
	FileID       string `json:"fileId"`
	FileCode     string `json:"fileCode"`
	FileName     string `json:"file_name"`
	ShowFileName string `json:"showFileName"`
	FileURL      string `json:"file_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Sort         int    `json:"sort"`
}

// ResultGroupOption 表示结果分组选项。
type ResultGroupOption struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// CutFileContent 表示 cut/filecode/content 响应。
type CutFileContent map[string]CutFileMeta

// CutFileMeta 表示单个 PSD 的切图元数据。
type CutFileMeta struct {
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	PSDFrames []PSDFrame `json:"psdFrames"`
}

// PSDFrame 表示 PSD 中单个图层。
type PSDFrame struct {
	Name        string    `json:"N"`
	Type        string    `json:"T"`
	Width       int       `json:"W"`
	Height      int       `json:"H"`
	X           float64   `json:"X"`
	Y           float64   `json:"Y"`
	OutputSize  string    `json:"OS"`
	Bounds      string    `json:"B"`
	File        string    `json:"F"`
	Matrix      []float64 `json:"M"`
	Perspective []Point2D `json:"PS"`
}

// Point2D 表示 PSD 透视点。
type Point2D struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SyncDesignResponse 表示保存设计后的响应。
// 当前真实响应是 200 + empty body，因此这里只保留占位结构。
type SyncDesignResponse struct {
	Ret  int    `json:"ret"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}
