package sheinmanaged

import (
	sheinclient "task-processor/internal/shein/client"
)

type apiFactory struct{}

func newAPIFactory() *apiFactory {
	return &apiFactory{}
}

func (f *apiFactory) BuildBaseClient(storeID int64) (*sheinclient.BaseAPIClient, string) {
	if storeID <= 0 {
		return nil, "未提供 shein_store_id，SHEIN 在线解析未启用"
	}
	return nil, "SHEIN managed runtime 已下线，已降级为离线解析"
}
