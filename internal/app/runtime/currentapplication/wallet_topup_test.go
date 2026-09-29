package currentapplication

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWalletTopUpRequiresSeparateMoneyOwner(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(validManifest()), &cfg); err != nil {
		t.Fatal(err)
	}
	cfg.WalletTopUp.Alipay.Enabled = true
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "wallet top-up requires") {
		t.Fatalf("accepted missing owner: %v", err)
	}
	cfg.Identity.TenantDirectoryToken = "synthetic-directory-token"
	owner := cfg.SourceAccountDatabase
	owner.User = "money_owner_runtime"
	cfg.MoneyOwnerDatabase = &owner
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}
	cfg.MoneyOwnerDatabase.User = "referral_runtime"
	if err := cfg.validate(); err == nil || !strings.Contains(err.Error(), "money_owner_runtime") {
		t.Fatalf("accepted referral writer: %v", err)
	}
	cfg.MoneyOwnerDatabase.User = "money_owner_runtime"
	cfg.MoneyOwnerDatabase.Password = "invalid password"
	if err := cfg.validate(); err == nil {
		t.Fatal("accepted invalid database credential")
	}
}

func TestWalletTopUpManifestKeepsExplicitAmountPolicyAndSecretReferences(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(validManifest()), &cfg); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"walletTopUp":{"minMinor":100,"maxMinor":500000,"quickAmounts":["2000","8000"],"paymentWindowSeconds":600,"payloadKeyFile":"/private/topup-key","alipay":{"enabled":false,"newPayments":false}}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	core := cfg.CoreConfig()
	if core.WalletTopUp.MinMinor != 100 || core.WalletTopUp.MaxMinor != 500000 || len(core.WalletTopUp.QuickAmounts) != 2 || core.WalletTopUp.PayloadKeyFile != "/private/topup-key" {
		t.Fatalf("policy lost: %+v", core.WalletTopUp)
	}
}
