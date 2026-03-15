package translate

import (
	"fmt"
	"net/http"
	"task-processor/internal/platforms/shein/api"
	"task-processor/internal/platforms/shein/client"
)

// Request 翻译请求
type Request struct {
	CapitalizeEnWords bool   `json:"capitalize_en_words"`
	TranslateList     []Item `json:"translate_list"`
}

// Item 翻译项
type Item struct {
	TargetLanguage string `json:"target_language"`
	Text           string `json:"text"`
	SourceLanguage string `json:"source_language"`
}

// Response 翻译响应
type Response struct {
	api.APIResponse
	Info Info `json:"info"`
}

// Info 翻译信息
type Info struct {
	Data []TranslationData `json:"data"`
	Meta Meta              `json:"meta"`
}

// TranslationData 翻译数据
type TranslationData struct {
	TargetLanguage string  `json:"target_language"`
	SourceLanguage string  `json:"source_language"`
	SourceText     string  `json:"source_text"`
	TranslatedText string  `json:"translated_text"`
	ErrorMsg       *string `json:"error_msg"`
	Code           int     `json:"code"`
}

// Meta 翻译元数据
type Meta struct {
	Count     int         `json:"count"`
	CustomObj interface{} `json:"customObj"`
}

// Client 翻译相关API实现
type Client struct {
	*client.BaseAPIClient
}

// NewClient 创建新的翻译API客户端
func NewClient(baseClient *client.BaseAPIClient) *Client {
	return &Client{BaseAPIClient: baseClient}
}

// Translate 翻译文本
func (t *Client) Translate(text string, from, to string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("text不能为空")
	}
	if to == "" {
		return "", fmt.Errorf("to不能为空")
	}
	if from == "" {
		from = "auto"
	}

	url := fmt.Sprintf("%s%s", t.GetBaseURL(), client.GetTranslateTextEndpoint())

	reqBody := Request{
		CapitalizeEnWords: false,
		TranslateList: []Item{
			{TargetLanguage: to, Text: text, SourceLanguage: from},
		},
	}

	var result Response
	if err := t.APIRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return "", err
	}

	if err := t.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return "", err
		}
		return "", &api.APIError{
			StatusCode: 0,
			Message:    fmt.Sprintf("翻译失败: %s", result.Msg),
			URL:        url,
		}
	}

	if len(result.Info.Data) == 0 {
		return "", fmt.Errorf("翻译结果为空")
	}

	translation := result.Info.Data[0]
	if translation.Code != 0 && translation.ErrorMsg != nil {
		return "", fmt.Errorf("翻译失败: %s", *translation.ErrorMsg)
	}

	return translation.TranslatedText, nil
}
