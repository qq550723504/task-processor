// Package service 提供Amazon产品列表相关服务
package service

import (
	"context"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"

	"task-processor/platforms/amazon/api"
)

// ProductListService Amazon产品列表服务
type ProductListService struct {
	apiClient *api.Client
	logger    *logrus.Entry
}

// NewProductListService 创建产品列表服务
func NewProductListService(apiClient *api.Client) *ProductListService {
	return &ProductListService{
		apiClient: apiClient,
		logger: logrus.WithFields(logrus.Fields{
			"component": "ProductListService",
		}),
	}
}

// ListSellerProducts 获取卖家的所有产品
func (s *ProductListService) ListSellerProducts(ctx context.Context, filters *ProductListFilters) ([]*api.SellerListing, error) {
	s.logger.WithFields(logrus.Fields{
		"filters": filters,
	}).Info("获取卖家产品列表")

	var allListings []*api.SellerListing
	pageToken := ""

	for {
		req := &api.GetSellerListingsRequest{
			MarketplaceID: s.apiClient.GetMarketplaceID(),
			SKUs:          filters.SKUs,
			ProductTypes:  filters.ProductTypes,
			Status:        filters.Status,
			PageSize:      20, // 每页最大数量
			PageToken:     pageToken,
		}

		resp, err := s.apiClient.GetSellerListings(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("获取产品列表失败: %w", err)
		}

		// 转换为指针切片
		for i := range resp.Listings {
			allListings = append(allListings, &resp.Listings[i])
		}

		s.logger.WithFields(logrus.Fields{
			"currentPage": len(resp.Listings),
			"totalSoFar":  len(allListings),
		}).Info("已获取产品页面")

		// 检查是否还有下一页
		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken

		// 避免请求过快
		time.Sleep(100 * time.Millisecond)
	}

	s.logger.WithFields(logrus.Fields{
		"totalProducts": len(allListings),
	}).Info("产品列表获取完成")

	return allListings, nil
}

// SearchProducts 搜索Amazon目录中的产品
func (s *ProductListService) SearchProducts(ctx context.Context, keywords string, filters *SearchFilters) ([]*api.CatalogItem, error) {
	s.logger.WithFields(logrus.Fields{
		"keywords": keywords,
		"filters":  filters,
	}).Info("搜索Amazon产品目录")

	var allItems []*api.CatalogItem
	pageToken := ""

	for {
		req := &api.SearchCatalogRequest{
			Keywords:      keywords,
			ProductTypes:  filters.ProductTypes,
			Brands:        filters.Brands,
			MarketplaceID: s.apiClient.GetMarketplaceID(),
			PageSize:      20, // 每页最大数量
			PageToken:     pageToken,
		}

		resp, err := s.apiClient.SearchCatalog(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("搜索产品失败: %w", err)
		}

		// 转换为指针切片
		for i := range resp.Items {
			allItems = append(allItems, &resp.Items[i])
		}

		s.logger.WithFields(logrus.Fields{
			"currentPage": len(resp.Items),
			"totalSoFar":  len(allItems),
		}).Info("已获取搜索页面")

		// 检查是否还有下一页或达到最大数量限制
		if resp.NextPageToken == "" || (filters.MaxResults > 0 && len(allItems) >= filters.MaxResults) {
			break
		}
		pageToken = resp.NextPageToken

		// 避免请求过快
		time.Sleep(100 * time.Millisecond)
	}

	// 如果设置了最大结果数，截取结果
	if filters.MaxResults > 0 && len(allItems) > filters.MaxResults {
		allItems = allItems[:filters.MaxResults]
	}

	s.logger.WithFields(logrus.Fields{
		"totalProducts": len(allItems),
	}).Info("产品搜索完成")

	return allItems, nil
}

