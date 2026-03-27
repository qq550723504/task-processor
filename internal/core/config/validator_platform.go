package config

import (
	"fmt"
	"strings"
)

func ValidatePlatformsConfig(platforms *PlatformsConfig) []error {
	var errors []error
	if platforms == nil {
		return errors
	}

	errors = append(errors, ValidatePlatformConfig("temu", &platforms.Temu)...)
	errors = append(errors, ValidatePlatformConfig("shein", &platforms.Shein)...)

	if platforms.Alibaba1688.Enabled {
		if platforms.Alibaba1688.Timeout <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "platforms.alibaba1688.timeout",
				Message: "1688 timeout must be greater than 0",
			})
		}
		if platforms.Alibaba1688.PoolSize <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "platforms.alibaba1688.poolSize",
				Message: "1688 poolSize must be greater than 0",
			})
		}
	}

	return errors
}

func ValidatePlatformConfig(platformName string, platform *PlatformConfig) []error {
	var errors []error
	if platform == nil {
		return errors
	}

	enabledScheduledTasks := 0

	if platform.AutoPricing.Enabled {
		enabledScheduledTasks++
		if platform.AutoPricing.Interval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.autoPricing.interval", platformName),
				Message: fmt.Sprintf("%s autoPricing interval must be greater than 0", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.autoPricing.interval to a positive number of seconds", platformName),
			})
		}
		if platform.AutoPricing.BatchSize <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.autoPricing.batchSize", platformName),
				Message: fmt.Sprintf("%s autoPricing batchSize must be greater than 0", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.autoPricing.batchSize to a positive integer", platformName),
			})
		}
	}

	if platform.ProductSync.Enabled {
		enabledScheduledTasks++
		if platform.ProductSync.Interval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.productSync.interval", platformName),
				Message: fmt.Sprintf("%s productSync interval must be greater than 0", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.productSync.interval to a positive number of seconds", platformName),
			})
		}
	}

	if platform.InventorySync.Enabled {
		enabledScheduledTasks++
		if platform.InventorySync.Interval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.inventorySync.interval", platformName),
				Message: fmt.Sprintf("%s inventorySync interval must be greater than 0", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.inventorySync.interval to a positive number of seconds", platformName),
			})
		}
	}

	if platform.ActivityRegistration.Enabled {
		enabledScheduledTasks++
		if platform.ActivityRegistration.Interval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.activityRegistration.interval", platformName),
				Message: fmt.Sprintf("%s activityRegistration interval must be greater than 0", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.activityRegistration.interval to a positive number of seconds", platformName),
			})
		}
	}

	if platform.SyncProduct.Enabled && platform.ProductSync.Interval <= 0 && platform.SyncProduct.Interval <= 0 {
		errors = append(errors, &ValidationError{
			Field:   fmt.Sprintf("platforms.%s.sync.interval", platformName),
			Message: fmt.Sprintf("%s legacy sync interval must be greater than 0 when used as a fallback", strings.ToUpper(platformName)),
			Hint:    fmt.Sprintf("prefer platforms.%s.productSync.interval, or set a positive legacy sync interval", platformName),
		})
	}
	if platform.SyncProduct.Enabled && platform.ProductSync.Enabled && platform.ProductSync.Interval > 0 && platform.SyncProduct.Interval > 0 && platform.ProductSync.Interval != platform.SyncProduct.Interval {
		errors = append(errors, &ValidationError{
			Field:   fmt.Sprintf("platforms.%s.sync.interval", platformName),
			Message: fmt.Sprintf("%s sync and productSync intervals conflict", strings.ToUpper(platformName)),
			Hint:    fmt.Sprintf("remove platforms.%s.sync and keep platforms.%s.productSync as the single source of truth", platformName, platformName),
		})
	}

	if platform.SyncProduct.Enabled && platform.SyncProduct.BatchSize > 0 {
		errors = append(errors, &ValidationError{
			Field:   fmt.Sprintf("platforms.%s.sync.batchSize", platformName),
			Message: fmt.Sprintf("%s legacy sync batchSize is deprecated and ignored", strings.ToUpper(platformName)),
			Hint:    fmt.Sprintf("remove platforms.%s.sync.batchSize; the scheduler reads platforms.%s.productSync instead", platformName, platformName),
		})
	}
	if platform.SyncProduct.Enabled && len(platform.SyncProduct.StoreIDs) > 0 {
		errors = append(errors, &ValidationError{
			Field:   fmt.Sprintf("platforms.%s.sync.storeIDs", platformName),
			Message: fmt.Sprintf("%s legacy sync storeIDs are deprecated and ignored", strings.ToUpper(platformName)),
			Hint:    fmt.Sprintf("remove platforms.%s.sync.storeIDs; this field is no longer consumed by the scheduler", platformName),
		})
	}

	if platform.Monitor.Enabled {
		if platform.Monitor.CheckInterval <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.monitor.checkInterval", platformName),
				Message: fmt.Sprintf("%s monitor checkInterval must be greater than 0", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.monitor.checkInterval to a positive number of seconds", platformName),
			})
		}
		if platform.Monitor.BatchSize <= 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.monitor.batchSize", platformName),
				Message: fmt.Sprintf("%s monitor batchSize must be greater than 0", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.monitor.batchSize to a positive integer", platformName),
			})
		}
		if platform.Monitor.PriceChangeThreshold < 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.monitor.priceChangeThreshold", platformName),
				Message: fmt.Sprintf("%s monitor priceChangeThreshold cannot be negative", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.monitor.priceChangeThreshold to 0 or a positive value", platformName),
			})
		}
		if platform.Monitor.StockChangeThreshold < 0 {
			errors = append(errors, &ValidationError{
				Field:   fmt.Sprintf("platforms.%s.monitor.stockChangeThreshold", platformName),
				Message: fmt.Sprintf("%s monitor stockChangeThreshold cannot be negative", strings.ToUpper(platformName)),
				Hint:    fmt.Sprintf("set platforms.%s.monitor.stockChangeThreshold to 0 or a positive value", platformName),
			})
		}
	}

	if platform.SchedulerEnabled && enabledScheduledTasks == 0 {
		errors = append(errors, &ValidationError{
			Field:   fmt.Sprintf("platforms.%s.schedulerEnabled", platformName),
			Message: fmt.Sprintf("%s schedulerEnabled requires at least one scheduled task to be enabled", strings.ToUpper(platformName)),
			Hint:    fmt.Sprintf("enable at least one scheduled task under platforms.%s, or turn off platforms.%s.schedulerEnabled", platformName, platformName),
		})
	}

	return errors
}
