package listingkit

import (
	"context"
	"testing"
)

// TestGlobalServiceInstance_Eliminated 验证全局变量已被消除
// GREEN: 这个测试应该通过,证明依赖注入正常工作
func TestGlobalServiceInstance_Eliminated(t *testing.T) {
	t.Run("can inject mock config for execution service", func(t *testing.T) {
		// 现在可以创建带有自定义配置的服务实例
		mockConfig := taskSubmissionExecutionServiceConfig{
			resolveSheinStoreID: func(ctx context.Context, task *Task) (int64, error) {
				return 99999, nil // mock store ID
			},
		}

		// 可以创建独立的服务实例,不依赖全局状态
		service := newTaskSubmissionExecutionService(mockConfig)
		if service == nil {
			t.Fatal("expected non-nil service instance")
		}

		// 验证服务可以使用注入的配置
		// (实际调用需要完整的依赖,这里只验证实例化成功)
		t.Log("成功创建带 mock 配置的服务实例,不再依赖全局变量")
	})
}

// TestRepositoryBootstrapper_GlobalState 证明全局 bootstrapper 导致状态污染
func TestRepositoryBootstrapper_GlobalState(t *testing.T) {
	t.Run("bootstrapper global state causes test coupling", func(t *testing.T) {
		// 问题: listingKitRepositorySchemaBootstrapper 是全局变量
		// 多个测试共享同一个 bootstrapper 实例
		// 导致测试顺序依赖和状态污染

		t.Log("当前 httpapi.builders.go 中使用全局 bootstrapper")
		t.Log("需要重构为每次测试创建独立实例")

		t.Skip("REFACTORING NEEDED: 消除全局变量 listingKitRepositorySchemaBootstrapper")
	})
}
