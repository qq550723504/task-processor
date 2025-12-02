package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/management/api"
	"time"
)

// RawJsonDataAPIClientImpl 原始JSON数据API客户端实现
type RawJsonDataAPIClientImpl struct {
	*ManagementAPIClientImpl
	dataFreshnessDays int // 数据新鲜度天数
}

// SetDataFreshnessDays 设置数据新鲜度天数
func (m *RawJsonDataAPIClientImpl) SetDataFreshnessDays(days int) {
	m.dataFreshnessDays = days
}

// GetRawJsonData 获取原始JSON数据
// 如果数据不新鲜（超过15天），返回 nil，调用方需要重新抓取
func (m *RawJsonDataAPIClientImpl) GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/raw-json-data/get", m.baseURL)

	var result APIResponse
	result.Data = &api.RawJsonDataRespDTO{}

	// 使用POST请求并将参数作为请求体传递
	err := m.apiRequest(http.MethodPost, url, req, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, fmt.Errorf("原始JSON数据为空")
	}

	// 安全的类型断言
	rawData, ok := result.Data.(*api.RawJsonDataRespDTO)
	if !ok {
		return nil, fmt.Errorf("原始JSON数据类型转换失败")
	}

	// 检查数据新鲜度（使用配置的天数，默认15天）
	freshnessDays := m.dataFreshnessDays
	if freshnessDays <= 0 {
		freshnessDays = 15 // 默认15天
	}
	if !isDataFresh(rawData.CreateTime, freshnessDays) {
		return nil, nil // 数据不新鲜，返回 nil 让调用方重新抓取
	}

	return rawData, nil
}

// isDataFresh 检查数据是否新鲜
// createTime: 创建时间（毫秒时间戳）
// freshnessDays: 新鲜度天数
func isDataFresh(createTime int64, freshnessDays int) bool {
	if createTime == 0 {
		return false
	}

	// 将毫秒时间戳转换为时间对象
	dataTime := time.Unix(createTime/1000, (createTime%1000)*1000000)

	// 计算数据年龄（天数）
	age := time.Since(dataTime)
	ageDays := age.Hours() / 24

	// 判断是否在新鲜度范围内（小于指定天数）
	return ageDays < float64(freshnessDays)
}

// CreateRawJsonData 创建原始JSON数据（提交到服务器缓存）
func (m *RawJsonDataAPIClientImpl) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/raw-json-data/create", m.baseURL)

	var result APIResponse
	var recordID int64
	result.Data = &recordID

	// 使用POST请求并将参数作为请求体传递
	err := m.apiRequest(http.MethodPost, url, req, &result)
	if err != nil {
		return 0, fmt.Errorf("创建原始JSON数据失败: %w", err)
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return 0, fmt.Errorf("处理API响应失败: %w", err)
	}

	// 安全的类型断言
	if idPtr, ok := result.Data.(*int64); ok {
		return *idPtr, nil
	}

	return 0, fmt.Errorf("无法解析返回的记录ID")
}
