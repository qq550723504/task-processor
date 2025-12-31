package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"task-processor/internal/utils"

	"github.com/sirupsen/logrus"
)

// OperationStrategyAPI 自动化运营策略 API 接口
type OperationStrategyAPI interface {
	// GetOperationStrategyByStoreId 根据店铺ID获取策略
	GetOperationStrategyByStoreId(storeId int64) (*OperationStrategyDTO, error)
}

// OperationStrategyDTO 自动化运营策略 DTO
type OperationStrategyDTO struct {
	ID                    int64          `json:"id"`
	TenantID              int64          `json:"tenantId"`
	StoreID               int64          `json:"storeId"`
	Name                  string         `json:"name"`
	Platform              string         `json:"platform"`
	Status                int16          `json:"status"` // 0=启用, 1=禁用
	StockChangeThreshold  int            `json:"stockChangeThreshold"`
	StockChangeAction     string         `json:"stockChangeAction"`
	OutOfStockAction      string         `json:"outOfStockAction"`
	MinProfitRate         float64        `json:"minProfitRate"`
	LowProfitAction       string         `json:"lowProfitAction"`
	PriceUpdateMultiplier float64        `json:"priceUpdateMultiplier"`
	StockUpdateRatio      float64        `json:"stockUpdateRatio"`
	Remark                string         `json:"remark"`
	CreateTime            FlexibleString `json:"createTime"` // 支持字符串或数字
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

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Code != 0 {
		return nil, fmt.Errorf("获取运营策略失败: %s", result.Msg)
	}

	return result.Data, nil
}
