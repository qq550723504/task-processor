package model

// Merchant rules are returned by official authenticated read endpoints. Missing
// flags stay unknown; callers cannot infer publish permission or requiredness.
type Category struct {
	ID            int64      `json:"category_id"`
	ProductTypeID int64      `json:"product_type_id"`
	ParentID      int64      `json:"parent_category_id"`
	Name          string     `json:"category_name"`
	Leaf          *bool      `json:"last_category"`
	Children      []Category `json:"children"`
}
type AttributeOption struct {
	ID         int64  `json:"attribute_value_id"`
	Name       string `json:"attribute_value"`
	Show       *int   `json:"is_show"`
	Custom     *int   `json:"is_custom_attribute_value"`
	SupplierID int64  `json:"supplier_id"`
}
type AttributeInputRule struct {
	ID       int64  `json:"id"`
	Operator *int   `json:"condition_operator"`
	Type     *int   `json:"condition_type"`
	Value    string `json:"value"`
}
type Attribute struct {
	ID                int64                `json:"attribute_id"`
	Name              string               `json:"attribute_name"`
	Show              *int                 `json:"attribute_is_show"`
	Type              *int                 `json:"attribute_type"`
	MainLabel         *int                 `json:"attribute_label"`
	Mode              *int                 `json:"attribute_mode"`
	MaximumSelections *int                 `json:"attribute_input_num"`
	Status            *int                 `json:"attribute_status"`
	Dimension         *int                 `json:"data_dimension"`
	Options           []AttributeOption    `json:"value_info_list"`
	Rules             []AttributeInputRule `json:"rule_info_list"`
}
type AttributeTemplate struct {
	ProductTypeID       int64       `json:"product_type_id"`
	MainAttributeStatus *int        `json:"main_attribute_status"`
	Attributes          []Attribute `json:"attribute_infos"`
}
type LinkedRulesRequest struct {
	Groups []LinkedRuleGroup `json:"get_linked_rule_req_list"`
}
type LinkedRuleGroup struct {
	ID            string           `json:"group_id"`
	CategoryID    int64            `json:"category_id"`
	ProductTypeID int64            `json:"product_type_id"`
	Attributes    []AttributeValue `json:"attribute_list"`
}
type LinkedAttributeRule struct {
	AttributeID     int64   `json:"attribute_id"`
	Values          []int64 `json:"attribute_value_list"`
	PrefilledValues []int64 `json:"attribute_value_pre_fill_list"`
}
type LinkedRules struct {
	GroupID    string                `json:"group_id"`
	Attributes []LinkedAttributeRule `json:"link_rule_attribute_list"`
}
type Brand struct {
	Code string `json:"brand_code"`
	Name string `json:"brand_name"`
}
type PublishPermission struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}