// GetProductDetail 获取产品详情
func (s *ProductListService) GetProductDetail(ctx context.Context, asin string) (*api.CatalogItem, error) {
	s.logger.WithFields(logrus.Fields{
		"asin": asin,
	}).Info("获取产品详情")

	item, err := s.apiClient.GetCatalogItem(ctx, asin, s.apiClient.GetMarketplaceID())
	if err != nil {
		return nil, fmt.Errorf("获取产品详情失败: %w", err)
	}

	s.logger.WithFields(logrus.Fields{
		"asin":  item.ASIN,
		"title": item.Title,
	}).Info("产品详情获取完成")

	return item, nil
}

// GetProductsByStatus 根据状态获取产品
func (s *ProductListService) GetProductsByStatus(ctx context.Context, status string) ([]*api.SellerListing, error) {
	filters := &ProductListFilters{
		Status: []string{status},
	}
	return s.ListSellerProducts(ctx, filters)
}

// GetActiveProducts 获取活跃产品
func (s *ProductListService) GetActiveProducts(ctx context.Context) ([]*api.SellerListing, error) {
	return s.GetProductsByStatus(ctx, "ACTIVE")
}

// GetInactiveProducts 获取非活跃产品
func (s *ProductListService) GetInactiveProducts(ctx context.Context) ([]*api.SellerListing, error) {
	return s.GetProductsByStatus(ctx, "INACTIVE")
}

// GetIncompleteProducts 获取不完整产品
func (s *ProductListService) GetIncompleteProducts(ctx context.Context) ([]*api.SellerListing, error) {
	return s.GetProductsByStatus(ctx, "INCOMPLETE")
}

// ProductListFilters 产品列表过滤器
type ProductListFilters struct {
	SKUs         []string `json:"skus,omitempty"`         // SKU过滤
	ProductTypes []string `json:"productTypes,omitempty"` // 产品类型过滤
	Status       []string `json:"status,omitempty"`       // 状态过滤
}

// SearchFilters 搜索过滤器
type SearchFilters struct {
	ProductTypes []string `json:"productTypes,omitempty"` // 产品类型过滤
	Brands       []string `json:"brands,omitempty"`       // 品牌过滤
	MaxResults   int      `json:"maxResults,omitempty"`   // 最大结果数
}

// ProductSummary 产品摘要信息
type ProductSummary struct {
	TotalProducts      int            `json:"totalProducts"`
	ActiveProducts     int            `json:"activeProducts"`
	InactiveProducts   int            `json:"inactiveProducts"`
	IncompleteProducts int            `json:"incompleteProducts"`
	ProductsByType     map[string]int `json:"productsByType"`
	RecentIssues       []api.Issue    `json:"recentIssues"`
}

// GetProductSummary 获取产品摘要统计
func (s *ProductListService) GetProductSummary(ctx context.Context) (*ProductSummary, error) {
	s.logger.Info("获取产品摘要统计")

	// 获取所有产品
	allProducts, err := s.ListSellerProducts(ctx, &ProductListFilters{})
	if err != nil {
		return nil, fmt.Errorf("获取产品列表失败: %w", err)
	}

	summary := &ProductSummary{
		TotalProducts:  len(allProducts),
		ProductsByType: make(map[string]int),
		RecentIssues:   make([]api.Issue, 0),
	}

	// 统计各种状态的产品数量
	for _, product := range allProducts {
		switch product.Status {
		case "ACTIVE":
			summary.ActiveProducts++
		case "INACTIVE":
			summary.InactiveProducts++
		case "INCOMPLETE":
			summary.IncompleteProducts++
		}

		// 统计产品类型
		if product.ProductType != "" {
			summary.ProductsByType[product.ProductType]++
		}

		// 收集问题
		for _, issue := range product.Issues {
			if issue.Severity == "ERROR" || issue.Severity == "WARNING" {
				summary.RecentIssues = append(summary.RecentIssues, issue)
			}
		}
	}

	s.logger.WithFields(logrus.Fields{
		"total":      summary.TotalProducts,
		"active":     summary.ActiveProducts,
		"inactive":   summary.InactiveProducts,
		"incomplete": summary.IncompleteProducts,
		"issues":     len(summary.RecentIssues),
	}).Info("产品摘要统计完成")

	return summary, nil
}
