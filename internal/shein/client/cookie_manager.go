// Package client 提供SHEIN平台的Cookie管理功能
package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/pkg/jsonx"
	"time"

	"task-processor/internal/core/logger"

	"github.com/sirupsen/logrus"
)

// CookieManager SHEIN Cookie管理器（参考TEMU实现）
type CookieManager struct {
	storeID          int64
	managementClient *management.ClientManager
	logger           *logrus.Entry
	resolvedTenantID int64
}

// NewCookieManager 创建Cookie管理器
func NewCookieManager(storeID int64, managementClient *management.ClientManager) *CookieManager {
	logger := logger.GetGlobalLogger("SheinCookieManager").WithField("storeID", storeID)

	return &CookieManager{
		storeID:          storeID,
		managementClient: managementClient,
		logger:           logger,
	}
}

// LoadCookies 从管理系统加载Cookie
func (cm *CookieManager) LoadCookies() ([]*http.Cookie, error) {
	cm.logger.WithField("storeID", cm.storeID).Debug("尝试从管理系统加载Cookie")

	if cm.managementClient != nil {
		cookieStr, tenantID, err := cm.loadCookieJSONFromRedis()
		if err != nil {
			cm.logger.WithError(err).Warn("从 login Redis 读取 SHEIN Cookie 失败，将回退到 management store api")
		} else if strings.TrimSpace(cookieStr) != "" {
			cm.resolvedTenantID = tenantID
			cookies, parseErr := cm.parseCookieString(cookieStr)
			if parseErr != nil {
				cm.logger.WithError(parseErr).Error("解析 Redis Cookie 字符串失败")
				return nil, fmt.Errorf("解析 Redis Cookie 字符串失败: %w", parseErr)
			}
			return cookies, nil
		}
	}

	// 检查管理系统客户端是否可用
	if cm.managementClient == nil {
		cm.logger.Error("管理系统客户端为空")
		return nil, fmt.Errorf("管理系统客户端为空")
	}

	// 从管理系统获取Cookie字符串
	storeClient := cm.managementClient.GetStoreClient()
	if storeClient == nil {
		return nil, fmt.Errorf("店铺客户端未初始化")
	}

	cookieStr, err := storeClient.GetStoreCookie(cm.storeID)
	if err != nil {
		// 检查是否是认证错误
		if strings.Contains(err.Error(), "访问令牌为空") || strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "403") {
			return nil, fmt.Errorf("认证失败，无法获取Cookie: %w", err)
		}
		return nil, fmt.Errorf("从管理系统获取Cookie失败: %w", err)
	}

	if cookieStr == "" {
		cm.logger.WithField("storeID", cm.storeID).Info("未找到Cookie数据")
		return nil, nil
	}

	// 解析Cookie字符串
	cookies, err := cm.parseCookieString(cookieStr)
	if err != nil {
		cm.logger.WithError(err).Error("解析Cookie字符串失败")
		return nil, fmt.Errorf("解析Cookie字符串失败: %w", err)
	}

	// 添加调试日志
	if cm.logger.Logger.IsLevelEnabled(logrus.DebugLevel) {
		cm.logger.WithFields(logrus.Fields{
			"cookieStrLength": len(cookieStr),
			"parsedCookies":   len(cookies),
		}).Debug("Cookie解析结果")

		for i, cookie := range cookies {
			cm.logger.WithFields(logrus.Fields{
				"index":       i,
				"name":        cookie.Name,
				"valueLength": len(cookie.Value),
				"domain":      cookie.Domain,
			}).Debug("解析的Cookie详情")
		}
	}

	return cookies, nil
}

// CookieData JSON格式的Cookie数据结构
type CookieData struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HttpOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite"`
}

// parseCookieString 解析Cookie字符串为http.Cookie对象（导出方法供ClientManager使用）
func (cm *CookieManager) ParseCookieString(cookieStr string) ([]*http.Cookie, error) {
	return cm.parseCookieString(cookieStr)
}

