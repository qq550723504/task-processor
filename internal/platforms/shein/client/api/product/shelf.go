// Package product 产品下架相关数据结构
package product

// ShelfOperateRequest 产品上下架请求
type ShelfOperateRequest struct {
	SkcSiteInfos []SkcSiteInfo `json:"skc_site_infos"`
	SpuName      string        `json:"spu_name"`
}

// SkcSiteInfo SKC站点信息
type SkcSiteInfo struct {
	BusinessModel int       `json:"business_model"`
	SubSites      []SubSite `json:"sub_sites"`     // 上架站点列表
	OffSubSites   []SubSite `json:"off_sub_sites"` // 下架站点列表
	SkcName       string    `json:"skc_name"`
}

// SubSite 子站点信息
type SubSite struct {
	SiteAbbr  string `json:"site_abbr"`
	StoreType int    `json:"store_type"`
}

// ShelfOperateResponse 产品上下架响应
type ShelfOperateResponse struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
	Info struct {
		Data []ShelfOperateResult `json:"data"`
		Meta struct {
			Count     int         `json:"count"`
			CustomObj interface{} `json:"customObj"`
		} `json:"meta"`
	} `json:"info"`
	BBL interface{} `json:"bbl"`
}

// ShelfOperateResult 上下架操作结果
type ShelfOperateResult struct {
	SpuName  string `json:"spu_name"`
	SaleName string `json:"sale_name"`
	SkcName  string `json:"skc_name"`
	Filtered bool   `json:"filtered"`
	Msg      string `json:"msg"`
}
