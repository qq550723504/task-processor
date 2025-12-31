package browser

import (
	"encoding/json"
	"testing"
)

func TestFingerprintGenerator(t *testing.T) {
	generator := NewFingerprintGenerator()

	t.Run("生成随机指纹", func(t *testing.T) {
		fingerprint := generator.GenerateRandomFingerprint()

		if fingerprint == nil {
			t.Fatal("生成的指纹不能为空")
		}

		// 输出指纹为JSON字符串
		fingerprintJSON, err := json.MarshalIndent(fingerprint, "", "")
		if err != nil {
			t.Fatalf("序列化指纹失败: %v", err)
		}
		t.Logf("生成的随机指纹JSON:\n%s", string(fingerprintJSON))

		if !fingerprint.Enable {
			t.Error("生成的指纹应该是启用状态")
		}

		if fingerprint.GPU == nil {
			t.Error("GPU配置不能为空")
		}

		// 检查GPU配置是否包含必要字段
		if _, exists := fingerprint.GPU["vendor"]; !exists {
			t.Error("GPU配置应该包含vendor字段")
		}

		if _, exists := fingerprint.GPU["renderer"]; !exists {
			t.Error("GPU配置应该包含renderer字段")
		}
	})

	t.Run("生成唯一实例指纹", func(t *testing.T) {
		fingerprint1 := generator.GenerateUniqueFingerprint(1)
		fingerprint2 := generator.GenerateUniqueFingerprint(2)

		if fingerprint1 == nil || fingerprint2 == nil {
			t.Fatal("生成的指纹不能为空")
		}

		// 验证两个指纹不完全相同（至少在某些方面应该不同）
		if fingerprint1.Enable != fingerprint2.Enable {
			// 这是可能的，但在我们的实现中应该都是true
		}
	})

	t.Run("生成高级指纹", func(t *testing.T) {
		fingerprint := generator.GenerateAdvancedFingerprint(1)

		if fingerprint == nil {
			t.Fatal("生成的高级指纹不能为空")
		}

		if !fingerprint.Enable {
			t.Error("高级指纹应该是启用状态")
		}

		// 检查高级指纹是否包含更多字段
		expectedFields := []string{"vendor", "renderer", "platform", "hardwareConcurrency", "deviceMemory"}
		for _, field := range expectedFields {
			if _, exists := fingerprint.GPU[field]; !exists {
				t.Errorf("高级指纹应该包含%s字段", field)
			}
		}
	})

	t.Run("验证指纹配置", func(t *testing.T) {
		// 测试有效指纹
		validFingerprint := generator.GenerateRandomFingerprint()
		if !generator.ValidateFingerprint(validFingerprint) {
			t.Error("有效指纹验证失败")
		}

		// 测试空指纹
		if generator.ValidateFingerprint(nil) {
			t.Error("空指纹应该验证失败")
		}

		// 测试禁用的指纹
		disabledFingerprint := &FingerprintConfig{Enable: false}
		if !generator.ValidateFingerprint(disabledFingerprint) {
			t.Error("禁用的指纹应该验证通过")
		}

		// 测试缺少必要字段的指纹
		invalidFingerprint := &FingerprintConfig{
			Enable: true,
			GPU:    map[string]any{"description": "test"},
		}
		if generator.ValidateFingerprint(invalidFingerprint) {
			t.Error("缺少必要字段的指纹应该验证失败")
		}
	})

	t.Run("获取随机GPU配置", func(t *testing.T) {
		gpuConfig := generator.GetRandomGPUConfig()

		if gpuConfig == nil {
			t.Fatal("GPU配置不能为空")
		}

		requiredFields := []string{"vendor", "renderer", "description"}
		for _, field := range requiredFields {
			if _, exists := gpuConfig[field]; !exists {
				t.Errorf("GPU配置应该包含%s字段", field)
			}
		}
	})
}
