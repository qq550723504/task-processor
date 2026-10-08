package httpapi

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/authidentity"
	"task-processor/internal/authz"
	"task-processor/internal/commercial/billing"
	billinghttp "task-processor/internal/commercial/billing/httpapi"
	coreconfig "task-processor/internal/core/config"
	"task-processor/internal/integration/wallettopup"
	topupconfig "task-processor/internal/integration/wallettopup/config"
	"task-processor/internal/ledger/money"
)

type topUpRuntimeAuthorizer struct {
	directory financialRecoveryAuthorizer
}

func (a topUpRuntimeAuthorizer) AuthorizeTopUp(ctx context.Context, org, actor string, refund bool) error {
	identity, ok := authidentity.AuthenticatedIdentityFromContext(ctx)
	if !ok || identity.UserID != actor || !identity.TokenExpiresAt.IsZero() && !time.Now().Before(identity.TokenExpiresAt) || a.directory.authorizer == nil {
		return billing.ErrAuthorizationRevoked
	}
	if refund {
		// The global route preserves only roles from the verified current token. It
		// clears organization scope before this check; target-tenant admin is insufficient.
		if org != "" || identity.TenantID != "" || identity.EffectiveOrganizationID != "" || !a.directory.authorizer.Authorize(actor, identity.Roles, authz.PermissionListingKitPlatformAdm) {
			return billing.ErrAuthorizationRevoked
		}
		return nil
	}
	if org == "" || identity.TenantID != org || identity.EffectiveOrganizationID != org || a.directory.reader == nil {
		return billing.ErrAuthorizationRevoked
	}
	result, err := a.directory.reader.ReadExactServiceProjectAuthorization(ctx, a.directory.serviceToken, actor, a.directory.projectID, org)
	if err != nil || !result.Found || result.State != "STATE_ACTIVE" || !authz.AllowedOrganization(ctx, a.directory.authorizer, actor, org, result.Roles, authz.PermissionWorkbenchCommercialWalletTopUp) {
		return billing.ErrAuthorizationRevoked
	}
	return nil
}
func readTopUpSecret(ctx context.Context, path string) (string, error) {
	if coreconfig.VerifyPrivateFiles(ctx, []string{path}) != nil {
		return "", billing.ErrFeatureUnavailable
	}
	return readTopUpKeyFile(path)
}

func readTopUpKeyFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", billing.ErrFeatureUnavailable
	}
	f, err := os.Open(path)
	if err != nil {
		return "", billing.ErrFeatureUnavailable
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil || len(b) == 0 || len(b) > 16384 {
		return "", billing.ErrInvalid
	}
	return strings.TrimSpace(string(b)), nil
}
func configureWalletTopUps(ctx context.Context, service *billing.Service, handler *billinghttp.Handler, store billing.TopUpStore, owner money.ProviderTopUpOwner, cfg topupconfig.Config, auth topUpRuntimeAuthorizer) error {
	policy := billing.TopUpAmountPolicy{MinMinor: cfg.MinMinor, MaxMinor: cfg.MaxMinor, PaymentWindow: time.Duration(cfg.PaymentWindowSeconds) * time.Second}
	for _, raw := range cfg.QuickAmounts {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return errors.New("invalid wallet top-up amount policy")
		}
		policy.QuickAmounts = append(policy.QuickAmounts, n)
	}
	if (cfg.MinMinor != 0 || cfg.MaxMinor != 0 || len(cfg.QuickAmounts) > 0 || cfg.PaymentWindowSeconds != 0) && policy.Validate() != nil {
		return errors.New("invalid wallet top-up amount policy")
	}
	var protection billing.TopUpPayloadProtection
	if cfg.PayloadKeyFile != "" {
		raw, err := readTopUpSecret(ctx, cfg.PayloadKeyFile)
		if err != nil {
			return errors.New("wallet top-up payload key unavailable")
		}
		key, err := base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return errors.New("invalid wallet top-up payload key")
		}
		protection, err = wallettopup.NewPayloadProtection(key)
		if err != nil {
			return errors.New("invalid wallet top-up payload key")
		}
	}
	var alipay, wechat billing.TopUpProviderPort
	var aliNotify, wxNotify billinghttp.NotificationVerifier
	for _, channel := range []struct {
		provider billing.PaymentProvider
		product  string
		config   topupconfig.Channel
	}{{billing.PaymentAlipay, "PAGE_PAY", cfg.Alipay}, {billing.PaymentWeChat, "NATIVE", cfg.WeChat}} {
		c := channel.config
		if !c.Enabled {
			continue
		}
		private, e1 := readTopUpSecret(ctx, c.PrivateKeyFile)
		public, e2 := readTopUpKeyFile(c.PublicKeyFile)
		if e1 != nil || e2 != nil {
			return errors.New("wallet top-up channel key unavailable")
		}
		merchant := billing.TopUpMerchant{Provider: channel.provider, Environment: c.Environment, ProfileVersion: c.ProfileVersion, MerchantID: c.MerchantID, AppID: c.AppID, Product: channel.product}
		if channel.provider == billing.PaymentAlipay {
			p, err := wallettopup.NewAlipay(wallettopup.AlipayConfig{Merchant: merchant, NewPayments: c.NewPayments, PrivateKey: private, PublicKey: public, NotifyURL: c.NotifyURL, ReturnURL: c.ReturnURL})
			if err != nil {
				return errors.New("invalid Alipay top-up configuration")
			}
			alipay = p
			aliNotify = p
		} else {
			apiKey, err := readTopUpSecret(ctx, c.APIv3KeyFile)
			if err != nil {
				return errors.New("WeChat APIv3 key unavailable")
			}
			p, err := wallettopup.NewWeChat(wallettopup.WeChatConfig{Merchant: merchant, NewPayments: c.NewPayments, PrivateKey: private, PublicKey: public, APIv3Key: apiKey, PublicKeyID: c.PublicKeyID, SerialNumber: c.SerialNumber, NotifyURL: c.NotifyURL})
			if err != nil {
				return errors.New("invalid WeChat top-up configuration")
			}
			wechat = p
			wxNotify = p
		}
	}
	if err := service.EnableWalletTopUps(store, owner, alipay, wechat, policy, protection, auth); err != nil {
		return err
	}
	handler.SetPaymentNotifications(aliNotify, wxNotify)
	return nil
}
