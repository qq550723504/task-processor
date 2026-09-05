package httpapi

import (
	"testing"

	"github.com/stretchr/testify/require"
	"task-processor/internal/core/config"
)

func TestBuildZitadelSMSServicePassesPhoneExpiryConfiguration(t *testing.T) {
	cfg := config.ListingKitZitadelSMSConfig{
		SigningKey: "test-key", TencentSecretID: "test-id", TencentSecretKey: "test-secret",
		TencentAppID: "test-app", TencentSignName: "test-sign", TencentTemplateID: "test-template",
		PhoneVerificationExpiryMinutes: 60,
	}
	require.NotNil(t, buildZitadelSMSService(cfg))
	cfg.PhoneVerificationExpiryMinutes = 61
	require.Nil(t, buildZitadelSMSService(cfg))
}
