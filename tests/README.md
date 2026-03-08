# tests 目录

## 用途

存放集成测试、端到端测试等跨包的测试文件。单元测试应该放在对应的包目录中（`*_test.go`）。

## 目录结构

```
tests/
├── integration_test.go      # 集成测试
├── e2e_test.go              # 端到端测试（建议添加）
├── benchmark_test.go        # 性能基准测试（建议添加）
├── test_config.yaml         # 测试配置文件
├── fixtures/                # 测试数据（建议添加）
│   ├── products.json
│   └── tasks.json
└── mocks/                   # 测试 Mock（建议添加）
    └── mock_services.go
```

## 应该放置的文件

- 集成测试文件（`*_test.go`）
- 端到端测试文件
- 性能基准测试
- 测试配置文件
- 测试数据（fixtures）
- Mock 对象
- 测试辅助工具

## 测试文件命名规范

1. 单元测试：`{包名}_test.go`（放在对应包目录）
2. 集成测试：`integration_test.go`
3. 端到端测试：`e2e_test.go`
4. 基准测试：`benchmark_test.go`

## 测试代码示例

### 集成测试

```go
// integration_test.go
package tests

import (
    "context"
    "testing"
    
    "task-processor/internal/app/bootstrap"
    "github.com/stretchr/testify/assert"
)

func TestApplicationIntegration(t *testing.T) {
    // 加载测试配置
    app := bootstrap.NewApplicationBootstrap(logger)
    err := app.Initialize("test_config.yaml", "test")
    assert.NoError(t, err)
    
    // 启动应用
    ctx := context.Background()
    err = app.Start(ctx, "test")
    assert.NoError(t, err)
    
    // 执行测试
    // ...
    
    // 清理
    defer app.Stop(ctx)
}
```

### 端到端测试

```go
// e2e_test.go
package tests

import (
    "testing"
    "time"
)

func TestTaskProcessingE2E(t *testing.T) {
    // 1. 提交任务
    taskID := submitTask(t, taskData)
    
    // 2. 等待处理完成
    time.Sleep(5 * time.Second)
    
    // 3. 验证结果
    result := getTaskResult(t, taskID)
    assert.Equal(t, "completed", result.Status)
}
```

### 基准测试

```go
// benchmark_test.go
package tests

import "testing"

func BenchmarkTaskProcessing(b *testing.B) {
    setup()
    defer teardown()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        processTask(testTask)
    }
}
```

## 测试配置文件示例

```yaml
# test_config.yaml
app:
  name: task-processor-test
  
database:
  host: localhost
  port: 5432
  name: testdb
  
rabbitmq:
  host: localhost
  port: 5672
  
platforms:
  temu:
    enabled: true
    workers: 1
  shein:
    enabled: false
```

## 运行测试

```bash
# 运行所有测试
go test ./tests/...

# 运行集成测试
go test ./tests/ -run Integration

# 运行基准测试
go test ./tests/ -bench=. -benchmem

# 生成覆盖率报告
go test ./tests/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## 注意事项

- 测试应该是独立的，不依赖执行顺序
- 使用 `t.Cleanup()` 或 `defer` 清理测试资源
- 集成测试可能需要外部依赖（数据库、消息队列等）
- 使用测试容器（testcontainers）隔离测试环境
- 提供清晰的测试失败信息
- 避免测试中的硬编码值
- 使用表驱动测试处理多个测试用例
