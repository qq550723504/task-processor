package api

// SensitiveWordInterface 敏感词管理接口
type SensitiveWordInterface interface {
	// CreateSensitiveWord 添加敏感词
	CreateSensitiveWord(req *CreateSensitiveWordReqDTO) (bool, error)

	// GetAllEnableSensitiveWordList 获取所有启用的敏感词列表
	GetAllEnableSensitiveWordList(language *string) (*[]string, error)

	// ValidateText 验证文本是否包含敏感词
	ValidateText(req *ValidateTextReqDTO) (bool, error)

	// GetSensitiveWords 获取文本中的所有敏感词
	GetSensitiveWords(req *GetSensitiveWordsReqDTO) (*[]string, error)

	// ReplaceSensitiveWords 替换文本中的敏感词
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
