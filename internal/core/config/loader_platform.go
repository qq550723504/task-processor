package config

import "github.com/spf13/viper"

func BuildPlatformConfig(v *viper.Viper, prefix string) PlatformConfig {
	cfg := PlatformConfig{
		Enabled:          v.GetBool(prefix + ".enabled"),
		SchedulerEnabled: v.GetBool(prefix + ".schedulerEnabled"),
		FetchMode:        v.GetString(prefix + ".fetchMode"),
		CookieRedis: RedisConfig{
			Host:     v.GetString(prefix + ".cookieRedis.host"),
			Port:     v.GetInt(prefix + ".cookieRedis.port"),
			Password: v.GetString(prefix + ".cookieRedis.password"),
			DB:       v.GetInt(prefix + ".cookieRedis.db"),
			PoolSize: v.GetInt(prefix + ".cookieRedis.pool_size"),
		},
		AutoPricing: AutoPricingConfig{
			Enabled:        v.GetBool(prefix + ".autoPricing.enabled"),
			Interval:       v.GetInt(prefix + ".autoPricing.interval"),
			BatchSize:      v.GetInt(prefix + ".autoPricing.batchSize"),
			UseAmazonPrice: v.GetBool(prefix + ".autoPricing.useAmazonPrice"),
		},
		ListingPricing: ListingPricingConfig{
			Enabled:        v.GetBool(prefix + ".listingPricing.enabled"),
			Currency:       v.GetString(prefix + ".listingPricing.currency"),
			MarkupRate:     v.GetFloat64(prefix + ".listingPricing.markupRate"),
			FixedMarkup:    v.GetFloat64(prefix + ".listingPricing.fixedMarkup"),
			ShippingCost:   v.GetFloat64(prefix + ".listingPricing.shippingCost"),
			CommissionRate: v.GetFloat64(prefix + ".listingPricing.commissionRate"),
			MinimumPrice:   v.GetFloat64(prefix + ".listingPricing.minimumPrice"),
			RoundTo:        v.GetFloat64(prefix + ".listingPricing.roundTo"),
		},
		ProductSync: ScheduledTaskConfig{
			Enabled:  v.GetBool(prefix + ".productSync.enabled"),
			Interval: v.GetInt(prefix + ".productSync.interval"),
		},
		InventorySync: ScheduledTaskConfig{
			Enabled:  v.GetBool(prefix + ".inventorySync.enabled"),
			Interval: v.GetInt(prefix + ".inventorySync.interval"),
		},
		ActivityRegistration: ScheduledTaskConfig{
			Enabled:  v.GetBool(prefix + ".activityRegistration.enabled"),
			Interval: v.GetInt(prefix + ".activityRegistration.interval"),
		},
		SyncProduct: SyncProductConfig{
			Enabled:   v.GetBool(prefix + ".sync.enabled"),
			StoreIDs:  getInt64Slice(v, prefix+".sync.storeIDs"),
			Interval:  v.GetInt(prefix + ".sync.interval"),
			BatchSize: v.GetInt(prefix + ".sync.batchSize"),
		},
		Monitor: MonitorConfig{
			Enabled:              v.GetBool(prefix + ".monitor.enabled"),
			StoreIDs:             getInt64Slice(v, prefix+".monitor.storeIDs"),
			CheckInterval:        v.GetInt(prefix + ".monitor.checkInterval"),
			BatchSize:            v.GetInt(prefix + ".monitor.batchSize"),
			EnablePriceAlert:     v.GetBool(prefix + ".monitor.enablePriceAlert"),
			EnableStockAlert:     v.GetBool(prefix + ".monitor.enableStockAlert"),
			PriceChangeThreshold: v.GetFloat64(prefix + ".monitor.priceChangeThreshold"),
			StockChangeThreshold: v.GetInt(prefix + ".monitor.stockChangeThreshold"),
		},
		LoginService: BuildLoginServiceConfig(v, prefix+".loginService"),
		HTTPClient:   BuildHTTPClientConfig(v, prefix+".httpClient"),
	}

	return normalizePlatformConfig(cfg)
}

func BuildLoginServiceConfig(v *viper.Viper, prefix string) LoginServiceConfig {
	return LoginServiceConfig{
		BaseURL:             v.GetString(prefix + ".baseURL"),
		SharedKey:           v.GetString(prefix + ".sharedKey"),
		TenantID:            v.GetString(prefix + ".tenantID"),
		Identifier:          v.GetString(prefix + ".identifier"),
		MerchantName:        v.GetString(prefix + ".merchantName"),
		Username:            v.GetString(prefix + ".username"),
		Password:            v.GetString(prefix + ".password"),
		MaxConcurrentLogins: v.GetInt(prefix + ".maxConcurrentLogins"),
		ProfileRootDir:      v.GetString(prefix + ".profileRootDir"),
		ArtifactDir:         v.GetString(prefix + ".artifactDir"),
		DefaultHeadless:     v.GetBool(prefix + ".defaultHeadless"),
	}
}

func BuildSDSAuthBootstrapConfig(v *viper.Viper, prefix string) SDSAuthBootstrapConfig {
	return SDSAuthBootstrapConfig{
		StaticAccessToken:       v.GetString(prefix + ".staticAccessToken"),
		StaticOutToken:          v.GetString(prefix + ".staticOutToken"),
		StaticMerchantID:        v.GetInt64(prefix + ".staticMerchantID"),
		StaticCookie:            v.GetString(prefix + ".staticCookie"),
		LoginDomainName:         v.GetString(prefix + ".loginDomainName"),
		LoginVerifyCaptchaParam: v.GetString(prefix + ".loginVerifyCaptchaParam"),
		LoginExtraInfo:          v.GetString(prefix + ".loginExtraInfo"),
		ManagementStoreID:       v.GetInt64(prefix + ".managementStoreID"),
	}
}

func normalizePlatformConfig(cfg PlatformConfig) PlatformConfig {
	if !cfg.ProductSync.Enabled && cfg.SyncProduct.Enabled {
		cfg.ProductSync.Enabled = true
	}
	if cfg.ProductSync.Interval == 0 && cfg.SyncProduct.Interval > 0 {
		cfg.ProductSync.Interval = cfg.SyncProduct.Interval
	}

	return cfg
}
