package models

// CategoryDisclaimRequest 分类免责声明请求
type CategoryDisclaimRequest struct {
	CateID int `json:"cate_id"`
}

// CategoryDisclaimResponse 分类免责声明响应
type CategoryDisclaimResponse struct {
	Success   bool                   `json:"success"`
	ErrorCode int                    `json:"error_code"`
	Result    CategoryDisclaimResult `json:"result"`
}

// CategoryDisclaimResult 分类免责声明结果
type CategoryDisclaimResult struct {
	DisclaimerDTO DisclaimerDTO `json:"disclaimer_dto"`
}

// DisclaimerDTO 免责声明数据传输对象
type DisclaimerDTO struct {
	PromptList []string `json:"prompt_list"`
}

// CategoryRecommendRequest 分类推荐请求
type CategoryRecommendRequest struct {
	GoodsName string `json:"goods_name"`
}

// CategoryRecommendResponse 分类推荐响应
type CategoryRecommendResponse struct {
	Success bool                    `json:"success"`
	Result  CategoryRecommendResult `json:"result"`
}

// CategoryRecommendResult 分类推荐结果
type CategoryRecommendResult struct {
	CategoryTreeList []Category `json:"category_tree_list"`
}

// Category 分类信息
type Category struct {
	CatID        int      `json:"cat_id"`
	Cate1ID      int      `json:"cate1_id"`
	Cate1Name    string   `json:"cate1_name"`
	Cate2ID      int      `json:"cate2_id"`
	Cate2Name    string   `json:"cate2_name"`
	Cate3ID      int      `json:"cate3_id"`
	Cate3Name    string   `json:"cate3_name"`
	Cate4ID      *int     `json:"cate4_id"`
	Cate4Name    *string  `json:"cate4_name"`
	Cate5ID      *int     `json:"cate5_id"`
	Cate5Name    *string  `json:"cate5_name"`
	Cate6ID      *int     `json:"cate6_id"`
	Cate6Name    *string  `json:"cate6_name"`
	Cate7ID      *int     `json:"cate7_id"`
	Cate7Name    *string  `json:"cate7_name"`
	Cate8ID      *int     `json:"cate8_id"`
	Cate8Name    *string  `json:"cate8_name"`
	Cate9ID      *int     `json:"cate9_id"`
	Cate9Name    *string  `json:"cate9_name"`
	Cate10ID     *int     `json:"cate10_id"`
	Cate10Name   *string  `json:"cate10_name"`
	CateNameList []string `json:"cate_name_list"`
	CateType     int      `json:"cate_type"`
	Level        int      `json:"level"`
}
