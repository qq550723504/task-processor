package product

import (
	"fmt"
	"net/http"

	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

// NameLengthConfigItem describes the maximum product-name length for one language.
type NameLengthConfigItem struct {
	Language  string `json:"language"`
	MaxLength int    `json:"max_length"`
}

func (m *productManager) queryProductNameLengthConfig(categoryID int) ([]NameLengthConfigItem, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetProductNameLengthConfigEndpoint())
	var result struct {
		api.APIResponse
		Info []NameLengthConfigItem `json:"info"`
	}
	body := struct {
		CategoryID int `json:"category_id"`
	}{CategoryID: categoryID}
	if err := m.baseClient.APIRequest(http.MethodPost, url, body, &result); err != nil {
		return nil, err
	}
	if err := m.errorHandler.ProcessAPIResponse(&result.APIResponse, "0", url, "获取产品名称长度配置失败"); err != nil {
		return nil, err
	}
	return result.Info, nil
}
