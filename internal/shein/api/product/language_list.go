package product

import (
	"fmt"
	"net/http"

	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

// LanguageListItem describes one product language supported by the current store.
type LanguageListItem struct {
	LanguageAbbr string `json:"language_abbr"`
	LanguageName string `json:"language_name"`
	InputMode    int    `json:"input_mode"`
}

func (m *productManager) queryLanguageList() ([]LanguageListItem, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetLanguageListEndpoint())
	var result struct {
		api.APIResponse
		Info struct {
			Data []LanguageListItem `json:"data"`
		} `json:"info"`
	}
	if err := m.baseClient.APIRequest(http.MethodPost, url, struct{}{}, &result); err != nil {
		return nil, err
	}
	if err := m.errorHandler.ProcessAPIResponse(&result.APIResponse, "0", url, "获取产品语言列表失败"); err != nil {
		return nil, err
	}
	return result.Info.Data, nil
}
