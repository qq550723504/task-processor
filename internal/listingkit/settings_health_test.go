package listingkit

import (
	"strings"
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildSettingsHealthAllowsStoreSelectionAtSubmission(t *testing.T) {
	t.Parallel()

	health := BuildSettingsHealth(SettingsHealthInputs{
		Shein: &SheinSettings{Site: "US", DefaultStock: 20, DefaultSubmitMode: "publish"},
	})

	var account *SettingsHealthItem
	for i := range health.Items {
		if health.Items[i].Key == "shein.account" {
			account = &health.Items[i]
			break
		}
	}
	if account == nil {
		t.Fatalf("missing shein.account in %#v", health.Items)
	}
	if account.Status != "ready" {
		t.Fatalf("shein.account status = %q, want ready; message=%q action=%q", account.Status, account.Message, account.Action)
	}
	if !strings.Contains(strings.Join(append(account.Impact, account.Action), " "), "显式指定") {
		t.Fatalf("shein.account impact/action must explain explicit store selection: impact=%#v action=%q", account.Impact, account.Action)
	}
}

func TestBuildSettingsHealthReportsBlockedAndUnknownConfiguration(t *testing.T) {
	t.Parallel()

	health := BuildSettingsHealth(SettingsHealthInputs{
		DefaultAI: &AIClientSettings{
			ClientName: "default",
			Enabled:    true,
			BaseURL:    "https://api.example.test/v1",
		},
		ImageAI: &AIClientSettings{
			ClientName: "image",
			Enabled:    false,
			BaseURL:    "https://api.example.test/v1",
			Model:      "image-model",
			APIKeySet:  true,
		},
		Shein: &SheinSettings{
			Site:              "US",
			DefaultSubmitMode: "publish",
			Pricing: sheinpub.PricingRule{
				TargetCurrency: "USD",
				ExchangeRate:   0,
			},
		},
	})

	if health.Status != "blocked" {
		t.Fatalf("overall status = %q, want blocked", health.Status)
	}
	assertHealthItem(t, health, "ai.default", "blocked", "生成 ListingKit 草稿")
	assertHealthItem(t, health, "ai.image", "blocked", "图片生成与重绘")
	assertHealthItem(t, health, "shein.account", "blocked", "显式指定店铺的 SHEIN 提交")
	assertHealthItem(t, health, "shein.pricing", "blocked", "价格预览")
	assertHealthItem(t, health, "sds.session", "unknown", "SDS 属性补全")
	assertHealthItem(t, health, "storage.object", "unknown", "图片上传")
}

func TestBuildSettingsHealthReportsReadyConfiguration(t *testing.T) {
	t.Parallel()

	health := BuildSettingsHealth(SettingsHealthInputs{
		DefaultAI: &AIClientSettings{
			ClientName: "default",
			Enabled:    true,
			BaseURL:    "https://api.example.test/v1",
			Model:      "gpt-test",
			APIKeySet:  true,
		},
		ImageAI: &AIClientSettings{
			ClientName: "image",
			Enabled:    true,
			BaseURL:    "https://api.example.test/v1",
			Model:      "image-model",
			APIKeySet:  true,
		},
		Shein: &SheinSettings{
			Site:              "US",
			DefaultStock:      10,
			DefaultSubmitMode: "publish",
			Pricing: sheinpub.PricingRule{
				TargetCurrency:   "USD",
				ExchangeRate:     7.2,
				MarkupMultiplier: 1.4,
			},
		},
		Probes: SettingsHealthProbes{
			SheinIntegration: SettingsHealthProbe{Configured: true},
			SDSLogin:         SettingsHealthProbe{Configured: true},
			ObjectStorage:    SettingsHealthProbe{Configured: true},
		},
	})

	if health.Status != "ready" {
		t.Fatalf("overall status = %q, want ready", health.Status)
	}
	assertHealthItem(t, health, "ai.default", "ready", "生成 ListingKit 草稿")
	assertHealthItem(t, health, "ai.image", "ready", "图片生成与重绘")
	assertHealthItem(t, health, "shein.account", "ready", "显式指定店铺的 SHEIN 提交")
	assertHealthItem(t, health, "shein.integration", "ready", "保存草稿和发布")
	assertHealthItem(t, health, "shein.pricing", "ready", "价格预览")
	assertHealthItem(t, health, "sds.session", "ready", "SDS 属性补全")
	assertHealthItem(t, health, "storage.object", "ready", "图片上传")
}

func TestBuildSettingsHealthReportsBlockedRuntimeProbeConfiguration(t *testing.T) {
	t.Parallel()

	health := BuildSettingsHealth(SettingsHealthInputs{
		Probes: SettingsHealthProbes{
			SheinIntegration: SettingsHealthProbe{Missing: []string{"loginService.baseURL 缺失"}},
			SDSLogin:         SettingsHealthProbe{Missing: []string{"loginService.identifier 缺失"}},
			ObjectStorage:    SettingsHealthProbe{Missing: []string{"publisher.s3.bucket 缺失"}},
		},
	})

	if health.Status != "blocked" {
		t.Fatalf("overall status = %q, want blocked", health.Status)
	}
	assertHealthItem(t, health, "shein.integration", "blocked", "保存草稿和发布")
	assertHealthItem(t, health, "sds.session", "blocked", "SDS 属性补全")
	assertHealthItem(t, health, "storage.object", "blocked", "图片上传")

	for _, item := range health.Items {
		if item.Key == "shein.integration" && !strings.Contains(item.Action, "任务显式选择的目标店铺") {
			t.Fatalf("shein.integration action = %q, want explicit task target store wording", item.Action)
		}
	}
}

func assertHealthItem(t *testing.T, health SettingsHealthPage, key string, status string, impact string) {
	t.Helper()
	for _, item := range health.Items {
		if item.Key != key {
			continue
		}
		if item.Status != status {
			t.Fatalf("%s status = %q, want %q", key, item.Status, status)
		}
		for _, got := range item.Impact {
			if got == impact {
				return
			}
		}
		t.Fatalf("%s impact = %#v, want to include %q", key, item.Impact, impact)
	}
	t.Fatalf("missing health item %q in %#v", key, health.Items)
}
