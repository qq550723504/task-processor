package browser

import (
	"testing"
)

func TestAddFingerprintArgs(t *testing.T) {
	// 测试空指纹
	args := []string{"--test"}
	result := AddFingerprintArgs(args, nil)
	if len(result) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(result))
	}

	// 测试禁用的指纹
	fingerprint := &FingerprintConfig{Enable: false}
	result = AddFingerprintArgs(args, fingerprint)
	if len(result) != 1 {
		t.Errorf("Expected 1 arg, got %d", len(result))
	}

	// 测试启用的指纹
	fingerprint = &FingerprintConfig{
		Enable: true,
		GPU: map[string]string{
			"vendor": "test",
		},
	}
	result = AddFingerprintArgs(args, fingerprint)
	if len(result) != 2 {
		t.Errorf("Expected 2 args, got %d", len(result))
	}

	// 验证参数格式
	found := false
	for _, arg := range result {
		if len(arg) > 15 && arg[:15] == "--kfingerprint=" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected --kfingerprint argument not found")
	}
}
