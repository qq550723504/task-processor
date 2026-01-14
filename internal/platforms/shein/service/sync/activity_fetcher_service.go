// Package sync 提供SHEIN平台活动产品获取功能
package sync

import (
	"fmt"
	"task-processor/internal/platforms/shein/api/marketing"
	"task-processor/internal/platforms/shein/repo/client"

	"github.com/sirupsen/logrus"
)

// ActivityFetcher 活动产品获取器
type ActivityFetcher struct {
	logger *logrus.Entry
}

// NewActivityFetcher 创建新的活动产品获取器
func NewActivityFetcher() *ActivityFetcher {
	return &ActivityFetcher{
		logger: logrus.WithField("component", "ActivityFetcher"),
	}
}

// FetchAllActivityProducts 获取所有可报名活动的产品
func (f *ActivityFetcher) FetchAllActivityProducts(apiClient *client.APIClient) ([]marketing.SkcInfo, error) {
	// 分页获取所有活动产品
	var allProducts []marketing.SkcInfo
	pageNum := 1
	pageSize := 100

	for {
		products, hasMore, err := f.fetchActivityProductPage(apiClient, pageNum, pageSize)
		if err != nil {
			return nil, fmt.Errorf("获取活动产品页面失败: %w", err)
		}

		allProducts = append(allProducts, products...)

		if !hasMore {
			break
		}
		pageNum++
	}

	f.logger.WithField("count", len(allProducts)).Info("成功获取所有活动产品")
	return allProducts, nil
}

// fetchActivityProductPage 获取单页活动产品
func (f *ActivityFetcher) fetchActivityProductPage(
	apiClient *client.APIClient,
	pageNum, pageSize int,
) ([]marketing.SkcInfo, bool, error) {
	// TODO: 实现活动产品列表API调用
	// 需要通过 apiClient 调用 SHEIN 的活动产品列表接口
	// req := &marketing.GetAvailableSkcListRequest{
	//     PageNum:  pageNum,
	//     PageSize: pageSize,
	// }
	// resp, err := apiClient.MarketingAPI.GetAvailableSkcList(req)

	f.logger.Debug("获取活动产品列表(待实现)")
	return []marketing.SkcInfo{}, false, nil
}
