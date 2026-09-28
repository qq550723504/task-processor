package httpapi

import (
	"task-processor/internal/core/config"
	"task-processor/internal/integration/zitadelsms"
)

func buildZitadelSMSService(cfg config.ListingKitZitadelSMSConfig) *zitadelsms.Service {
	sender, err := zitadelsms.NewTencentSender(cfg.TencentSecretID, cfg.TencentSecretKey)
	if err != nil {
		return nil
	}
	service, err := zitadelsms.NewService(zitadelsms.Config{
		SigningKey:                     cfg.SigningKey,
		TemplateID:                     cfg.TencentTemplateID,
		SignName:                       cfg.TencentSignName,
		AppID:                          cfg.TencentAppID,
		PhoneVerificationExpiryMinutes: cfg.PhoneVerificationExpiryMinutes,
	}, sender)
	if err != nil {
		return nil
	}
	return service
}
