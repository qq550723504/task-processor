package management

import (
	"fmt"
	"net/http"
	"sync"
	"task-processor/internal/infra/clients/management/api"
	"time"

	"golang.org/x/sync/singleflight"
)

// ProductImportMappingAPIClient 产品导入映射API客户端实现
type ProductImportMappingAPIClient struct {
	*ManagementAPIClient
	localDataProvider *LocalDataProvider
}

const productImportMappingCacheTTL = 30 * time.Second

type productImportMappingCacheEntry struct {
	expiresAt time.Time
	mapping   *api.ProductImportMappingRespDTO
	found     bool
}

type productImportMappingBoolCacheEntry struct {
	expiresAt time.Time
	value     bool
}

var productImportMappingCache = struct {
	mu    sync.RWMutex
	items map[string]productImportMappingCacheEntry
	bools map[string]productImportMappingBoolCacheEntry
	group singleflight.Group
}{
	items: make(map[string]productImportMappingCacheEntry),
	bools: make(map[string]productImportMappingBoolCacheEntry),
}

func cloneProductImportMapping(mapping *api.ProductImportMappingRespDTO) *api.ProductImportMappingRespDTO {
	if mapping == nil {
		return nil
	}

	cloned := *mapping
	return &cloned
}

func getProductImportMappingCache(key string) (*api.ProductImportMappingRespDTO, bool, bool) {
	productImportMappingCache.mu.RLock()
	entry, ok := productImportMappingCache.items[key]
	productImportMappingCache.mu.RUnlock()
	if !ok {
		return nil, false, false
	}

	if time.Now().After(entry.expiresAt) {
		productImportMappingCache.mu.Lock()
		delete(productImportMappingCache.items, key)
		productImportMappingCache.mu.Unlock()
		return nil, false, false
	}

	return cloneProductImportMapping(entry.mapping), entry.found, true
}

func setProductImportMappingCache(key string, mapping *api.ProductImportMappingRespDTO, found bool) {
	productImportMappingCache.mu.Lock()
	productImportMappingCache.items[key] = productImportMappingCacheEntry{
		expiresAt: time.Now().Add(productImportMappingCacheTTL),
		mapping:   cloneProductImportMapping(mapping),
		found:     found,
	}
	productImportMappingCache.mu.Unlock()
}

func getProductImportMappingBoolCache(key string) (bool, bool) {
	productImportMappingCache.mu.RLock()
	entry, ok := productImportMappingCache.bools[key]
	productImportMappingCache.mu.RUnlock()
	if !ok {
		return false, false
	}

	if time.Now().After(entry.expiresAt) {
		productImportMappingCache.mu.Lock()
		delete(productImportMappingCache.bools, key)
		productImportMappingCache.mu.Unlock()
		return false, false
	}

	return entry.value, true
}

func setProductImportMappingBoolCache(key string, value bool) {
	productImportMappingCache.mu.Lock()
	productImportMappingCache.bools[key] = productImportMappingBoolCacheEntry{
		expiresAt: time.Now().Add(productImportMappingCacheTTL),
		value:     value,
	}
	productImportMappingCache.mu.Unlock()
}

func clearProductImportMappingCache() {
	productImportMappingCache.mu.Lock()
	productImportMappingCache.items = make(map[string]productImportMappingCacheEntry)
	productImportMappingCache.bools = make(map[string]productImportMappingBoolCacheEntry)
	productImportMappingCache.mu.Unlock()
}

func loadProductImportMapping(
	cacheKey string,
	notFoundErr error,
	load func() (*api.ProductImportMappingRespDTO, error),
) (*api.ProductImportMappingRespDTO, error) {
	if cached, found, ok := getProductImportMappingCache(cacheKey); ok {
		if !found {
			if notFoundErr != nil {
				return nil, notFoundErr
			}
			return nil, nil
		}
		return cached, nil
	}

	value, err, _ := productImportMappingCache.group.Do(cacheKey, func() (any, error) {
		if cached, found, ok := getProductImportMappingCache(cacheKey); ok {
			if !found {
				return nil, nil
			}
			return cached, nil
		}

		mapping, loadErr := load()
		if loadErr != nil {
			return nil, loadErr
		}

		if mapping == nil {
			setProductImportMappingCache(cacheKey, nil, false)
			return nil, nil
		}

		setProductImportMappingCache(cacheKey, mapping, true)
		return cloneProductImportMapping(mapping), nil
	})
	if err != nil {
		return nil, err
	}

	if value == nil {
		if notFoundErr != nil {
			return nil, notFoundErr
		}
		return nil, nil
	}

	mapping, ok := value.(*api.ProductImportMappingRespDTO)
	if !ok {
		return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
	}
	return mapping, nil
}

