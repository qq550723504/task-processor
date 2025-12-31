package impl

import (
	"fmt"
	"net/http"

	"task-processor/internal/pkg/management/api"

	"github.com/sirupsen/logrus"
)

// OperationStrategyClientImpl 运营策略客户端实现
type OperationStrategyClientImpl struct {
	*ManagementAPIClientImpl
}

// GetOperationStrategyByStoreId 根据店铺ID获取策略
func (c *OperationStrategyClientImpl) GetOperationStrategyByStoreId(storeId int64) (*api.OperationStrategyDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/operation-strategy/get-by-store?storeId=%d", c.baseURL, storeId)

	var result APIResponse
	result.Data = &api.OperationStrategyDTO{}

	err := c.apiRequest(http.MethodGet, url, nil, &result)
	if err != nil {
		logrus.WithError(err).Warn("获取运营策略失败")
		return nil, nil // 返回 nil 而不是错误，允许继续执行
	}

	if err := c.ProcessAPIResponse(&result, 0); err != nil {
		logrus.WithError(err).Warn("处理运营策略响应失败")
		return nil, nil // 返回 nil 而不是错误，允许继续执行
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		logrus.Debug("店铺未配置运营策略")
		return nil, nil
	}

	// 安全的类型断言
	strategy, ok := result.Data.(*api.OperationStrategyDTO)
	if !ok {
		logrus.Error("运营策略数据类型转换失败")
		return nil, nil
	}

	return strategy, nil
}
