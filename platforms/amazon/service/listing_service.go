// Package service 提供Amazon平台业务逻辑
package service

import (
	"context"
	"fmt"
	"task-processor/platforms/amazon/api"

	"github.com/sirupsen/logrus"
)

// ListingService listing业务服务
type ListingService struct {
	apiClient *api.Client
	logger    *logrus.Entry
}

// NewListingService 创建listing服务
func NewListingService(apiClient *api.Client) *ListingService {
	return &ListingService{
		apiClient: apiClient,
		logger: logrus.WithFields(logrus.Fields{
			"service": "ListingService",
		}),
	}
}

// CreateListing 创建产品listing
func (s *ListingService) CreateListing(ctx context.Context, req *api.ListingRequest) (*api.ListingResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"sku": req.SKU,
	}).Info("创建产品listing")

	// 验证请求
	if err := s.validateListingRequest(req); err != nil {
		return nil, fmt.Errorf("请求验证失败: %w", err)
	}

	// 调用API
	resp, err := s.apiClient.CreateListing(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("API调用失败: %w", err)
	}

	return resp, nil
}

// UpdateListing 更新产品listing
func (s *ListingService) UpdateListing(ctx context.Context, req *api.ListingRequest) (*api.ListingResponse, error) {
	s.logger.WithFields(logrus.Fields{
		"sku": req.SKU,
	}).Info("更新产品listing")

	return s.apiClient.UpdateListing(ctx, req)
}

// DeleteListing 删除产品listing
func (s *ListingService) DeleteListing(ctx context.Context, sku string) error {
	s.logger.WithFields(logrus.Fields{
		"sku": sku,
	}).Info("删除产品listing")

	return s.apiClient.DeleteListing(ctx, sku)
}

// validateListingRequest 验证listing请求
func (s *ListingService) validateListingRequest(req *api.ListingRequest) error {
	if req.SKU == "" {
		return fmt.Errorf("SKU不能为空")
	}

	if req.ProductType == "" {
		return fmt.Errorf("产品类型不能为空")
	}

	return nil
}
