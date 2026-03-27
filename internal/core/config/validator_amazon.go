package config

func ValidateAmazonConfig(amazon *AmazonConfig) []error {
	var errors []error
	if amazon == nil || !amazon.Enabled {
		return errors
	}

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
