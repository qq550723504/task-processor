package impl

import (
	"fmt"
	"net/http"
	"task-processor/common/shein/api"
)

// TranslateAPI 翻译相关API实现
type TranslateAPI struct {
	*BaseAPIClient
}

// TranslateRequest 翻译请求结构体
type TranslateRequest struct {
	CapitalizeEnWords bool            `json:"capitalize_en_words"`
	TranslateList     []TranslateItem `json:"translate_list"`
}

// TranslateItem 翻译项结构体
type TranslateItem struct {
	TargetLanguage string `json:"target_language"`
	Text           string `json:"text"`
	SourceLanguage string `json:"source_language"`
}

// TranslateResponse 翻译响应结构体
type TranslateResponse struct {
	api.APIResponse
	Info TranslateInfo `json:"info"`
}

// TranslateInfo 翻译信息结构体
type TranslateInfo struct {
	Data []TranslationData `json:"data"`
	Meta TranslateMeta     `json:"meta"`
}

// TranslationData 翻译数据结构体
type TranslationData struct {
	TargetLanguage string  `json:"target_language"`
	SourceLanguage string  `json:"source_language"`
	SourceText     string  `json:"source_text"`
	TranslatedText string  `json:"translated_text"`
	ErrorMsg       *string `json:"error_msg"`
	Code           int     `json:"code"`
}

// TranslateMeta 翻译元数据结构体
type TranslateMeta struct {
	Count     int         `json:"count"`
	CustomObj interface{} `json:"customObj"`
}

// NewTranslateAPI 创建新的翻译API实现
func NewTranslateAPI(baseClient *BaseAPIClient) *TranslateAPI {
	return &TranslateAPI{
		BaseAPIClient: baseClient,
	}
}

// Translate 翻译文本
func (t *TranslateAPI) Translate(text string, from, to string) (string, error) {
	// 参数验证
	if text == "" {
		return "", fmt.Errorf("text不能为空")
	}
	if to == "" {
		return "", fmt.Errorf("to不能为空")
	}
	if from == "" {
		from = "auto"
	}

	// 构建URL
	url := fmt.Sprintf("%s%s", t.GetBaseURL(), translateTextEndpoint)

	// 构建请求体
	reqBody := TranslateRequest{
		CapitalizeEnWords: false,
		TranslateList: []TranslateItem{
			{
				TargetLanguage: to,
				Text:           text,
				SourceLanguage: from,
			},
		},
	}

	// 执行请求
	var result TranslateResponse

	if err := t.apiRequest(http.MethodPost, url, reqBody, &result); err != nil {
		return "", err
	}

	// 统一错误处理 - 使用 ProcessAPIResponse 检查认证过期
	if err := t.ProcessAPIResponse(&result.APIResponse, "0"); err != nil {
		// 如果是认证过期错误，直接返回
		if _, isAuthExpired := api.IsAuthenticationExpired(err); isAuthExpired {
			return "", err
		}
		// 其他错误，包装为 APIError
		return "", &api.APIError{
			StatusCode: 0, // 业务错误码
			Message:    fmt.Sprintf("翻译失败: %s", result.Msg),
			URL:        url,
		}
	}

	// 检查翻译结果
	if len(result.Info.Data) == 0 {
		return "", fmt.Errorf("翻译结果为空")
	}

	// 检查单个翻译项是否有错误
	translation := result.Info.Data[0]
	if translation.Code != 0 && translation.ErrorMsg != nil {
		return "", fmt.Errorf("翻译失败: %s", *translation.ErrorMsg)
	}

	return translation.TranslatedText, nil
}
