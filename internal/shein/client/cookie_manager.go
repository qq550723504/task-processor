package client

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"task-processor/internal/core/logger"
	"task-processor/internal/pkg/jsonx"

	"github.com/sirupsen/logrus"
)

type CookieManager struct {
	storeID             int64
	cookieProvider      CookieProvider
	storeConfigProvider StoreConfigProvider
	logger              *logrus.Entry
	resolvedTenantID    int64
}

func NewCookieManager(storeID int64, cookieProvider CookieProvider, storeConfigProvider StoreConfigProvider) *CookieManager {
	return &CookieManager{
		storeID:             storeID,
		cookieProvider:      cookieProvider,
		storeConfigProvider: storeConfigProvider,
		logger:              logger.GetGlobalLogger("SheinCookieManager").WithField("storeID", storeID),
	}
}

func (cm *CookieManager) LoadCookies() ([]*http.Cookie, error) {
	cm.logger.Debug("loading SHEIN cookies from runtime provider")

	if cm.cookieProvider == nil {
		cm.logger.Error("cookie provider is nil")
		return nil, fmt.Errorf("cookie provider is nil")
	}

	cookieStr, tenantID, err := cm.loadCookieJSONFromSource()
	if err != nil {
		cm.logger.WithError(err).Warn("failed to read SHEIN cookie payload from runtime provider")
		return nil, err
	}
	if strings.TrimSpace(cookieStr) == "" {
		cm.logger.Warn("cookie provider returned empty cookie payload")
		return nil, nil
	}

	cm.resolvedTenantID = tenantID
	cookies, parseErr := cm.parseCookieString(cookieStr)
	if parseErr != nil {
		cm.logger.WithError(parseErr).Error("failed to parse SHEIN cookie payload")
		return nil, fmt.Errorf("parse SHEIN cookie payload: %w", parseErr)
	}
	return cookies, nil
}

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

type cookiePayload struct {
	Cookies []CookieData `json:"cookies"`
}

func (cm *CookieManager) ParseCookieString(cookieStr string) ([]*http.Cookie, error) {
	return cm.parseCookieString(cookieStr)
}

func (cm *CookieManager) parseCookieString(cookieStr string) ([]*http.Cookie, error) {
	if cookieStr == "" {
		return nil, nil
	}

	var cookieDataList []CookieData
	if err := jsonx.UnmarshalString(cookieStr, &cookieDataList, ""); err == nil {
		return buildCookiesFromJSON(cookieDataList), nil
	}

	var payload cookiePayload
	if err := jsonx.UnmarshalString(cookieStr, &payload, ""); err == nil && len(payload.Cookies) > 0 {
		return buildCookiesFromJSON(payload.Cookies), nil
	}

	cookiePairs := strings.Split(cookieStr, ";")
	cookies := make([]*http.Cookie, 0, len(cookiePairs))
	for _, pair := range cookiePairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		parts := strings.SplitN(pair, "=", 2)
		if len(parts) != 2 {
			cm.logger.Warnf("skip invalid cookie format: %s", pair)
			continue
		}

		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if name == "" {
			continue
		}

		domain := ".shein.com"
		switch {
		case strings.Contains(name, "sso"), strings.Contains(name, "geiwohuo"):
			domain = ".geiwohuo.com"
		}

		cookies = append(cookies, &http.Cookie{
			Name:   name,
			Value:  value,
			Domain: domain,
			Path:   "/",
		})
	}

	return cookies, nil
}

func buildCookiesFromJSON(cookieDataList []CookieData) []*http.Cookie {
	cookies := make([]*http.Cookie, 0, len(cookieDataList))
	for _, cookieData := range cookieDataList {
		cookie := &http.Cookie{
			Name:     cookieData.Name,
			Value:    cookieData.Value,
			Domain:   cookieData.Domain,
			Path:     cookieData.Path,
			HttpOnly: cookieData.HttpOnly,
			Secure:   cookieData.Secure,
		}
		if cookieData.Expires > 0 {
			cookie.Expires = time.Unix(int64(cookieData.Expires), 0)
		}

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
	return cookies
}

func (cm *CookieManager) RefreshCookies() ([]*http.Cookie, error) {
	cm.logger.Info("refreshing cookies")
	return cm.LoadCookies()
}

func (cm *CookieManager) ForceRefreshCookies() ([]*http.Cookie, error) {
	tenantID := cm.forceLoginTenantID()
	if tenantID <= 0 {
		return nil, fmt.Errorf("cannot determine tenant for SHEIN store %d", cm.storeID)
	}

	if localRefresher := loadLocalLoginRefresher(); localRefresher != nil {
		if err := localRefresher.ForceLogin(context.Background(), tenantID, cm.storeID); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("local SHEIN login refresher is not configured for store %d tenant %d", cm.storeID, tenantID)
	}
	cm.resolvedTenantID = tenantID

	if cm.cookieProvider != nil {
		cookieStr, refreshedTenantID, err := cm.loadCookieJSONFromSource()
		if err != nil {
			return nil, fmt.Errorf("read SHEIN cookie after refresh: %w", err)
		}
		if refreshedTenantID > 0 {
			cm.resolvedTenantID = refreshedTenantID
		}
		if strings.TrimSpace(cookieStr) != "" {
			cookies, parseErr := cm.parseCookieString(cookieStr)
			if parseErr != nil {
				return nil, fmt.Errorf("parse SHEIN cookie after refresh: %w", parseErr)
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

func (cm *CookieManager) loadCookieJSONFromSource() (string, int64, error) {
	if cm.cookieProvider != nil {
		for _, storeID := range cm.cookieLookupStoreIDs() {
			result, err := cm.cookieProvider.GetCookie(context.Background(), storeID)
			if err != nil {
				return "", 0, err
			}
			if result != nil && strings.TrimSpace(result.CookieJSON) != "" {
				return result.CookieJSON, result.TenantID, nil
			}
		}
	}
	return "", 0, nil
}

func (cm *CookieManager) cookieLookupStoreIDs() []int64 {
	if cm.storeID > 0 {
		return []int64{cm.storeID}
	}
	if configured := cm.configuredLoginStoreID(); configured > 0 {
		return []int64{configured}
	}
	return nil
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
	if cm.resolvedTenantID > 0 {
		return cm.resolvedTenantID
	}
	if tenantID := cm.resolveTenantID(); tenantID > 0 {
		return tenantID
	}
	return 0
}

func (cm *CookieManager) resolveTenantID() int64 {
	if cm.storeConfigProvider == nil {
		return 0
	}
	store, err := cm.storeConfigProvider.GetStoreConfig(context.Background(), cm.storeID)
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

func (cm *CookieManager) TestConnection() error {
	cm.logger.Debug("testing store config connection")
	if cm.storeConfigProvider == nil {
		return fmt.Errorf("store config provider is nil")
	}

	if _, err := cm.storeConfigProvider.GetStoreConfig(context.Background(), cm.storeID); err != nil {
		cm.logger.WithError(err).Error("store config connection test failed")
		return fmt.Errorf("store config connection test failed: %w", err)
	}

	cm.logger.Debug("store config connection test succeeded")
	return nil
}

func (cm *CookieManager) GetResolvedTenantID() int64 {
	return cm.resolvedTenantID
}
