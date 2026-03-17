package attribute

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

// Client 属性相关API实现
type Client struct {
	*client.BaseAPIClient
}

// NewClient 创建新的属性API客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	return &Client{
		BaseAPIClient: baseClient,
	}
}

// GetAttributeTemplates 获取属性模板
func (a *Client) GetAttributeTemplates(categoryID int) (*AttributeTemplateInfo, error) {
	if categoryID <= 0 {
		return nil, fmt.Errorf("categoryID必须大于0")
	}

	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetAttributeTemplatesEndpoint())

	reqBody := struct {
		CategoryID int     `json:"category_id"`
		ForUpdate  bool    `json:"for_update"`
		SPUName    *string `json:"spu_name"`
	}{
		CategoryID: categoryID,
		ForUpdate:  false,
		SPUName:    nil,
	}

	var result struct {
		api.APIResponse
		Info AttributeTemplateInfo `json:"info"`
	}
	if err := a.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return nil, err
	}

	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("获取属性模板失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}

// ValidateCustomAttributeValue 验证自定义属性值
func (a *Client) ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*ValidateAttributeResponse, error) {
	if attributeID <= 0 {
		return nil, fmt.Errorf("attributeID必须大于0")
	}
	if attributeValue == "" {
		return nil, fmt.Errorf("attributeValue不能为空")
	}
	if categoryID <= 0 {
		return nil, fmt.Errorf("categoryID必须大于0")
	}

	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetValidateAttributeEndpoint())

	requestBody := struct {
		AttributeID    int    `json:"attribute_id"`
		AttributeValue string `json:"attribute_value"`
		CategoryID     int    `json:"category_id"`
		ProductTypeID  *int   `json:"product_type_id"`
		SPUName        string `json:"spu_name"`
	}{
		AttributeID:    attributeID,
		AttributeValue: attributeValue,
		CategoryID:     categoryID,
		ProductTypeID:  nil,
		SPUName:        spuName,
	}

	var result struct {
		api.APIResponse
		Info ValidateAttributeResponse `json:"info"`
	}
	if err := a.APIRequest(http.MethodPost, url, requestBody, &result); err != nil {
		return nil, err
	}

	if err := a.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return nil, err
		}
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("验证自定义属性值失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result.Info, nil
}

// AddCustomAttributeValue 添加自定义属性值
func (a *Client) AddCustomAttributeValue(req *AddCustomAttributeValueRequest) (*AddCustomAttributeValueResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("请求参数不能为空")
	}
	if req.CategoryID <= 0 {
		return nil, fmt.Errorf("CategoryID必须大于0")
	}
	if len(req.PreAttributeValueList) == 0 {
		return nil, fmt.Errorf("PreAttributeValueList不能为空")
	}

	url := fmt.Sprintf("%s%s", a.GetBaseURL(), client.GetAddAttributeValueEndpoint())

	var result AddCustomAttributeValueResponse
	if err := a.APIRequest(http.MethodPost, url, req, &result); err != nil {
		return nil, err
	}

	if result.Code != "0" {
		return nil, &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("添加自定义属性值失败: %s", result.Msg),
			URL:        url,
		}
	}

	return &result, nil
}
