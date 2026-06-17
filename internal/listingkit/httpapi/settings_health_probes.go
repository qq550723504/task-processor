package httpapi

import (
	"strings"

	"task-processor/internal/core/config"
	"task-processor/internal/listingkit"
)

func buildSettingsHealthProbesFromConfig(cfg *config.Config) listingkit.SettingsHealthProbes {
	if cfg == nil {
		return listingkit.SettingsHealthProbes{
			SheinIntegration: missingProbe("shein.loginService.baseURL 缺失", "shein.loginService.tenantID 缺失", "shein.loginService.identifier 缺失"),
			SDSLogin:         missingProbe("sds.loginService.baseURL 缺失", "sds.loginService.tenantID 缺失", "sds.loginService.identifier 缺失"),
			ObjectStorage:    missingProbe("productimage.publisher.provider 缺失"),
		}
	}
	return listingkit.SettingsHealthProbes{
		SheinIntegration: sheinIntegrationProbe(cfg),
		SDSLogin:         sdsLoginProbe(cfg),
		ObjectStorage:    objectStorageProbe(cfg),
	}
}

func completeSettingsHealthProbesWithSubmitRuntime(probes listingkit.SettingsHealthProbes, submit submitModule) listingkit.SettingsHealthProbes {
	missing := append([]string(nil), probes.SheinIntegration.Missing...)
	if submit.shein.productAPIBuilder == nil {
		missing = append(missing, "shein.productAPIBuilder 未接入")
	}
	if submit.shein.imageAPIBuilder == nil {
		missing = append(missing, "shein.imageAPIBuilder 未接入")
	}
	if submit.shein.categoryResolver == nil {
		missing = append(missing, "shein.categoryResolver 未接入")
	}
	probes.SheinIntegration = probeFromMissing(missing)
	return probes
}

func sheinIntegrationProbe(cfg *config.Config) listingkit.SettingsHealthProbe {
	login := cfg.Platforms.Shein.LoginService
	missing := requiredStringFields("shein.loginService", map[string]string{
		"baseURL":    login.BaseURL,
		"tenantID":   login.TenantID,
		"identifier": login.Identifier,
	})
	cookieRedis := cfg.EffectiveSheinCookieRedis()
	if strings.TrimSpace(cookieRedis.Host) == "" {
		missing = append(missing, "shein.cookieRedis.host 缺失")
	}
	return probeFromMissing(missing)
}

func sdsLoginProbe(cfg *config.Config) listingkit.SettingsHealthProbe {
	login := cfg.Platforms.SDS.LoginService
	missing := requiredStringFields("sds.loginService", map[string]string{
		"baseURL":    login.BaseURL,
		"tenantID":   login.TenantID,
		"identifier": login.Identifier,
	})
	return probeFromMissing(missing)
}

func objectStorageProbe(cfg *config.Config) listingkit.SettingsHealthProbe {
	publisher := cfg.ProductImage.Publisher
	provider := strings.TrimSpace(strings.ToLower(publisher.Provider))
	if provider == "" || provider == "local" || provider == "filesystem" || provider == "file" {
		return listingkit.SettingsHealthProbe{Configured: true}
	}
	if provider != "s3" {
		return missingProbe("productimage.publisher.provider 不支持: " + publisher.Provider)
	}
	missing := requiredStringFields("productimage.publisher.s3", map[string]string{
		"bucket":          publisher.S3.Bucket,
		"endpoint":        publisher.S3.Endpoint,
		"accessKeyID":     publisher.S3.AccessKeyID,
		"secretAccessKey": publisher.S3.SecretAccessKey,
	})
	return probeFromMissing(missing)
}

func requiredStringFields(prefix string, values map[string]string) []string {
	missing := make([]string, 0, len(values))
	for key, value := range values {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, prefix+"."+key+" 缺失")
		}
	}
	return missing
}

func missingProbe(items ...string) listingkit.SettingsHealthProbe {
	return listingkit.SettingsHealthProbe{Missing: items}
}

func probeFromMissing(missing []string) listingkit.SettingsHealthProbe {
	if len(missing) > 0 {
		return listingkit.SettingsHealthProbe{Missing: missing}
	}
	return listingkit.SettingsHealthProbe{Configured: true}
}
