package browser

import (
	"fmt"
	"task-processor/internal/core/config"
	sharedbrowser "task-processor/internal/crawler/shared/browser"
	"testing"

	"github.com/sirupsen/logrus"
)

// TestDifferentStrategies 测试不同的配置策略
func TestDifferentStrategies(t *testing.T) {
	logrus.SetLevel(logrus.InfoLevel)

	amazonConfig := &config.AmazonConfig{
		Headless:       true,
		BrowserPath:    "chrome/chrome.exe",
		ViewportWidth:  1920,
		ViewportHeight: 1080,
	}

	strategies := []string{"random", "stable", "windows", "preset"}
	presets := []string{"windows_high_end", "windows_mid_range", "mac_high_end"}

	for _, strategy := range strategies {
		for _, preset := range presets {
			t.Run(fmt.Sprintf("Strategy_%s_Preset_%s", strategy, preset), func(t *testing.T) {
				// 创建配置管理器
				configManager := NewConfigManager()

				// 生成配置
				browserConfig := configManager.GenerateBrowserConfig(amazonConfig, strategy, preset, 1)

				// 验证配置
				if browserConfig == nil {
					t.Fatal("生成的配置为空")
				}

				// 验证配置有效性
				if issues := sharedbrowser.ValidateConfig(browserConfig); len(issues) > 0 {
					t.Logf("配置验证问题: %v", issues)
				}

				// 记录配置信息
				t.Logf("策略: %s, 预设: %s", strategy, preset)
				t.Logf("平台: %s %s", browserConfig.FingerprintPlatform, browserConfig.FingerprintPlatformVersion)
				t.Logf("浏览器: %s %s", browserConfig.FingerprintBrand, browserConfig.FingerprintBrandVersion)
				t.Logf("GPU: %s - %s", browserConfig.FingerprintGPUVendor, browserConfig.FingerprintGPURenderer)
				t.Logf("语言: %s", browserConfig.Language)
				t.Logf("时区: %s", browserConfig.Timezone)
			})
		}
	}
}

// TestFingerprintGeneration 测试指纹生成
func TestFingerprintGeneration(t *testing.T) {
	generator := sharedbrowser.NewFingerprintGenerator()

	// 测试随机指纹生成
	t.Run("RandomFingerprint", func(t *testing.T) {
		fingerprint1 := generator.GenerateRandomFingerprint("")
		fingerprint2 := generator.GenerateRandomFingerprint("")

		if fingerprint1 == nil || fingerprint2 == nil {
			t.Fatal("生成的指纹为空")
		}

		// 验证指纹不同（随机性）
		if fmt.Sprintf("%v", fingerprint1.GPU) == fmt.Sprintf("%v", fingerprint2.GPU) {
			t.Log("注意: 两个随机指纹的GPU相同（可能是巧合）")
		}

		t.Logf("指纹1 GPU: %v", fingerprint1.GPU)
		t.Logf("指纹2 GPU: %v", fingerprint2.GPU)
	})

	// 测试稳定指纹生成
	t.Run("StableFingerprint", func(t *testing.T) {
		userID := "test_user_123"
		fingerprint1 := generator.GenerateStableFingerprint(userID)
		fingerprint2 := generator.GenerateStableFingerprint(userID)

		if fingerprint1 == nil || fingerprint2 == nil {
			t.Fatal("生成的指纹为空")
		}

		// 验证指纹相同（稳定性）
		if fmt.Sprintf("%v", fingerprint1.GPU) != fmt.Sprintf("%v", fingerprint2.GPU) {
			t.Fatal("相同用户ID的稳定指纹应该相同")
		}

		t.Logf("稳定指纹 GPU: %v", fingerprint1.GPU)
	})
}

// TestConfigManagerPresets 测试配置管理器预设
func TestConfigManagerPresets(t *testing.T) {
	configManager := NewConfigManager()

	// 获取可用预设
	presets := configManager.GetAvailablePresets()
	if len(presets) == 0 {
		t.Fatal("没有可用的预设配置")
	}

	t.Logf("可用预设: %v", presets)

	// 测试每个预设
	for _, presetName := range presets {
		t.Run(fmt.Sprintf("Preset_%s", presetName), func(t *testing.T) {
			info, err := configManager.GetPresetInfo(presetName)
			if err != nil {
				t.Fatalf("获取预设信息失败: %v", err)
			}

			t.Logf("预设 %s 信息: %+v", presetName, info)
		})
	}
}

// BenchmarkConfigGeneration 配置生成性能测试
func BenchmarkConfigGeneration(b *testing.B) {
	configManager := NewConfigManager()
	amazonConfig := &config.AmazonConfig{
		Headless:       true,
		BrowserPath:    "chrome/chrome.exe",
		ViewportWidth:  1920,
		ViewportHeight: 1080,
	}

	b.Run("RandomConfig", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = configManager.GenerateBrowserConfig(amazonConfig, "random", "", i)
		}
	})

	b.Run("StableConfig", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = configManager.GenerateBrowserConfig(amazonConfig, "stable", "", i)
		}
	})

	b.Run("PresetConfig", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = configManager.GenerateBrowserConfig(amazonConfig, "preset", "windows_high_end", i)
		}
	})
}