// parseCookieString 解析Cookie字符串为http.Cookie对象（内部方法）
func (cm *CookieManager) parseCookieString(cookieStr string) ([]*http.Cookie, error) {
	if cookieStr == "" {
		return nil, nil
	}

	var cookies []*http.Cookie

	// 尝试解析JSON格式的Cookie数据
	var cookieDataList []CookieData
	if err := jsonx.UnmarshalString(cookieStr, &cookieDataList, ""); err == nil {
		// JSON格式解析成功
		for _, cookieData := range cookieDataList {
			cookie := &http.Cookie{
				Name:     cookieData.Name,
				Value:    cookieData.Value,
				Domain:   cookieData.Domain,
				Path:     cookieData.Path,
				HttpOnly: cookieData.HttpOnly,
				Secure:   cookieData.Secure,
			}

			// 处理过期时间
			if cookieData.Expires > 0 {
				cookie.Expires = time.Unix(int64(cookieData.Expires), 0)
			}

			// 处理SameSite属性
			switch strings.ToLower(cookieData.SameSite) {
			case "strict":
				cookie.SameSite = http.SameSiteStrictMode
			case "lax":
				cookie.SameSite = http.SameSiteLaxMode
			case "none":
				cookie.SameSite = http.SameSiteNoneMode
			default:
				cookie.SameSite = http.SameSiteDefaultMode
			}

			cookies = append(cookies, cookie)
		}
		return cookies, nil
	}

	// 如果JSON解析失败，尝试传统的Cookie字符串格式
	cookiePairs := strings.Split(cookieStr, ";")

	for _, pair := range cookiePairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		// 分割name=value
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			cm.logger.Warnf("跳过无效的Cookie格式: %s", pair)
			continue
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		if name == "" {
			continue
		}

		// 根据不同的域名设置Cookie域
		var domain string
		switch {
		case strings.Contains(name, "sso") || strings.Contains(name, "geiwohuo"):
			domain = ".geiwohuo.com"
		default:
			domain = ".shein.com"
		}

		cookie := &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: domain,
			Path:   "/",
		}

		cookies = append(cookies, cookie)
	}

	return cookies, nil
}

// RefreshCookies 刷新Cookie（重新从管理系统获取）
func (cm *CookieManager) RefreshCookies() ([]*http.Cookie, error) {
	cm.logger.Info("刷新Cookie")
	return cm.LoadCookies()
}

func (cm *CookieManager) ForceRefreshCookies() ([]*http.Cookie, error) {
	tenantID := cm.forceLoginTenantID()
	if tenantID <= 0 {
		return nil, fmt.Errorf("无法确定 SHEIN 店铺 %d 的 tenant_id，无法自动重新登录", cm.storeID)
	}

	if localRefresher := loadLocalLoginRefresher(); localRefresher != nil {
		if err := localRefresher.ForceLogin(context.Background(), tenantID, cm.storeID); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("local SHEIN login refresher is not configured for store %d tenant %d", cm.storeID, tenantID)
	}
	cm.resolvedTenantID = tenantID

	if cm.managementClient != nil {
		cookieStr, refreshedTenantID, err := cm.loadCookieJSONFromRedis()
		if err != nil {
			return nil, fmt.Errorf("重新登录后读取 SHEIN Cookie 失败: %w", err)
		}
		if refreshedTenantID > 0 {
			cm.resolvedTenantID = refreshedTenantID
		}
		if strings.TrimSpace(cookieStr) != "" {
			cookies, parseErr := cm.parseCookieString(cookieStr)
			if parseErr != nil {
				return nil, fmt.Errorf("解析重新登录后的 SHEIN Cookie 失败: %w", parseErr)
			}
			return cookies, nil
		}
	}

	return cm.LoadCookies()
}

type LocalLoginRefresher interface {
	ForceLogin(ctx context.Context, tenantID int64, storeID int64) error
}

var localLoginRefresherMu sync.RWMutex
var localLoginRefresher LocalLoginRefresher

func ConfigureLocalLoginRefresher(refresher LocalLoginRefresher) {
	localLoginRefresherMu.Lock()
	defer localLoginRefresherMu.Unlock()
	localLoginRefresher = refresher
}

func loadLocalLoginRefresher() LocalLoginRefresher {
	localLoginRefresherMu.RLock()
	defer localLoginRefresherMu.RUnlock()
	return localLoginRefresher
}

func (cm *CookieManager) loadCookieJSONFromRedis() (string, int64, error) {
	if cm.managementClient == nil {
		return "", 0, nil
	}
	for _, storeID := range cm.cookieLookupStoreIDs() {
		cookieStr, tenantID, err := cm.managementClient.GetSheinCookie(storeID)
		if err != nil {
			return "", 0, err
		}
		if strings.TrimSpace(cookieStr) != "" {
			return cookieStr, tenantID, nil
		}
	}
	return "", 0, nil
}

