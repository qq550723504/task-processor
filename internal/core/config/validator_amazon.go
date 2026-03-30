package config

func ValidateAmazonConfig(amazon *AmazonConfig) []error {
	var errors []error
	if amazon == nil {
		return errors
	}

	if amazon.Enabled || amazon.RemoteAPI.Enabled || amazon.SPAPI.Enabled {
		if amazon.DataFreshnessDays <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "amazon.dataFreshnessDays",
				Message: "Amazon dataFreshnessDays must be greater than 0",
				Hint:    "set a positive freshness window in days for amazon.dataFreshnessDays",
			})
		}
		if amazon.CrawlTimeout <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "amazon.crawlTimeout",
				Message: "Amazon crawlTimeout must be greater than 0",
				Hint:    "set a positive crawl timeout in seconds for amazon.crawlTimeout",
			})
		}
		if hasExplicitAmazonProductDedupeConfig(amazon) {
			if amazon.ProductDedupe.LockTTLSeconds <= 0 {
				errors = append(errors, &ValidationError{
					Field:   "amazon.productDedupe.lockTTLSeconds",
					Message: "Amazon productDedupe lockTTLSeconds must be greater than 0",
					Hint:    "set a positive lock TTL in seconds for amazon.productDedupe.lockTTLSeconds",
				})
			}
			if amazon.ProductDedupe.ResultTTLSeconds <= 0 {
				errors = append(errors, &ValidationError{
					Field:   "amazon.productDedupe.resultTTLSeconds",
					Message: "Amazon productDedupe resultTTLSeconds must be greater than 0",
					Hint:    "set a positive result cache TTL in seconds for amazon.productDedupe.resultTTLSeconds",
				})
			}
			if amazon.ProductDedupe.WaitTimeoutSeconds <= 0 {
				errors = append(errors, &ValidationError{
					Field:   "amazon.productDedupe.waitTimeoutSeconds",
					Message: "Amazon productDedupe waitTimeoutSeconds must be greater than 0",
					Hint:    "set a positive wait timeout in seconds for amazon.productDedupe.waitTimeoutSeconds",
				})
			}
			if amazon.ProductDedupe.PollIntervalMillis <= 0 {
				errors = append(errors, &ValidationError{
					Field:   "amazon.productDedupe.pollIntervalMillis",
					Message: "Amazon productDedupe pollIntervalMillis must be greater than 0",
					Hint:    "set a positive polling interval in milliseconds for amazon.productDedupe.pollIntervalMillis",
				})
			}
		}
		if amazon.FailureArtifacts.Enabled {
			if amazon.FailureArtifacts.Directory == "" {
				errors = append(errors, &ValidationError{
					Field:   "amazon.failureArtifacts.directory",
					Message: "Amazon failureArtifacts directory cannot be empty when enabled",
					Hint:    "set a writable directory for amazon.failureArtifacts.directory",
				})
			}
			if amazon.FailureArtifacts.MaxHTMLBytes <= 0 {
				errors = append(errors, &ValidationError{
					Field:   "amazon.failureArtifacts.maxHTMLBytes",
					Message: "Amazon failureArtifacts maxHTMLBytes must be greater than 0",
					Hint:    "set a positive byte limit for amazon.failureArtifacts.maxHTMLBytes",
				})
			}
		}
		thresholds := map[string]int{
			"amazon.riskControl.captchaRecreateThreshold":        amazon.RiskControl.CaptchaRecreateThreshold,
			"amazon.riskControl.authenticationRecreateThreshold": amazon.RiskControl.AuthenticationRecreateThreshold,
			"amazon.riskControl.browserCrashRecreateThreshold":   amazon.RiskControl.BrowserCrashRecreateThreshold,
			"amazon.riskControl.timeoutRecreateThreshold":        amazon.RiskControl.TimeoutRecreateThreshold,
			"amazon.riskControl.networkRecreateThreshold":        amazon.RiskControl.NetworkRecreateThreshold,
			"amazon.riskControl.serverErrorRecreateThreshold":    amazon.RiskControl.ServerErrorRecreateThreshold,
		}
		for field, value := range thresholds {
			if value <= 0 {
				errors = append(errors, &ValidationError{
					Field:   field,
					Message: "Amazon riskControl threshold must be greater than 0",
					Hint:    "set a positive recreate threshold for " + field,
				})
			}
		}
		if amazon.RegionGuard.Enabled {
			regionGuardFields := map[string]int{
				"amazon.regionGuard.failureThreshold":        amazon.RegionGuard.FailureThreshold,
				"amazon.regionGuard.evaluationWindowSeconds": amazon.RegionGuard.EvaluationWindowSeconds,
				"amazon.regionGuard.cooldownSeconds":         amazon.RegionGuard.CooldownSeconds,
			}
			for field, value := range regionGuardFields {
				if value <= 0 {
					errors = append(errors, &ValidationError{
						Field:   field,
						Message: "Amazon regionGuard setting must be greater than 0",
						Hint:    "set a positive value for " + field,
					})
				}
			}
		}
		if amazon.QualityControl.RetryOnValidationFailure && amazon.QualityControl.ValidationRetryMaxAttempts <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "amazon.qualityControl.validationRetryMaxAttempts",
				Message: "Amazon qualityControl validationRetryMaxAttempts must be greater than 0 when retry is enabled",
				Hint:    "set a positive retry attempt count for amazon.qualityControl.validationRetryMaxAttempts",
			})
		}
		if amazon.ProxyPool.Enabled {
			if amazon.ProxyPool.Strategy == "" {
				errors = append(errors, &ValidationError{
					Field:   "amazon.proxyPool.strategy",
					Message: "Amazon proxyPool strategy cannot be empty when enabled",
					Hint:    "set amazon.proxyPool.strategy to round_robin",
				})
			}
			if amazon.ProxyPool.FailureCooldownSeconds <= 0 {
				errors = append(errors, &ValidationError{
					Field:   "amazon.proxyPool.failureCooldownSeconds",
					Message: "Amazon proxyPool failureCooldownSeconds must be greater than 0 when enabled",
					Hint:    "set a positive cooldown in seconds for amazon.proxyPool.failureCooldownSeconds",
				})
			}
			if len(amazon.ProxyPool.Proxies) == 0 {
				errors = append(errors, &ValidationError{
					Field:   "amazon.proxyPool.proxies",
					Message: "Amazon proxyPool proxies cannot be empty when enabled",
					Hint:    "configure at least one proxy server under amazon.proxyPool.proxies",
				})
			}
		}
	}

	if amazon.RemoteAPI.Enabled {
		if amazon.RemoteAPI.BaseURL == "" {
			errors = append(errors, &ValidationError{
				Field:   "amazon.remoteAPI.baseURL",
				Message: "Amazon remoteAPI baseURL cannot be empty",
				Hint:    "set amazon.remoteAPI.baseURL in YAML or export TASK_PROCESSOR_AMAZON_REMOTE_API_BASE_URL",
			})
		}
		if amazon.RemoteAPI.Timeout <= 0 {
			errors = append(errors, &ValidationError{
				Field:   "amazon.remoteAPI.timeout",
				Message: "Amazon remoteAPI timeout must be greater than 0",
				Hint:    "set a positive timeout in seconds for amazon.remoteAPI.timeout",
			})
		}
	}

	if amazon.SPAPI.Enabled {
		if amazon.SPAPI.Region == "" {
			errors = append(errors, &ValidationError{
				Field:   "amazon.spapi.region",
				Message: "Amazon SP-API region cannot be empty",
				Hint:    "set amazon.spapi.region in YAML or export TASK_PROCESSOR_AMAZON_SPAPI_REGION, for example us-east-1",
			})
		}
		if amazon.SPAPI.ClientID == "" {
			errors = append(errors, &ValidationError{
				Field:   "amazon.spapi.clientID",
				Message: "Amazon SP-API clientID cannot be empty",
				Hint:    "set amazon.spapi.clientID in YAML or export TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_ID before enabling amazon.spapi.enabled",
			})
		}
		if amazon.SPAPI.ClientSecret == "" {
			errors = append(errors, &ValidationError{
				Field:   "amazon.spapi.clientSecret",
				Message: "Amazon SP-API clientSecret cannot be empty",
				Hint:    "set amazon.spapi.clientSecret in YAML or export TASK_PROCESSOR_AMAZON_SPAPI_CLIENT_SECRET before enabling amazon.spapi.enabled",
			})
		}
		if amazon.SPAPI.RefreshToken == "" {
			errors = append(errors, &ValidationError{
				Field:   "amazon.spapi.refreshToken",
				Message: "Amazon SP-API refreshToken cannot be empty",
				Hint:    "set amazon.spapi.refreshToken in YAML or export TASK_PROCESSOR_AMAZON_SPAPI_REFRESH_TOKEN before enabling amazon.spapi.enabled",
			})
		}
		if amazon.SPAPI.DefaultMarketplace == "" {
			errors = append(errors, &ValidationError{
				Field:   "amazon.spapi.defaultMarketplace",
				Message: "Amazon SP-API defaultMarketplace cannot be empty",
				Hint:    "set amazon.spapi.defaultMarketplace to a marketplace ID like ATVPDKIKX0DER, or to a configured markets key such as us",
			})
		}
		if amazon.SPAPI.DefaultFulfillmentType == "" {
			errors = append(errors, &ValidationError{
				Field:   "amazon.spapi.defaultFulfillmentType",
				Message: "Amazon SP-API defaultFulfillmentType cannot be empty",
				Hint:    "set amazon.spapi.defaultFulfillmentType in YAML or export TASK_PROCESSOR_AMAZON_SPAPI_DEFAULT_FULFILLMENT_TYPE",
			})
		}
		if amazon.SPAPI.DefaultCondition == "" {
			errors = append(errors, &ValidationError{
				Field:   "amazon.spapi.defaultCondition",
				Message: "Amazon SP-API defaultCondition cannot be empty",
				Hint:    "set amazon.spapi.defaultCondition in YAML or export TASK_PROCESSOR_AMAZON_SPAPI_DEFAULT_CONDITION",
			})
		}
	}

	return errors
}

func hasExplicitAmazonProductDedupeConfig(amazon *AmazonConfig) bool {
	if amazon == nil {
		return false
	}
	return amazon.ProductDedupe.LockTTLSeconds != 0 ||
		amazon.ProductDedupe.ResultTTLSeconds != 0 ||
		amazon.ProductDedupe.WaitTimeoutSeconds != 0 ||
		amazon.ProductDedupe.PollIntervalMillis != 0
}
