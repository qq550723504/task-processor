package attribute

// AttributeAPI 属性相关API接口
type AttributeAPI interface {
	// GetAttributeTemplates 获取属性模板
	GetAttributeTemplates(categoryID int) (*AttributeTemplateInfo, error)

	// ValidateCustomAttributeValue 验证自定义属性值
	ValidateCustomAttributeValue(attributeID int, attributeValue string, categoryID int, spuName string) (*ValidateAttributeResponse, error)

	// AddCustomAttributeValue 添加自定义属性值
	AddCustomAttributeValue(req *AddCustomAttributeValueRequest) (*AddCustomAttributeValueResponse, error)
}

// AttributeValueGroup 属性值组
type AttributeValueGroup struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	MultiName string `json:"multi_name"`
}

// AttributeValue 属性值信息
type AttributeValue struct {
	AttributeValueID           int                   `json:"attribute_value_id"`
	AttributeValue             string                `json:"attribute_value"`
	AttributeValueEn           string                `json:"attribute_value_en"`
	IsCustomAttributeValue     bool                  `json:"is_custom_attribute_value"`
	IsShow                     int                   `json:"is_show"`
	FromCateMis                *string               `json:"from_cate_mis"`
	SupplierID                 int                   `json:"supplier_id"`
	AttributeValueDoc          *string               `json:"attribute_value_doc"`
	AttributeValueDocImageList []any                 `json:"attribute_value_doc_image_list"`
	AttributeValueGroupList    []AttributeValueGroup `json:"attribute_value_group_list"`
}

// AttributeInfo 属性信息
type AttributeInfo struct {
	AttributeID                 int              `json:"attribute_id"`
	AttributeName               string           `json:"attribute_name"`
	AttributeNameEn             string           `json:"attribute_name_en"`
	AttributeRemarkList         []any            `json:"attribute_remark_list"`
	AttributeIsShow             int              `json:"attribute_is_show"`
	AttributeSource             int              `json:"attribute_source"`
	AttributeLabel              int              `json:"attribute_label"`
	AttributeMode               int              `json:"attribute_mode"`
	AttributeInputNum           int              `json:"attribute_input_num"`
	DataDimension               int              `json:"data_dimension"`
	AttributeStatus             int              `json:"attribute_status"`
	AttributeStatusGpc          *string          `json:"attribute_status_gpc"`
	AttributeType               int              `json:"attribute_type"`
	BusinessMode                int              `json:"business_mode"`
	IsSample                    int              `json:"is_sample"`
	SupplierID                  int              `json:"supplier_id"`
	AttributeDoc                *string          `json:"attribute_doc"`
	RuleInfoList                any              `json:"rule_info_list"`
	AttributeDocImageList       []any            `json:"attribute_doc_image_list"`
	AttributeValueInfoList      []AttributeValue `json:"attribute_value_info_list"`
	SiteTitle                   *string          `json:"site_title"`
	SiteURL                     *string          `json:"site_url"`
	CascadeAttributeID          int              `json:"cascade_attribute_id"`
	CascadeAttributeValueIDList *string          `json:"cascade_attribute_value_id_list"`
	SKCScope                    *bool            `json:"skc_scope"`
	SortOrder                   int              `json:"sort_order"`
	SourceSystemIDList          []int            `json:"source_system_id_list"`
}

// AttributeTemplate 属性模板
type AttributeTemplate struct {
	ProductTypeID  int             `json:"product_type_id"`
	BusinessMode   int             `json:"business_mode"`
	AttributeInfos []AttributeInfo `json:"attribute_infos"`
	AttributeID    []int           `json:"attribute_id"`
}

// AttributeTemplateInfo 属性模板信息
type AttributeTemplateInfo struct {
	Data []AttributeTemplate `json:"data"`
	Meta struct {
		Count     int `json:"count"`
		CustomObj any `json:"customObj"`
	} `json:"meta"`
}

// ValidateAttributeResponse 验证属性值响应
type ValidateAttributeResponse struct {
	Data struct {
		AttributeID              int `json:"attribute_id"`
		PreAttributeValueID      int `json:"pre_attribute_value_id"`
		AttributeValueNameMultis []struct {
			Language                string `json:"language"`
			AttributeValueNameMulti string `json:"attribute_value_name_multi"`
			WarningType             int    `json:"warning_type"`
		} `json:"attribute_value_name_multis"`
	} `json:"data"`
}

// AttributeValueNameMulti 多语言属性值名称
type AttributeValueNameMulti struct {
	Language           string `json:"language"`
	AttributeValueName string `json:"attribute_value_name_multi"`
	WarningType        int    `json:"warning_type"`
}

// PreAttributeValue 预定义属性值
type PreAttributeValue struct {
	AttributeID              int                       `json:"attribute_id"`
	PreAttributeValueID      int64                     `json:"pre_attribute_value_id"`
	AttributeValue           string                    `json:"attribute_value"`
	AttributeValueNameMultis []AttributeValueNameMulti `json:"attribute_value_name_multis"`
}

// AddCustomAttributeValueRequest 添加自定义属性值请求
type AddCustomAttributeValueRequest struct {
	CategoryID            int                 `json:"category_id"`
	PreAttributeValueList []PreAttributeValue `json:"pre_attribute_value_list"`
	ProductTypeID         *int                `json:"product_type_id"`
}

// CustomAttributeRelation 自定义属性关系
type CustomAttributeRelation struct {
	PreAttributeValueID int64 `json:"pre_attribute_value_id"`
	AttributeValueID    int64 `json:"attribute_value_id"`
}

// AddCustomAttributeValueResponse 添加自定义属性值响应
type AddCustomAttributeValueResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data struct {
			CustomAttributeRelation []CustomAttributeRelation `json:"custom_attribute_relation"`
		} `json:"data"`
	} `json:"info"`
	BBL *string `json:"bbl"`
}
