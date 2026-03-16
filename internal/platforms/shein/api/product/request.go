// Package product 产品请求数据结构
package product

// ProductRecordRequest 产品记录请求
type ProductRecordRequest struct {
	Language                  string    `json:"language"`
	OnlyCurrentMonthRecommend bool      `json:"only_current_month_recommend"`
	OnlySpmbCopyProduct       bool      `json:"only_spmb_copy_product"`
	QueryTimeOut              bool      `json:"query_time_out"`
	QueryState                *int      `json:"query_state"`
	SearchDiyCustom           bool      `json:"search_diy_custom"`
	SupplierCodeList          *[]string `json:"supplier_code_list"`
	SupplierCodeSearchType    int       `json:"supplier_code_search_type"`
}

// ProductListRequest 产品列表请求
type ProductListRequest struct {
	Language             string `json:"language"`
	OnlyRecommendResell  bool   `json:"only_recommend_resell"`
	OnlySpmbCopyProduct  bool   `json:"only_spmb_copy_product"`
	SearchAbandonProduct bool   `json:"search_abandon_product"`
	SearchIllegal        bool   `json:"search_illegal"`
	SearchLessInventory  bool   `json:"search_less_inventory"`
	ShelfType            string `json:"shelf_type,omitempty"` // ON_SHELF, OFF_SHELF, 可为空
	SortType             int    `json:"sort_type"`
}
