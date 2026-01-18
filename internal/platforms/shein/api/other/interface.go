package other

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// FlexibleString can unmarshal both string and number values from JSON
type FlexibleString string

// UnmarshalJSON implements json.Unmarshaler interface
func (fs *FlexibleString) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as string first
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*fs = FlexibleString(s)
		return nil
	}

	// If that fails, try as number
	var n json.Number
	if err := json.Unmarshal(data, &n); err == nil {
		*fs = FlexibleString(n.String())
		return nil
	}

	// If both fail, try as int64
	var i int64
	if err := json.Unmarshal(data, &i); err == nil {
		*fs = FlexibleString(strconv.FormatInt(i, 10))
		return nil
	}

	// If both fail, try as float64
	var f float64
	if err := json.Unmarshal(data, &f); err == nil {
		*fs = FlexibleString(fmt.Sprintf("%.0f", f))
		return nil
	}

	return fmt.Errorf("cannot unmarshal %s into FlexibleString", string(data))
}

// String returns the string representation
func (fs FlexibleString) String() string {
	return string(fs)
}

type OtherAPI interface {
	BatchCheckOnWay(spuNameList []string) (*BatchCheckOnWayResponse, error)
	GetUser(uuid int64) (*UserInfo, error)
	GetSupplierOperateInfo() (*SupplierOperateInfoResponse, error)
	GetSpuLimitCount() (*SpuLimitCountInfo, error)
	QueryShelfQuota() (*ShelfQuotaResponse, error)
}

// SpuLimitCountResponse 查询发品额度响应结构
type SpuLimitCountResponse struct {
	Code string                   `json:"code"`
	Msg  string                   `json:"msg"`
	Info SpuLimitCountInfoWrapper `json:"info"`
	Bbl  *string                  `json:"bbl"`
}

// SpuLimitCountInfoWrapper 发品额度信息包装器
type SpuLimitCountInfoWrapper struct {
	Data SpuLimitCountInfo `json:"data"`
}

// SpuLimitCountInfo 发品额度信息
type SpuLimitCountInfo struct {
	// 额度可用状态 (1: 可用, 0: 不可用)
	AbleStatus int `json:"able_status"`
	// 当前可用额度
	QuotaAvailable int `json:"quota_available"`
	// 已使用额度
	QuotaUsed int `json:"quota_used"`
	// 剩余额度
	QuotaRemain int `json:"quota_remain"`
	// 是否显示 (1: 显示, 0: 不显示)
	IsShow int `json:"is_show"`
	// 总额度
	QuotaAvailableTotal int `json:"quota_available_total"`
	// 跨月恢复数量
	SpanMonthRecoveryCount int `json:"span_month_recovery_count"`
}

type SupplierOperateInfoResponse struct {
	Code string              `json:"code"`
	Msg  string              `json:"msg"`
	Info SupplierOperateInfo `json:"info"`
	BBL  *string             `json:"bbl"`
}

// SupplierOperateInfo SSO供应商操作信息
type SupplierOperateInfo struct {
	SupplierID            int64           `json:"supplierId"`
	StoreID               int64           `json:"storeId"`
	StoreTitle            *string         `json:"storeTitle"`
	StoreLogo             string          `json:"storeLogo"`
	CooperationModelCode  string          `json:"cooperationModelCode"`
	CooperationModelTitle string          `json:"cooperationModelTitle"`
	CurrentLevel          string          `json:"currentLevel"`
	UserName              string          `json:"userName"`
	UlpName               *string         `json:"ulpName"`
	UlpEnName             *string         `json:"ulpEnName"`
	UlpEmplid             *string         `json:"ulpEmplid"`
	IsUlpLogin            int             `json:"isUlpLogin"`
	IsChildren            int             `json:"isChildren"`
	RoleList              []string        `json:"roleList"`
	OperationType         *FlexibleString `json:"operationType"`
	OperationContact      *string         `json:"operationContact"`
	OperationContactEn    *string         `json:"operationContactEn"`
	OperationContactEnDpt *string         `json:"operationContactEnDpt"`
	IsWxBind              int             `json:"isWxBind"`
	JoinWechat            int             `json:"joinWechat"`
	IsShowOperationInfo   int             `json:"isShowOperationInfo"`
	OperationGroupName    *string         `json:"operationGroupName"`
	IsSimplePlatform      bool            `json:"isSimplePlatform"`
	IsShowWecomQrCode     bool            `json:"isShowWecomQrCode"`
}

type BatchCheckOnWayResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info []struct {
		SpuName    string `json:"spu_name"`
		SkcName    string `json:"skc_name"`
		DocumentSn string `json:"document_sn"`
	} `json:"info"`
	BBL interface{} `json:"bbl"`
}

type GetUserResponse struct {
	Code string    `json:"code"`
	Msg  string    `json:"msg"`
	Info *UserInfo `json:"info"`
	Bbl  *string   `json:"bbl"`
}

// UserInfo 用户信息
type UserInfo struct {
	UserName         string  `json:"userName"`         // 用户名
	UserID           int64   `json:"userId"`           // 用户ID
	SupplierID       int64   `json:"supplierId"`       // 供应商ID
	UlpName          *string `json:"ulpName"`          // ULP名称
	UlpEnName        *string `json:"ulpEnName"`        // ULP英文名称
	UlpEmplid        *string `json:"ulpEmplid"`        // ULP员工ID
	IsUlpLogin       int     `json:"isUlpLogin"`       // 是否ULP登录
	Timezone         string  `json:"timezone"`         // 时区
	TimezoneName     string  `json:"timezoneName"`     // 时区名称
	SwitchNewMenu    int     `json:"switchNewMenu"`    // 新菜单开关
	SupplierUserName string  `json:"supplierUserName"` // 供应商用户名
	SsoTopNav        int     `json:"ssoTopNav"`        // SSO顶部导航
	SsoHost          string  `json:"ssoHost"`          // SSO主机地址
	CategoryID       int64   `json:"categoryId"`       // 分类ID
	CategoryOutID    int64   `json:"categoryOutId"`    // 外部分类ID
	SupplierSource   int     `json:"supplierSource"`   // 供应商来源
	ExternalID       int64   `json:"externalId"`       // 外部ID
	StoreCode        int64   `json:"storeCode"`        // 店铺代码
	UtcTimezone      string  `json:"utcTimezone"`      // UTC时区
	MerchantCode     string  `json:"merchantCode"`     // 商户代码
	Lv1CategoryID    int64   `json:"lv1CategoryId"`    // 一级分类ID
	MainUserName     string  `json:"mainUserName"`     // 主用户名
	MainUserID       int64   `json:"mainUserId"`       // 主用户ID
	AreaTimezone     string  `json:"areaTimezone"`     // 区域时区
	Lv1CategoryName  string  `json:"lv1CategoryName"`  // 一级分类名称
	Lv2CategoryName  string  `json:"lv2CategoryName"`  // 二级分类名称
}

// ShelfQuotaResponse 商品上架配额查询响应结构
type ShelfQuotaResponse struct {
	Code string         `json:"code"`
	Msg  string         `json:"msg"`
	Info ShelfQuotaInfo `json:"info"`
	Bbl  *string        `json:"bbl"`
}

// ShelfQuotaInfo 商品上架配额信息
type ShelfQuotaInfo struct {
	// 是否需要配额检查
	Need bool `json:"need"`
	// 剩余配额数量
	RemainCount int `json:"remain_count"`
	// 总配额数量
	TotalQuotaCount int `json:"total_quota_count"`
	// 已上架商品数量
	OnShelfCount int `json:"on_shelf_count"`
}
