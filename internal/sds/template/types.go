package template

// ListRequest 表示模板列表请求参数。
type ListRequest struct {
	Params     ListParams
	ExtraQuery map[string]string
}

// DetailRequest 表示模板详情请求参数。
type DetailRequest struct {
	ProductID  string
	ExtraQuery map[string]string
}
