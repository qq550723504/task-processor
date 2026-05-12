package management

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"task-processor/internal/infra/clients/management/api"
	"task-processor/internal/pkg/types"
	"time"
)

// RawJsonDataAPIClient 原始JSON数据API客户端实现
type RawJsonDataAPIClient struct {
	*ManagementAPIClient
	dataFreshnessDays int
	localDataProvider *LocalDataProvider
}

// SetDataFreshnessDays 设置数据新鲜度天数
func (m *RawJsonDataAPIClient) SetDataFreshnessDays(days int) {
	m.dataFreshnessDays = days
}

// GetRawJsonData 获取原始JSON数据，数据不新鲜时返回 nil
func (m *RawJsonDataAPIClient) GetRawJsonData(req *api.RawJsonDataReqDTO) (*api.RawJsonDataRespDTO, error) {
	if m.localDataProvider != nil {
		if rawData, err := m.localDataProvider.GetRawJSONData(req); err != nil || rawData != nil {
			if err != nil || rawData == nil {
				return rawData, err
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
	}
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
func isDataFresh(createTime, updateTime types.FlexibleTime, freshnessDays int) bool {
	latestTime := createTime.Time
	if updateTime.After(latestTime) {
		latestTime = updateTime.Time
	}
	if latestTime.IsZero() {
		return false
	}
	ageDays := time.Since(latestTime).Hours() / 24
	return ageDays < float64(freshnessDays)
}

// CreateRawJsonData 创建原始JSON数据
func (m *RawJsonDataAPIClient) CreateRawJsonData(req *api.RawJsonDataCreateReqDTO) (int64, error) {
	if m.localDataProvider != nil {
		if id, err := m.localDataProvider.CreateRawJSONData(req); err != nil || id != 0 {
			return id, err
		}
	}
	url := fmt.Sprintf("%s/rpc-api/listing/raw-json-data/create", m.baseURL)
	var result struct {
		Code    int             `json:"code"`
		Message string          `json:"message"`
		Data    json.RawMessage `json:"data,omitempty"`
	}

	if err := m.apiRequest(http.MethodPost, url, req, &result); err != nil {
		return 0, fmt.Errorf("创建原始JSON数据失败: %w", err)
	}
	if result.Code != 0 {
		return 0, &ManagementAPIError{
			Code:    result.Code,
			Message: result.Message,
		}
	}

	id, err := parseInt64Result(result.Data)
	if err != nil {
		return 0, fmt.Errorf("创建原始JSON数据失败: %w", err)
	}
	return id, nil
}

func parseInt64Result(data json.RawMessage) (int64, error) {
	if len(data) == 0 || string(data) == "null" {
		return 0, fmt.Errorf("响应数据为空")
	}

	var numeric int64
	if err := json.Unmarshal(data, &numeric); err == nil {
		return numeric, nil
	}

	var text string
	if err := json.Unmarshal(data, &text); err != nil {
		return 0, fmt.Errorf("解析响应数据失败: %w", err)
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("响应数据为空字符串")
	}

	numeric, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("解析字符串响应数据失败: %w", err)
	}
	return numeric, nil
}