func loadProductImportMappingBool(cacheKey string, load func() (bool, error)) (bool, error) {
	if cached, ok := getProductImportMappingBoolCache(cacheKey); ok {
		return cached, nil
	}

	value, err, _ := productImportMappingCache.group.Do(cacheKey, func() (any, error) {
		if cached, ok := getProductImportMappingBoolCache(cacheKey); ok {
			return cached, nil
		}

		loaded, loadErr := load()
		if loadErr != nil {
			return false, loadErr
		}

		setProductImportMappingBoolCache(cacheKey, loaded)
		return loaded, nil
	})
	if err != nil {
		return false, err
	}

	loaded, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("检查产品是否存在响应数据类型转换失败")
	}
	return loaded, nil
}

// CreateProductImportMapping 创建产品导入映射关系
func (m *ProductImportMappingAPIClient) CreateProductImportMapping(createReqDTO *api.ProductImportMappingCreateReqDTO) (int64, error) {
	if m.localDataProvider != nil && m.localDataProvider.HasDB() {
		id, err := m.localDataProvider.CreateProductImportMapping(createReqDTO)
		if err == nil {
			clearProductImportMappingCache()
		}
		return id, err
	}
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/create", m.baseURL)

	var result APIResponse
	result.Data = new(int64)

	if err := m.apiRequest(http.MethodPost, url, createReqDTO, &result); err != nil {
		return 0, err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return 0, err
	}

	if result.Data == nil {
		return 0, fmt.Errorf("创建产品导入映射关系响应数据为空")
	}

	id, ok := result.Data.(*int64)
	if !ok {
		return 0, fmt.Errorf("创建产品导入映射关系响应数据类型转换失败")
	}

	clearProductImportMappingCache()
	return *id, nil
}

// GetProductImportMappingByPlatformProductId 通过平台产品ID获取产品导入映射关系
func (m *ProductImportMappingAPIClient) GetProductImportMappingByPlatformProductId(req *api.ProductImportMappingGetReqDTO) (*api.ProductImportMappingRespDTO, error) {
	cacheKey := fmt.Sprintf("platform-product:%s", req.PlatformProductId)
	return loadProductImportMapping(
		cacheKey,
		NewNonRetryableError("产品导入映射关系数据为空: 可能不存在对应的映射关系", nil),
		func() (*api.ProductImportMappingRespDTO, error) {
			if m.localDataProvider != nil && m.localDataProvider.HasDB() {
				mapping, found, err := m.localDataProvider.GetProductImportMappingByPlatformProductID(req.PlatformProductId)
				if err != nil || found {
					return mapping, err
				}
				return nil, nil
			}
			url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/get", m.baseURL)

			params := map[string]any{
				"platformProductId": req.PlatformProductId,
			}

			var result APIResponse
			result.Data = &api.ProductImportMappingRespDTO{}

			if err := m.apiRequest(http.MethodGet, url, params, &result); err != nil {
				return nil, err
			}

			if err := m.ProcessAPIResponse(&result, 0); err != nil {
				return nil, err
			}

			if result.Data == nil {
				return nil, nil
			}

			mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
			if !ok {
				return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
			}

			return mapping, nil
		},
	)
}

// GetProductImportMappingByTaskAndSku 根据任务ID和SKU查询映射关系
func (m *ProductImportMappingAPIClient) GetProductImportMappingByTaskAndSku(importTaskId int64, sku string) (*api.ProductImportMappingRespDTO, error) {
	cacheKey := fmt.Sprintf("task-sku:%d:%s", importTaskId, sku)
	return loadProductImportMapping(cacheKey, nil, func() (*api.ProductImportMappingRespDTO, error) {
		if m.localDataProvider != nil && m.localDataProvider.HasDB() {
			mapping, found, err := m.localDataProvider.GetProductImportMappingByTaskAndSKU(importTaskId, sku)
			if err != nil || found {
				return mapping, err
			}
			return nil, nil
		}
		url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/get-by-task-and-sku?importTaskId=%d&sku=%s",
			m.baseURL, importTaskId, sku)

		var result APIResponse
		result.Data = &api.ProductImportMappingRespDTO{}

		if err := m.apiRequest(http.MethodGet, url, nil, &result); err != nil {
			return nil, err
		}

		if err := m.ProcessAPIResponse(&result, 0); err != nil {
			return nil, err
		}

		if result.Data == nil {
			return nil, nil
		}

		mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
		if !ok {
			return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
		}

		return mapping, nil
	})
}

// UpdateProductImportMapping 更新产品导入映射关系
func (m *ProductImportMappingAPIClient) UpdateProductImportMapping(updateReqDTO *api.ProductImportMappingCreateReqDTO) error {
	if m.localDataProvider != nil && m.localDataProvider.HasDB() {
		ok, err := m.localDataProvider.UpdateProductImportMapping(updateReqDTO)
		if err == nil && ok {
			clearProductImportMappingCache()
		}
		return err
	}
	url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/update", m.baseURL)

	var result APIResponse
	result.Data = new(bool)

	if err := m.apiRequest(http.MethodPost, url, updateReqDTO, &result); err != nil {
		return err
	}

	if err := m.ProcessAPIResponse(&result, 0); err != nil {
		return err
	}

	clearProductImportMappingCache()
	return nil
}

