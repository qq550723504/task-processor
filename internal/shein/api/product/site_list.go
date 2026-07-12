package product

import (
	"fmt"
	"net/http"
	"task-processor/internal/shein/api"
	"task-processor/internal/shein/client"
)

type SiteListSubSite struct {
	SiteName   string `json:"site_name"`
	SiteAbbr   string `json:"site_abbr"`
	SiteStatus int    `json:"site_status"`
	StoreType  int    `json:"store_type"`
	Currency   string `json:"currency"`
}
type SiteListGroup struct {
	MainSite     string            `json:"main_site"`
	MainSiteName string            `json:"main_site_name"`
	SubSiteList  []SiteListSubSite `json:"sub_site_list"`
}

func (m *productManager) querySiteList() ([]SiteListGroup, error) {
	url := fmt.Sprintf("%s%s", m.baseClient.GetBaseURL(), client.GetSiteListEndpoint())
	var result struct {
		api.APIResponse
		Info struct {
			Data []SiteListGroup `json:"data"`
		} `json:"info"`
	}
	if err := m.baseClient.APIRequest(http.MethodPost, url, struct{}{}, &result); err != nil {
		return nil, err
	}
	if err := m.errorHandler.ProcessAPIResponse(&result.APIResponse, "0", url, "获取商品站点列表失败"); err != nil {
		return nil, err
	}
	return result.Info.Data, nil
}
