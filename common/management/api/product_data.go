// Package api 产品数据API接口定义
package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// ProductDataAPI 产品数据API接口定义
type ProductDataAPI interface {
	// CreateOrUpdate 创建或更新单个产品数据
	CreateOrUpdate(product *ProductDataDTO) error

	// BatchCreateOrUpdate 批量创建或更新产品数据
	BatchCreateOrUpdate(products []*ProductDataDTO) error

	// ListByStore 查询店铺的所有产品数据
	ListByStore(platform string, tenantID, storeID int64, shelfStatus *int) ([]*ProductDataDTO, error)
}

// FlexibleString 灵活的字符串类型，可以接收字符串或数字
type FlexibleString string

// UnmarshalJSON 自定义 JSON 反序列化，支持字符串和数字
func (fs *FlexibleString) UnmarshalJSON(data []byte) error {
	// 尝试作为字符串解析
	var str string
	if err := json.Unmarshal(data, &str); err == nil {
		*fs = FlexibleString(str)
		return nil
	}

	// 尝试作为数字解析
	var num float64
	if err := json.Unmarshal(data, &num); err == nil {
		*fs = FlexibleString(fmt.Sprintf("%.2f", num))
		return nil
	}

	// 尝试作为整数解析
	var intNum int64
	if err := json.Unmarshal(data, &intNum); err == nil {
		*fs = FlexibleString(strconv.FormatInt(intNum, 10))
		return nil
	}

	return fmt.Errorf("无法解析为字符串或数字")
}

// String 转换为字符串
func (fs FlexibleString) String() string {
	return string(fs)
}

// ProductDataDTO 产品数据传输对象
type ProductDataDTO struct {
	// 基础字段
	ID              int64          `json:"id"`
	Source          string         `json:"source"`
	ImportTaskID    int64          `json:"import_task_id"`
	StoreID         int64          `json:"store_id"`
	Platform        string         `json:"platform"`
	CategoryID      int64          `json:"category_id"`
	Region          string         `json:"region"`
	ParentProductID string         `json:"parent_product_id"`
	ProductID       string         `json:"product_id"`
	Title           string         `json:"title"`
	Description     string         `json:"description"`
	OriginalPrice   FlexibleString `json:"original_price"`
	SpecialPrice    FlexibleString `json:"special_price"`
	PriceCurrency   string         `json:"price_currency"`
	Stock           FlexibleString `json:"stock"`
	Brand           string         `json:"brand"`
	Category        string         `json:"category"`
	MainImageURL    string         `json:"main_image_url"`
	ImageURLs       string         `json:"image_urls"`
	Attributes      string         `json:"attributes"`
	SourceURL       string         `json:"source_url"`
	Status          int16          `json:"status"`
	RawJSONDataID   int64          `json:"raw_json_data_id"`

	// 多平台扩展字段
	PlatformProductID string     `json:"platform_product_id"`
	PlatformStatus    string     `json:"platform_status"`
	ShelfStatus       int        `json:"shelf_status"`
	PublishTime       *time.Time `json:"publish_time"`
	ShelfTime         *time.Time `json:"shelf_time"`
	LastSyncTime      *time.Time `json:"last_sync_time"`
	PlatformData      string     `json:"platform_data"`

	// 租户字段
	TenantID   int64      `json:"tenant_id"`
	CreateTime *time.Time `json:"create_time"`
	UpdateTime *time.Time `json:"update_time"`
	Creator    string     `json:"creator"`
	Updater    string     `json:"updater"`
	Deleted    bool       `json:"deleted"`
}

// ShelfStatus 上架状态枚举
const (
	ShelfStatusPending   = 0 // 待上架
	ShelfStatusReviewing = 1 // 审核中
	ShelfStatusOnShelf   = 2 // 已上架
	ShelfStatusOffShelf  = 3 // 已下架
	ShelfStatusRejected  = 4 // 审核拒绝
	ShelfStatusDeleted   = 5 // 已删除
)
