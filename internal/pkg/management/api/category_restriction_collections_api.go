package api

// CategoryRestrictionCollectionsInterface 品类限制集合管理接口
type CategoryRestrictionCollectionsInterface interface {
	// CreateCategoryRestrictionCollections 添加品类限制集合
	CreateCategoryRestrictionCollections(req *CategoryRestrictionCollectionsCreateReqDTO) (int64, error)

	// GetListByCategoryAndPlatform 获取指定品类和平台的限制集合
	GetListByPlatform(platformName string) ([]CategoryRestrictionInfoRespDTO, error)

	// GetConfirmedListByCategoryAndPlatform 获取已确认的限制集合
	GetConfirmedListByPlatform(platformName string) ([]CategoryRestrictionInfoRespDTO, error)

	// IsAttributeRestricted 检查属性是否被限制
	IsAttributeRestricted(categoryId int, platformName string, attributeId int) (bool, error)

	// UpdateCategoryRestrictionCollectionsStatus 更新品类限制集合状态
	UpdateCategoryRestrictionCollectionsStatus(id int64, isConfirmed bool, isAutoApplied bool) (bool, error)
}

// CategoryRestrictionCollectionsCreateReqDTO 创建品类限制集合请求DTO
type CategoryRestrictionCollectionsCreateReqDTO struct {
	CategoryId             int     `json:"categoryId"`
	PlatformName           string  `json:"platformName"`
	ForbiddenAttributeId   int     `json:"forbiddenAttributeId"`
	ForbiddenAttributeName string  `json:"forbiddenAttributeName"`
	DefaultAttributeId     int     `json:"defaultAttributeId"`
	DefaultAttributeName   string  `json:"defaultAttributeName"`
	OccurrenceCount        int     `json:"occurrenceCount"`
	ConfidenceScore        float64 `json:"confidenceScore"`
	IsConfirmed            bool    `json:"isConfirmed"`
	IsAutoApplied          bool    `json:"isAutoApplied"`
}

// CategoryRestrictionInfoRespDTO 品类限制信息响应DTO
type CategoryRestrictionInfoRespDTO struct {
	ID                     int     `json:"id"`
	CategoryId             int     `json:"categoryId"`
	PlatformName           string  `json:"platformName"`
	ForbiddenAttributeId   int     `json:"forbiddenAttributeId"`
	ForbiddenAttributeName string  `json:"forbiddenAttributeName"`
	DefaultAttributeId     int     `json:"defaultAttributeId"`
	DefaultAttributeName   string  `json:"defaultAttributeName"`
	OccurrenceCount        int     `json:"occurrenceCount"`
	ConfidenceScore        float64 `json:"confidenceScore"`
	IsConfirmed            bool    `json:"isConfirmed"`
	IsAutoApplied          bool    `json:"isAutoApplied"`
}
