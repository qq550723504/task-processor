package management

import (
	"sync"
	"task-processor/common/management/api"
	"time"

	"github.com/sirupsen/logrus"
)

// CategoryRestrictionCache 品类限制缓存
type CategoryRestrictionCache struct {
	clientMgr    *ClientManager
	cache        map[string][]api.CategoryRestrictionInfoRespDTO
	cacheTime    map[string]time.Time
	mutex        sync.RWMutex
	cacheTimeout time.Duration
}

// NewCategoryRestrictionCache 创建新的品类限制缓存
func NewCategoryRestrictionCache(clientMgr *ClientManager) *CategoryRestrictionCache {
	return &CategoryRestrictionCache{
		clientMgr:    clientMgr,
		cache:        make(map[string][]api.CategoryRestrictionInfoRespDTO),
		cacheTime:    make(map[string]time.Time),
		cacheTimeout: 5 * time.Minute, // 默认5分钟缓存超时
	}
}

// GetConfirmedListByPlatform 获取已确认的限制集合（带缓存）
func (c *CategoryRestrictionCache) GetConfirmedListByPlatform(platformName string) ([]api.CategoryRestrictionInfoRespDTO, error) {
	cacheKey := platformName

	// 检查缓存
	c.mutex.RLock()
	if cached, exists := c.cache[cacheKey]; exists {
		if time.Since(c.cacheTime[cacheKey]) < c.cacheTimeout {
			c.mutex.RUnlock()
			logrus.Debugf("从缓存获取品类限制数据: platformName=%s", platformName)
			return cached, nil
		}
	}
	c.mutex.RUnlock()

	// 缓存未命中或已过期，从API获取
	client := c.clientMgr.GetCategoryRestrictionCollectionsClient()
	restrictions, err := client.GetConfirmedListByPlatform(platformName)
	if err != nil {
		return nil, err
	}

	// 更新缓存
	c.mutex.Lock()
	c.cache[cacheKey] = restrictions
	c.cacheTime[cacheKey] = time.Now()
	c.mutex.Unlock()

	logrus.Debugf("从API获取品类限制数据并更新缓存: platformName=%s, count=%d", platformName, len(restrictions))
	return restrictions, nil
}

// SetCacheTimeout 设置缓存超时时间
func (c *CategoryRestrictionCache) SetCacheTimeout(timeout time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cacheTimeout = timeout
}

// ClearCache 清空缓存
func (c *CategoryRestrictionCache) ClearCache() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.cache = make(map[string][]api.CategoryRestrictionInfoRespDTO)
	c.cacheTime = make(map[string]time.Time)
}
