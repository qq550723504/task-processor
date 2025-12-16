# 代码重构迁移指南

## 📋 概述

本指南帮助开发者从旧的重复代码架构迁移到新的优化架构。

## 🔄 主要变更

### 1. 类型定义更新

**旧代码:**
```go
var data map[string]interface{}
func SetData(key string, value interface{}) {}
```

**新代码:**
```go
var data map[string]any
func SetData(key string, value any) {}
```

**迁移步骤:**
- 全局替换 `interface{}` 为 `any`
- 更新所有相关的类型声明

### 2. 适配器架构重构

**旧代码:**
```go
type TemuProcessorAdapter struct {
    processor *temu.TemuProcessor
    logger    *logrus.Logger
    status    *dispatcher.ProcessorStatus
}

func (t *TemuProcessorAdapter) Start(ctx context.Context) error {
    t.logger.Info("启动处理器")
    // 大量重复的状态管理代码
    t.status.Status = "running"
    t.status.StartTime = time.Now()
    // ...
}
```

**新代码:**
```go
type TemuProcessorAdapter struct {
    *processor.BaseProcessorAdapter  // 继承基础功能
    processor *temu.TemuProcessor
}

func (t *TemuProcessorAdapter) Start(ctx context.Context) error {
    t.StartBase(ctx)  // 使用基础启动逻辑
    return t.processor.Start(ctx)
}
```

**迁移步骤:**
1. 修改适配器结构体，继承 `BaseProcessorAdapter`
2. 删除重复的字段定义
3. 使用基础方法替换重复逻辑
4. 只保留平台特定的逻辑

### 3. 处理器创建方式

**旧代码:**
```go
// 直接创建，重复的初始化逻辑
temuProcessor := &TemuProcessor{
    config: cfg,
    logger: logger,
    // ... 大量重复字段
}
```

**新代码:**
```go
// 方式1: 使用工厂
factory := processor.NewProcessorFactory()
temuProcessor, err := factory.CreateProcessor("temu", cfg, logger)

// 方式2: 使用构建器
temuProcessor, err := processor.NewProcessorBuilder("temu").
    WithConfig(cfg).
    WithLogger(logger).
    Build(factory)
```

## 🛠️ 分步迁移计划

### 第一步: 更新类型定义 (低风险)

1. **全局搜索替换**
   ```bash
   # 在项目根目录执行
   find . -name "*.go" -exec sed -i 's/interface{}/any/g' {} \;
   ```

2. **验证编译**
   ```bash
   go build ./...
   ```

### 第二步: 迁移适配器 (中风险)

1. **更新导入**
   ```go
   import (
       "task-processor/common/processor"
       // 其他导入...
   )
   ```

2. **修改结构体定义**
   ```go
   // 旧的
   type PlatformAdapter struct {
       processor SomeProcessor
       logger    *logrus.Logger
       status    *dispatcher.ProcessorStatus
   }
   
   // 新的
   type PlatformAdapter struct {
       *processor.BaseProcessorAdapter
       processor SomeProcessor
   }
   ```

3. **更新构造函数**
   ```go
   // 旧的
   func NewPlatformAdapter(proc SomeProcessor, logger *logrus.Logger) *PlatformAdapter {
       return &PlatformAdapter{
           processor: proc,
           logger:    logger,
           status:    &dispatcher.ProcessorStatus{...}, // 大量重复代码
       }
   }
   
   // 新的
   func NewPlatformAdapter(proc SomeProcessor, logger *logrus.Logger) *PlatformAdapter {
       return &PlatformAdapter{
           BaseProcessorAdapter: processor.NewBaseProcessorAdapter("platform", logger),
           processor:            proc,
       }
   }
   ```

4. **简化方法实现**
   ```go
   // 旧的 - 大量重复代码
   func (p *PlatformAdapter) Start(ctx context.Context) error {
       p.logger.Info("启动处理器")
       if err := p.processor.Start(ctx); err != nil {
           p.status.Status = "error"
           p.status.ErrorMessage = err.Error()
           return err
       }
       p.status.Status = "running"
       p.status.StartTime = time.Now()
       return nil
   }
   
   // 新的 - 简洁明了
   func (p *PlatformAdapter) Start(ctx context.Context) error {
       p.StartBase(ctx)
       return p.processor.Start(ctx)
   }
   ```

### 第三步: 使用工厂模式 (可选)

1. **创建工厂实例**
   ```go
   factory := processor.NewProcessorFactory()
   ```

2. **注册处理器创建器**
   ```go
   factory.RegisterCreator("temu", func(cfg *config.Config, logger *logrus.Logger, managementClient *management.ClientManager, sharedResources map[string]any) (processor.Processor, error) {
       return temu.NewTemuProcessor(cfg, logger), nil
   })
   ```

3. **使用工厂创建处理器**
   ```go
   processor, err := factory.CreateProcessor("temu", cfg, logger)
   ```

## ⚠️ 注意事项

### 兼容性考虑

1. **向后兼容**
   - 保留原有的构造函数
   - 新旧方法可以并存
   - 逐步迁移，不强制一次性更改

2. **测试覆盖**
   - 每个迁移步骤后运行完整测试
   - 确保功能不受影响
   - 性能测试验证

### 常见问题

1. **编译错误: 找不到字段**
   ```
   Error: t.logger undefined (cannot refer to unexported field logger)
   ```
   **解决方案**: 删除直接访问logger的代码，使用基础方法

2. **方法签名不匹配**
   ```
   Error: method signature mismatch
   ```
   **解决方案**: 检查接口定义，确保实现了正确的方法

3. **循环导入**
   ```
   Error: import cycle not allowed
   ```
   **解决方案**: 重新组织包结构，避免循环依赖

## 🧪 测试策略

### 单元测试

```go
func TestAdapterMigration(t *testing.T) {
    // 测试新适配器功能
    adapter := NewTemuProcessorAdapter(mockProcessor, logger)
    
    // 验证基础功能
    assert.NotNil(t, adapter.BaseProcessorAdapter)
    
    // 验证平台特定功能
    assert.Equal(t, "temu", adapter.GetPlatformName())
}
```

### 集成测试

```go
func TestFactoryIntegration(t *testing.T) {
    factory := processor.NewProcessorFactory()
    
    // 注册创建器
    registerCreators(factory)
    
    // 测试创建
    proc, err := factory.CreateProcessor("temu", cfg, logger)
    assert.NoError(t, err)
    assert.NotNil(t, proc)
}
```

## 📊 迁移检查清单

- [ ] 所有 `interface{}` 已替换为 `any`
- [ ] 适配器继承了 `BaseProcessorAdapter`
- [ ] 删除了重复的状态管理代码
- [ ] 使用基础方法替换重复逻辑
- [ ] 编译通过，无错误
- [ ] 单元测试通过
- [ ] 集成测试通过
- [ ] 性能测试无回归
- [ ] 代码审查完成
- [ ] 文档更新完成

## 🚀 迁移后的优势

1. **代码减少**: 适配器代码减少约60%
2. **维护性**: 统一的错误处理和状态管理
3. **扩展性**: 新平台接入更容易
4. **一致性**: 所有平台使用相同的基础架构
5. **测试性**: 更容易编写和维护测试

## 📞 支持

如果在迁移过程中遇到问题，请：

1. 查看本指南的常见问题部分
2. 参考 `examples/processor_usage_example.go` 中的示例
3. 查看单元测试了解正确用法
4. 联系开发团队获取支持

---

**记住**: 迁移是一个渐进的过程，不需要一次性完成所有更改。优先处理高频使用的代码，逐步完善整个系统。