func (cm *CookieManager) cookieLookupStoreIDs() []int64 {
	ids := make([]int64, 0, 2)
	if configured := cm.configuredLoginStoreID(); configured > 0 {
		ids = append(ids, configured)
	}
	if cm.storeID > 0 && cm.storeID != idsFirst(ids) {
		ids = append(ids, cm.storeID)
	}
	return ids
}

func idsFirst(ids []int64) int64 {
	if len(ids) == 0 {
		return 0
	}
	return ids[0]
}

func (cm *CookieManager) configuredLoginStoreID() int64 {
	identifier := strings.TrimSpace(loadSheinLoginAccountConfig().identifier)
	if identifier == "" {
		return 0
	}
	value, err := strconv.ParseInt(identifier, 10, 64)
	if err != nil || value <= 0 {
		return 0
	}
	return value
}

func (cm *CookieManager) forceLoginTenantID() int64 {
	if tenantID, err := strconv.ParseInt(strings.TrimSpace(loadSheinLoginAccountConfig().tenantID), 10, 64); err == nil && tenantID > 0 {
		return tenantID
	}
	if cm.resolvedTenantID > 0 {
		return cm.resolvedTenantID
	}
	return cm.resolveTenantID()
}

func (cm *CookieManager) resolveTenantID() int64 {
	if cm.managementClient == nil {
		return 0
	}
	storeClient := cm.managementClient.GetStoreClient()
	if storeClient == nil {
		return 0
	}
	store, err := storeClient.GetStore(cm.storeID)
	if err != nil || store == nil {
		return 0
	}
	return store.TenantID
}

type sheinLoginAccountConfig struct {
	tenantID   string
	identifier string
}

var sheinLoginServiceConfigMu sync.RWMutex
var sheinLoginServiceConfigOverride sheinLoginAccountConfig

func ConfigureLoginAccount(account ...string) {
	sheinLoginServiceConfigMu.Lock()
	defer sheinLoginServiceConfigMu.Unlock()
	tenantID := ""
	identifier := ""
	if len(account) > 0 {
		tenantID = account[0]
	}
	if len(account) > 1 {
		identifier = account[1]
	}
	sheinLoginServiceConfigOverride = sheinLoginAccountConfig{
		tenantID:   strings.TrimSpace(tenantID),
		identifier: strings.TrimSpace(identifier),
	}
}

func loadSheinLoginAccountConfig() sheinLoginAccountConfig {
	cfg := sheinLoginAccountConfig{
		tenantID: firstNonEmptyEnv(
			"TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_TENANT_ID",
			"TASK_PROCESSOR_LOGIN_SERVICE_TENANT_ID",
		),
		identifier: firstNonEmptyEnv(
			"TASK_PROCESSOR_SHEIN_LOGIN_SERVICE_IDENTIFIER",
			"TASK_PROCESSOR_LOGIN_SERVICE_IDENTIFIER",
		),
	}

	sheinLoginServiceConfigMu.RLock()
	override := sheinLoginServiceConfigOverride
	sheinLoginServiceConfigMu.RUnlock()
	if override.tenantID != "" {
		cfg.tenantID = override.tenantID
	}
	if override.identifier != "" {
		cfg.identifier = override.identifier
	}
	return cfg
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		value := strings.TrimSpace(strings.Trim(os.Getenv(key), `"'`))
		if value != "" {
			return value
		}
	}
	return ""
}

// TestConnection 测试管理系统连接和认证状态
func (cm *CookieManager) TestConnection() error {
	cm.logger.Debug("测试管理系统连接")

	if cm.managementClient == nil {
		return fmt.Errorf("管理系统客户端为空")
	}

	// 尝试获取store信息来测试连接
	storeClient := cm.managementClient.GetStoreClient()
	if storeClient == nil {
		return fmt.Errorf("店铺客户端未初始化")
	}

	_, err := storeClient.GetStore(cm.storeID)
	if err != nil {
		cm.logger.WithError(err).Error("管理系统连接测试失败")
		return fmt.Errorf("管理系统连接测试失败: %w", err)
	}

	cm.logger.Debug("管理系统连接测试成功")
	return nil
}

func (cm *CookieManager) GetResolvedTenantID() int64 {
	return cm.resolvedTenantID
}
