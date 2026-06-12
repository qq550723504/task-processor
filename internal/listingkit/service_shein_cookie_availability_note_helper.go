package listingkit

import (
	"context"
	"fmt"
)

func (s *service) resolveSheinCookieAvailabilityNote(ctx context.Context, task *Task) string {
	if s == nil || task == nil || task.Result == nil || task.Result.Shein == nil {
		return ""
	}
	if resolveSheinStoreCatalog(s) == nil || resolveSheinAPIClientFactory(s) == nil {
		return ""
	}

	apiClient, _, err := s.newSheinAPIClient(ctx, task)
	if err != nil {
		return fmt.Sprintf("SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：%v", err)
	}
	if apiClient.HasCookies() {
		return ""
	}
	if err := apiClient.ForceRefreshCookies(); err != nil {
		return fmt.Sprintf("SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：%v", err)
	}
	if !apiClient.HasCookies() {
		return "SHEIN 店铺 cookie 不可用，在线类目、属性和销售属性解析受阻：刷新后仍未获取到有效 cookie"
	}
	return ""
}
