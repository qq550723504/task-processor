package template

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/sds/client"
)

// Service 封装 SDS 模板相关请求。
type Service struct {
	client *client.Client
}

// NewService 创建模板服务。
func NewService(c *client.Client) *Service {
	return &Service{client: c}
}

// List 获取模板列表。
// SDS 的真实响应结构尚未固定，因此调用方自行传入 result。
func (s *Service) List(ctx context.Context, req ListRequest, result any) error {
	query := make(map[string]string)
	for key, value := range req.ExtraQuery {
		query[key] = value
	}

	params := req.Params
	if params.Keyword != "" {
		query["keyword"] = params.Keyword
	}
	if params.Page > 0 {
		query["page"] = strconv.Itoa(params.Page)
	}
	if params.Size > 0 {
		query["size"] = strconv.Itoa(params.Size)
	}
	if params.CategoryID != "" {
		query["categoryId"] = params.CategoryID
	}
	if params.Sort != "" {
		query["sort"] = params.Sort
	}
	if params.SortField != "" {
		query["sortField"] = params.SortField
	}
	if params.SortType != "" {
		query["sortType"] = params.SortType
	}
	if params.MemberLevel != "" {
		query["memberLevel"] = params.MemberLevel
	}
	if params.ProductSupplyChain != "" {
		query["productSupplyChainParam"] = params.ProductSupplyChain
	}
	if params.PreciseSearch != "" {
		query["preciseSearch"] = params.PreciseSearch
	}
	if params.ShipmentArea != "" {
		query["shipmentArea"] = params.ShipmentArea
	}
	if params.OverseasArea != "" {
		query["overseasArea"] = params.OverseasArea
	}
	if params.SideActiveID != "" {
		query["sideActiveId"] = params.SideActiveID
	}
	if params.IsOverseas != "" {
		query["isOverseas"] = params.IsOverseas
	}
	if params.BestStatus != nil {
		query["bestStatus"] = strconv.Itoa(*params.BestStatus)
	}
	if params.HotSellStatus != nil {
		query["hotSellStatus"] = strconv.Itoa(*params.HotSellStatus)
	}
	if params.OnSaleStatus != nil {
		query["onSaleStatus"] = strconv.Itoa(*params.OnSaleStatus)
	}
	if params.NewStatus != nil {
		query["newStatus"] = strconv.Itoa(*params.NewStatus)
	}
	if params.PublicStatus != nil {
		query["publicStatus"] = strconv.Itoa(*params.PublicStatus)
	}
	if params.Timestamp == 0 {
		params.Timestamp = time.Now().UnixMilli()
	}
	query["t"] = strconv.FormatInt(params.Timestamp, 10)

	_, err := s.client.Do(ctx, "GET", s.client.Config().Endpoints.TemplateListPath, query, nil, result)
	return err
}

// ListProducts 获取真实产品分页数据。
func (s *Service) ListProducts(ctx context.Context, params ListParams) (*ListResponse, error) {
	result := new(ListResponse)
	if err := s.List(ctx, ListRequest{Params: params}, result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetOptionGroups 获取筛选项分组。
func (s *Service) GetOptionGroups(ctx context.Context, params OptionGroupParams) (*OptionGroups, error) {
	query := make(map[string]string)
	if params.Size > 0 {
		query["size"] = strconv.Itoa(params.Size)
	}
	if params.Page > 0 {
		query["page"] = strconv.Itoa(params.Page)
	}
	query["preciseSearch"] = strconv.Itoa(params.PreciseSearch)
	if params.ShipmentArea != "" {
		query["shipmentArea"] = params.ShipmentArea
	}
	if params.OverseasArea != "" {
		query["overseasArea"] = params.OverseasArea
	}
	if params.Timestamp == 0 {
		params.Timestamp = time.Now().UnixMilli()
	}
	query["t"] = strconv.FormatInt(params.Timestamp, 10)

	result := new(OptionGroups)
	_, err := s.client.Do(ctx, "GET", s.client.Config().Endpoints.TemplateGroupsPath, query, nil, result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// Detail 获取模板详情。
func (s *Service) Detail(ctx context.Context, req DetailRequest, result any) error {
	if req.ProductID == "" {
		return fmt.Errorf("productID cannot be empty")
	}

	query := make(map[string]string)
	for key, value := range req.ExtraQuery {
		query[key] = value
	}

	path := fmt.Sprintf(s.client.Config().Endpoints.TemplateDetailPath, req.ProductID)

	_, err := s.client.Do(ctx, "GET", path, query, nil, result)
	return err
}

// GetProduct 获取产品详情。
func (s *Service) GetProduct(ctx context.Context, productID string) (*ProductDetail, error) {
	result := new(ProductDetail)
	if err := s.Detail(ctx, DetailRequest{ProductID: productID}, result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetCycle 获取产品生产周期。
func (s *Service) GetCycle(ctx context.Context, productID string) (*CycleInfo, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, fmt.Errorf("productID cannot be empty")
	}

	result := new(CycleInfo)
	path := fmt.Sprintf(s.client.Config().Endpoints.TemplateCyclePath, productID)
	_, err := s.client.Do(ctx, "GET", path, nil, nil, result)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetRecommendations 获取产品推荐列表。
func (s *Service) GetRecommendations(ctx context.Context, productID string) ([]ProductSummary, error) {
	if strings.TrimSpace(productID) == "" {
		return nil, fmt.Errorf("productID cannot be empty")
	}

	var result []ProductSummary
	path := fmt.Sprintf(s.client.Config().Endpoints.TemplateRecommend, productID)
	_, err := s.client.Do(ctx, "GET", path, nil, nil, &result)
	if err != nil {
		return nil, err
	}

	return result, nil
}
