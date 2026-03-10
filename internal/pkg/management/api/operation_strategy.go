package api

import (
	"fmt"
	"io"
	"net/http"
	"task-processor/internal/pkg/jsonutil"
	"task-processor/internal/pkg/types"
	"task-processor/internal/pkg/utils"

	"github.com/sirupsen/logrus"
)

// OperationStrategyAPI 自动化运营策略 API 接口
type OperationStrategyAPI interface {
	// GetOperationStrategyByStoreId 根据店铺ID获取策略
	GetOperationStrategyByStoreId(storeId int64) (*OperationStrategyDTO, error)
}

// OperationStrategyDTO 自动化运营策略 DTO
type OperationStrategyDTO struct {
	ID                    int64   `json:"id"`
	TenantID              int64   `json:"tenantId"`
	StoreID               int64   `json:"storeId"`
	Name                  string  `json:"name"`
	Platform              string  `json:"platform"`
	Status                int16   `json:"status"` // 0=启用, 1=禁用
	StockChangeThreshold  int     `json:"stockChangeThreshold"`
	StockChangeAction     string  `json:"stockChangeAction"`
	OutOfStockAction      string  `json:"outOfStockAction"`
	MinProfitRate         float64 `json:"minProfitRate"`
	LowProfitAction       string  `json:"lowProfitAction"`
	PriceUpdateMultiplier float64 `json:"priceUpdateMultiplier"`
	StockUpdateRatio      float64 `json:"stockUpdateRatio"`

	// 活动相关配置
	ActivityEnabled       bool    `json:"activityEnabled"`       // 是否启用活动功能
	ActivityType          string  `json:"activityType"`          // 活动类型: PROMOTION(促销活动), TIME_LIMITED(限时折扣), MIXED(混合模式)
	ActivityDiscountRate  float64 `json:"activityDiscountRate"`  // 活动折扣率（0-1之间，如0.1表示打9折）
	ActivityStockRatio    float64 `json:"activityStockRatio"`    // 活动库存比例（0-1之间，如0.5表示50%库存用于活动）
	PromotionRatio        float64 `json:"promotionRatio"`        // 促销活动比例（仅MIXED模式，0-1之间，如0.5表示50%产品用于促销，剩余用于限时折扣）
	ActivityMinProfitRate float64 `json:"activityMinProfitRate"` // 活动最低利润率（0-1之间，如0.15表示15%利润率）
	ActivityPriceMode     string  `json:"activityPriceMode"`     // 活动定价模式（DISCOUNT:按折扣率, PROFIT:按最低利润率）

	// 限时折扣专属配置
	TimeLimitedDiscountRate      float64 `json:"timeLimitedDiscountRate"`      // 限时折扣-折扣率（0-1之间，如0.4表示打6折，即40%off）
	TimeLimitedMinProfitRate     float64 `json:"timeLimitedMinProfitRate"`     // 限时折扣-最低利润率（0-1之间，如0.15表示15%利润率，优先级高于通用配置）
	TimeLimitedPriceMode         string  `json:"timeLimitedPriceMode"`         // 限时折扣-定价模式（DISCOUNT:按折扣率, PROFIT:按最低利润率，优先级高于通用配置）
	TimeLimitedUserLimit         bool    `json:"timeLimitedUserLimit"`         // 限时折扣-是否启用单用户限购（true:限购, false:不限购）
	TimeLimitedUserLimitNum      int     `json:"timeLimitedUserLimitNum"`      // 限时折扣-单用户限购数量（当UserLimit=true时生效）
	TimeLimitedStockLimit        bool    `json:"timeLimitedStockLimit"`        // 限时折扣-是否启用活动库存限量（true:限量, false:不限量）
	TimeLimitedStockLimitPercent int     `json:"timeLimitedStockLimitPercent"` // 限时折扣-活动库存限量百分比（当StockLimit=true时生效，1-100）

	// 价格调整配置
	FixedPriceAdjustment float64 `json:"fixedPriceAdjustment"` // 固定价格调整值（在最低售价基础上增加的固定金额）

	// 价格变化处理策略
	PriceIncreaseThreshold float64 `json:"priceIncreaseThreshold"` // 价格上涨阈值（百分比，如10.0表示10%）
	PriceDecreaseThreshold float64 `json:"priceDecreaseThreshold"` // 价格下降阈值（百分比，如5.0表示5%）
	PriceIncreaseAction    string  `json:"priceIncreaseAction"`    // 价格上涨时的处理动作（SET_ZERO_STOCK:设置库存为0, DELIST:下架产品, NONE:不处理）
	PriceDecreaseAction    string  `json:"priceDecreaseAction"`    // 价格下降时的处理动作（RESTORE_STOCK:恢复库存, UPDATE_STOCK:更新库存, NONE:不处理）
	RestoreStockAmount     int     `json:"restoreStockAmount"`     // 恢复库存时的数量（当PriceDecreaseAction=RESTORE_STOCK时生效）

	Remark     string               `json:"remark"`
	CreateTime types.FlexibleString `json:"createTime"` // 支持字符串或数字
}

// IsEnabled 判断策略是否启用
func (s *OperationStrategyDTO) IsEnabled() bool {
	return s.Status == 0
}

// OperationStrategyClient 自动化运营策略客户端
type OperationStrategyClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewOperationStrategyClient 创建自动化运营策略客户端
func NewOperationStrategyClient(baseURL string) *OperationStrategyClient {
	return &OperationStrategyClient{
		baseURL:    baseURL,
		httpClient: utils.CreateSimpleHTTPClient(),
	}
}

// GetOperationStrategyByStoreId 根据店铺ID获取策略
func (c *OperationStrategyClient) GetOperationStrategyByStoreId(storeId int64) (*OperationStrategyDTO, error) {
	url := fmt.Sprintf("%s/admin-api/listing/operation-strategy/get-by-store?storeId=%d", c.baseURL, storeId)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logrus.Warnf("获取运营策略失败，状态码: %d, 响应: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("获取运营策略失败，状态码: %d", resp.StatusCode)
	}

	var result struct {
		Code int                   `json:"code"`
		Data *OperationStrategyDTO `json:"data"`
		Msg  string                `json:"msg"`
	}

	if err := jsonutil.UnmarshalBytes(body, &result, "解析响应失败"); err != nil {
		return nil, err
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("获取运营策略失败: %s", result.Msg)
	}

	return result.Data, nil
}
