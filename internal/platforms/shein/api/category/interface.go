package category

// CategoryAPI 分类相关API接口
type CategoryAPI interface {
	// GetCategory 获取分类信息
	GetCategory(categoryID int) (*CategoryInfo, error)

	// GetCategoryTree 列出所有分类
	GetCategoryTree() (*CategoryTreeResponse, error)
}

// CategoryInfo 产品类别信息
type CategoryInfo struct {
	CategoryID             int     `json:"category_id"`
	ProductTypeID          int     `json:"product_type_id"`
	LevelOneCategoryID     int     `json:"level_one_category_id"`
	LevelOneCategoryName   string  `json:"level_one_category_name"`
	LevelTwoCategoryID     int     `json:"level_two_category_id"`
	LevelTwoCategoryName   string  `json:"level_two_category_name"`
	LevelThreeCategoryID   int     `json:"level_three_category_id"`
	LevelThreeCategoryName string  `json:"level_three_category_name"`
	LevelFourCategoryID    *int    `json:"level_four_category_id"`
	LevelFourCategoryName  *string `json:"level_four_category_name"`
	CustomizeCategory      bool    `json:"customize_category"`
}

// CategoryTreeResponse 类别树响应
type CategoryTreeResponse struct {
	Data []CategoryTreeNode `json:"data"`
	Meta struct {
		Count     int         `json:"count"`
		CustomObj any `json:"customObj"`
	} `json:"meta"`
}

// CategoryTreeNode 类别树节点
type CategoryTreeNode struct {
	CategoryID       int                `json:"category_id"`
	ProductTypeID    int                `json:"product_type_id"`
	ParentCategoryID int                `json:"parent_category_id"`
	CategoryName     string             `json:"category_name"`
	LastCategory     bool               `json:"last_category"`
	Children         []CategoryTreeNode `json:"children"`
}
