package impl

import (
	"fmt"
	"net/http"
	"task-processor/internal/common/management/api"
)

// SensitiveWordAPIClientImpl 敏感词API客户端实现
type SensitiveWordAPIClientImpl struct {
	*ManagementAPIClientImpl
}

// CreateSensitiveWord 添加敏感词
func (m *SensitiveWordAPIClientImpl) CreateSensitiveWord(req *api.CreateSensitiveWordReqDTO) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/sensitive-word/create", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"word": req.Word,
	}

	if req.Language != "" {
		params["language"] = req.Language
	}

	if req.Level != nil {
		params["level"] = *req.Level
	}

	if req.Status != nil {
		params["status"] = *req.Status
	}

	if req.Remark != nil {
		params["remark"] = *req.Remark
	}

	var result APIResponse
	err := m.apiRequest(http.MethodPost, url, params, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	return true, nil
}

// GetAllEnableSensitiveWordList 获取所有启用的敏感词列表
func (m *SensitiveWordAPIClientImpl) GetAllEnableSensitiveWordList(language *string) (*[]string, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/sensitive-word/list-all-enable", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{}

	if language != nil {
		params["language"] = *language
	}

	var result APIResponse
	result.Data = &[]string{}

	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return nil, fmt.Errorf("敏感词列表数据为空")
	}

	// 安全的类型断言
	words, ok := result.Data.(*[]string)
	if !ok {
		return nil, fmt.Errorf("敏感词列表数据类型转换失败")
	}

	return words, nil
}

// ValidateText 验证文本是否包含敏感词
func (m *SensitiveWordAPIClientImpl) ValidateText(req *api.ValidateTextReqDTO) (bool, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/sensitive-word/validate-text", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"text": req.Text,
	}

	if req.Language != nil {
		params["language"] = *req.Language
	}

	var result APIResponse
	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return false, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return false, err
	}

	return true, nil
}

// GetSensitiveWords 获取文本中的所有敏感词
func (m *SensitiveWordAPIClientImpl) GetSensitiveWords(req *api.GetSensitiveWordsReqDTO) (*[]string, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/sensitive-word/get-sensitive-words", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"text": req.Text,
	}

	if req.Language != nil {
		params["language"] = *req.Language
	}

	var result APIResponse
	result.Data = &[]string{}

	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return nil, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return nil, err
	}

	// 检查 Data 是否为 nil
	if result.Data == nil {
		return nil, fmt.Errorf("获取敏感词响应数据为空")
	}

	// 安全的类型断言
	sensitiveWords, ok := result.Data.(*[]string)
	if !ok {
		return nil, fmt.Errorf("获取敏感词响应数据类型转换失败")
	}

	return sensitiveWords, nil
}

// ReplaceSensitiveWords 替换文本中的敏感词
func (m *SensitiveWordAPIClientImpl) ReplaceSensitiveWords(req *api.ReplaceSensitiveWordsReqDTO) (string, error) {
	url := fmt.Sprintf("%s/rpc-api/listing/sensitive-word/replace-sensitive-words", m.baseURL)

	// 构建查询参数
	params := map[string]interface{}{
		"text": req.Text,
	}

	if req.Language != nil {
		params["language"] = *req.Language
	}

	if req.ReplaceText != nil {
		params["replaceText"] = *req.ReplaceText
	}

	var result APIResponse
	err := m.apiRequest(http.MethodGet, url, params, &result)
	if err != nil {
		return "", err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return "", err
	}

	return result.Data.(string), nil
}
