package management

import (
	"fmt"
	"net/http"
	"task-processor/internal/infra/clients/management/api"
	"time"
)

// RawJsonDataAPIClient 原始JSON数据API客户端实现
type RawJsonDataAPIClient struct {
	*ManagementAPIClient
	dataFreshnessDays int
}

// SetDataFreshnessDays 设置数据新鲜度天数
func (m *RawJsonDataAPIClient) SetDataFreshnessDays(days int) {
	m.dataFreshnessDays = days
}

// GetRawJsonData 获取原始JSON数据，数据不新鲜时返回 nil
func (m *RawJsonDataAPIClient) GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/raw-json-data/get", m.baseURL)

	var result APIResponse
	result.Data = &api.RawJsonDataRespDTO{}

	if err := m.apiRequest(http.MethodPost, url, req, &result); err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	if result.Data == nil {
		return nil, fmt.Errorf("原始JSON数据为空")
	}

	rawData, ok := result.Data.(*api.RawJsonDataRespDTO)
	if !ok {
		return nil, fmt.Errorf("原始JSON数据类型转换失败")
	}

	freshnessDays := m.dataFreshnessDays
	if freshnessDays <= 0 {
		freshnessDays = 15
	}
	if !isDataFresh(rawData.CreateTime, rawData.UpdateTime, freshnessDays) {
		return nil, nil
	}

	return rawData, nil
}

// isDataFresh 检查数据是否在新鲜度范围内
func isDataFresh(createTime, updateTime int64, freshnessDays int) bool {
	latestTime := createTime
	if updateTime > latestTime {
		latestTime = updateTime
	}
	if latestTime == 0 {
		return false
	}
	dataTime := time.Unix(latestTime/1000, (latestTime%1000)*1000000)
	ageDays := time.Since(dataTime).Hours() / 24
	return ageDays < float64(freshnessDays)
}

// CreateRawJsonData 创建原始JSON数据
func (m *RawJsonDataAPIClient) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/raw-json-data/create", m.baseURL)

	var result APIResponse
	var recordID int64
	result.Data = &recordID

	if err := m.apiRequest(http.MethodPost, url, req, &result); err != nil {
		return 0, fmt.Errorf("创建原始JSON数据失败: %w", err)
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return 0, fmt.Errorf("处理API响应失败: %w", err)
	}

	if idPtr, ok := result.Data.(*int64); ok {
		return *idPtr, nil
	}

	return 0, fmt.Errorf("无法解析返回的记录ID")
}
