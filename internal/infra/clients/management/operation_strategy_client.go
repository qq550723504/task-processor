package management

import (
	"task-processor/internal/core/logger"
	"fmt"
	"net/http"

	"task-processor/internal/infra/clients/management/api"

	"github.com/sirupsen/logrus"
)

// OperationStrategyClient 运营策略客户端实现
type OperationStrategyClient struct {
	*ManagementAPIClient
}

// GetOperationStrategyByStoreId 根据店铺ID获取策略
func (c *OperationStrategyClient) GetOperationStrategyByStoreId(storeId int64) (*api.OperationStrategyDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/operation-strategy/get-by-store?storeId=%d", c.baseURL, storeId)

	var result APIResponse
	result.Data = &api.OperationStrategyDTO{}

	if err := c.apiRequest(http.MethodGet, url, nil, &result); err != nil {
		logrus.WithError(err).Warn("获取运营策略失败")
		return nil, nil
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		logrus.WithError(err).Warn("处理运营策略响应失败")
		return nil, nil
	}

	if result.Data == nil {
		logger.GetGlobalLogger("infra/clients").Debug("店铺未配置运营策略")
		return nil, nil
	}

	strategy, ok := result.Data.(*api.OperationStrategyDTO)
	if !ok {
		logger.GetGlobalLogger("infra/clients").Error("运营策略数据类型转换失败")
		return nil, nil
	}

	return strategy, nil
}
