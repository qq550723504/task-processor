package httpapi

import (
	"testing"
)

// TestRepositorySchemaBootstrapper_Injectable 验证 bootstrapper 可以被注入
// GREEN: 这个测试应该通过,证明可以替换全局 bootstrapper
func TestRepositorySchemaBootstrapper_Injectable(t *testing.T) {
	t.Run("can inject custom bootstrapper for schema migration", func(t *testing.T) {
		// 保存原始的 bootstrapper
		original := listingKitRepositorySchemaBootstrapper
		defer func() {
			// 恢复原始的 bootstrapper
			SetRepositorySchemaBootstrapper(original)
		}()

		// 创建自定义的 mock bootstrapper
		mockBootstrapper := newRepositorySchemaBootstrapper()

		// 注入 mock bootstrapper
		SetRepositorySchemaBootstrapper(mockBootstrapper)

		// 验证注入成功
		if listingKitRepositorySchemaBootstrapper != mockBootstrapper {
			t.Error("expected mock bootstrapper to be set")
		}

		t.Log("成功注入自定义 bootstrapper,不再硬编码依赖全局状态")
	})
}