// CheckProductExists 检查产品是否已上架
func (m *ProductImportMappingAPIClient) CheckProductExists(req *api.ProductImportMappingCheckReqDTO) (bool, error) {
	cacheKey := fmt.Sprintf("exists:%d:%s:%s:%s", req.StoreId, req.Platform, req.Region, req.ProductId)
	return loadProductImportMappingBool(cacheKey, func() (bool, error) {
		if m.localDataProvider != nil && m.localDataProvider.HasDB() {
			exists, handled, err := m.localDataProvider.CheckProductExists(req)
			if err != nil || handled {
				return exists, err
			}
			return false, nil
		}
		url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/check-exists?storeId=%d&platform=%s&region=%s&productId=%s",
			m.baseURL, req.StoreId, req.Platform, req.Region, req.ProductId)

		var result APIResponse
		result.Data = new(bool)

		if err := m.apiRequest(http.MethodGet, url, nil, &result); err != nil {
			return false, err
		}

		if err := m.ProcessAPIResponse(&result, 0); err != nil {
			return false, err
		}

		if result.Data == nil {
			return false, fmt.Errorf("检查产品是否存在响应数据为空")
		}

		exists, ok := result.Data.(*bool)
		if !ok {
			return false, fmt.Errorf("检查产品是否存在响应数据类型转换失败")
		}

		return *exists, nil
	})
}

// GetProductImportMappingBySku 通过SKU获取产品导入映射关系
func (m *ProductImportMappingAPIClient) GetProductImportMappingBySku(req *api.ProductImportMappingGetBySkuReqDTO) (*api.ProductImportMappingRespDTO, error) {
	cacheKey := fmt.Sprintf("sku:%d:%s", req.StoreId, req.Sku)
	return loadProductImportMapping(
		cacheKey,
		NewNonRetryableError("产品导入映射关系数据为空: 可能不存在对应的SKU映射关系", nil),
		func() (*api.ProductImportMappingRespDTO, error) {
			if m.localDataProvider != nil && m.localDataProvider.HasDB() {
				mapping, found, err := m.localDataProvider.GetProductImportMappingBySKU(req.Sku, req.StoreId)
				if err != nil || found {
					return mapping, err
				}
				return nil, nil
			}
			url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/get-by-sku", m.baseURL)

			params := map[string]any{
				"sku":     req.Sku,
				"storeId": req.StoreId,
			}

			var result APIResponse
			result.Data = &api.ProductImportMappingRespDTO{}

			if err := m.apiRequest(http.MethodGet, url, params, &result); err != nil {
				return nil, err
			}

			if err := m.ProcessAPIResponse(&result, 0); err != nil {
				return nil, err
			}

			if result.Data == nil {
				return nil, nil
			}

			mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
			if !ok {
				return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
			}

			return mapping, nil
		},
	)
}

// GetProductImportMappingByPlatformProductIdAndStore 通过平台产品ID和店铺ID获取产品导入映射关系
func (m *ProductImportMappingAPIClient) GetProductImportMappingByPlatformProductIdAndStore(req *api.ProductImportMappingGetByPlatformProductIdAndStoreReqDTO) (*api.ProductImportMappingRespDTO, error) {
	cacheKey := fmt.Sprintf("platform-product-store:%s:%d", req.PlatformProductId, req.StoreId)
	return loadProductImportMapping(cacheKey, nil, func() (*api.ProductImportMappingRespDTO, error) {
		if m.localDataProvider != nil && m.localDataProvider.HasDB() {
			mapping, found, err := m.localDataProvider.GetProductImportMappingByPlatformProductIDAndStore(req.PlatformProductId, req.StoreId)
			if err != nil || found {
				return mapping, err
			}
			return nil, nil
		}
		url := fmt.Sprintf("%s/rpc-api/listing/product-import-mapping/get-by-platform-product-id-and-store", m.baseURL)

		params := map[string]any{
			"platformProductId": req.PlatformProductId,
			"storeId":           req.StoreId,
		}

		var result APIResponse
		result.Data = &api.ProductImportMappingRespDTO{}

		if err := m.apiRequest(http.MethodGet, url, params, &result); err != nil {
			return nil, err
		}

		if err := m.ProcessAPIResponse(&result, 0); err != nil {
			return nil, err
		}

		if result.Data == nil {
			return nil, nil
		}

		mapping, ok := result.Data.(*api.ProductImportMappingRespDTO)
		if !ok {
			return nil, fmt.Errorf("产品导入映射关系数据类型转换失败")
		}

		return mapping, nil
	})
}
