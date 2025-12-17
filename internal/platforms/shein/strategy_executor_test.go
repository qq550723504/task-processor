package shein

import (
	"task-processor/internal/common/management/api"
	"task-processor/internal/common/shein/api/product"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuildOffShelfRequest(t *testing.T) {
	// 创建测试数据
	prod := &api.ProductDataDTO{
		PlatformProductID: "p2511247547975",
		Title:             "Test Product",
	}

	mappings := []*SKUMappingData{
		{
			MappingInfo: &MappingInfo{
				SKU: "sp251124754797599194",
			},
			Stock: 10,
		},
	}

	// 创建执行器
	executor := &StrategyExecutor{
		strategy: &api.OperationStrategyDTO{},
	}

	// 构建请求
	request := executor.buildOffShelfRequest(prod, mappings)

	// 验证请求
	assert.NotNil(t, request)
	assert.Equal(t, "p2511247547975", request.SpuName)
	assert.Len(t, request.SkcSiteInfos, 1)

	skcInfo := request.SkcSiteInfos[0]
	assert.Equal(t, 1, skcInfo.BusinessModel)
	assert.Equal(t, "sp251124754797599194", skcInfo.SkcName)
	assert.Len(t, skcInfo.OffSubSites, 1)
	assert.Equal(t, "shein-us", skcInfo.OffSubSites[0].SiteAbbr)
	assert.Equal(t, 1, skcInfo.OffSubSites[0].StoreType)
}

func TestShelfOperateRequest(t *testing.T) {
	// 测试下架请求结构
	offShelfRequest := &product.ShelfOperateRequest{
		SkcSiteInfos: []product.SkcSiteInfo{
			{
				BusinessModel: 1,
				SubSites:      []product.SubSite{},
				OffSubSites: []product.SubSite{
					{
						SiteAbbr:  "shein-us",
						StoreType: 1,
					},
				},
				SkcName: "sp251124754797599194",
			},
		},
		SpuName: "p2511247547975",
	}

	assert.NotNil(t, offShelfRequest)
	assert.Equal(t, "p2511247547975", offShelfRequest.SpuName)
	assert.Len(t, offShelfRequest.SkcSiteInfos, 1)

	// 测试上架请求结构
	onShelfRequest := &product.ShelfOperateRequest{
		SkcSiteInfos: []product.SkcSiteInfo{
			{
				BusinessModel: 1,
				SubSites: []product.SubSite{
					{
						SiteAbbr:  "shein-us",
						StoreType: 1,
					},
				},
				OffSubSites: []product.SubSite{},
				SkcName:     "sp251123411017403946",
			},
		},
		SpuName: "p2511234110174",
	}

	assert.NotNil(t, onShelfRequest)
	assert.Equal(t, "p2511234110174", onShelfRequest.SpuName)
	assert.Len(t, onShelfRequest.SkcSiteInfos, 1)
}

func TestBuildOnShelfRequest(t *testing.T) {
	// 创建测试数据
	prod := &api.ProductDataDTO{
		PlatformProductID: "p2511234110174",
		Title:             "Test Product",
	}

	mappings := []*SKUMappingData{
		{
			MappingInfo: &MappingInfo{
				SKU: "sp251123411017403946",
			},
			Stock: 10,
		},
	}

	// 创建执行器
	executor := &StrategyExecutor{
		strategy: &api.OperationStrategyDTO{},
	}

	// 构建请求
	request := executor.buildOnShelfRequest(prod, mappings)

	// 验证请求
	assert.NotNil(t, request)
	assert.Equal(t, "p2511234110174", request.SpuName)
	assert.Len(t, request.SkcSiteInfos, 1)

	skcInfo := request.SkcSiteInfos[0]
	assert.Equal(t, 1, skcInfo.BusinessModel)
	assert.Equal(t, "sp251123411017403946", skcInfo.SkcName)
	assert.Len(t, skcInfo.SubSites, 1)
	assert.Equal(t, "shein-us", skcInfo.SubSites[0].SiteAbbr)
	assert.Equal(t, 1, skcInfo.SubSites[0].StoreType)
	assert.Len(t, skcInfo.OffSubSites, 0)
}
