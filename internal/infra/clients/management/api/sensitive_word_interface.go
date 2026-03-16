package api

// SensitiveWord 敏感词管理接口
type SensitiveWord interface {
	CreateSensitiveWord(req *CreateSensitiveWordReqDTO) (bool, error)
	GetAllEnableSensitiveWordList(language *string) (*[]string, error)
	ValidateText(req *ValidateTextReqDTO) (bool, error)
	GetSensitiveWords(req *GetSensitiveWordsReqDTO) (*[]string, error)
	ReplaceSensitiveWords(req *ReplaceSensitiveWordsReqDTO) (string, error)
}

// CreateSensitiveWordReqDTO 添加敏感词请求DTO
type CreateSensitiveWordReqDTO struct {
	Word     string  `json:"word"`
	Language string  `json:"language"`
	Level    *int    `json:"level,omitempty"`
	Status   *int    `json:"status,omitempty"`
	Remark   *string `json:"remark,omitempty"`
}

// ValidateTextReqDTO 验证文本请求DTO
type ValidateTextReqDTO struct {
	Text     string  `json:"text"`
	Language *string `json:"language,omitempty"`
}

// GetSensitiveWordsReqDTO 获取敏感词请求DTO
type GetSensitiveWordsReqDTO struct {
	Text     string  `json:"text"`
	Language *string `json:"language,omitempty"`
}

// ReplaceSensitiveWordsReqDTO 替换敏感词请求DTO
type ReplaceSensitiveWordsReqDTO struct {
	Text        string  `json:"text"`
	Language    *string `json:"language,omitempty"`
	ReplaceText *string `json:"replaceText,omitempty"`
}
