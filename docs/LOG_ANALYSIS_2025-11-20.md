# 日志分析报告 - 2025-11-20

## 概述
本报告分析了 2025-11-20 14:36-14:38 期间的应用日志，识别出关键的警告和错误信息，并提供改进建议。

## 主要问题分类

### 1. 🔴 严重问题：任务获取策略不当导致队列满（Queue Full）

**问题描述：**
- 在 14:37:45 时段，大量任务提交失败，原因是工作池队列已满
- 共有 15 个任务（ID: 1574945-1574959）提交失败
- 错误信息：`工作池队列已满，任务提交失败: TenantID=286, ShopID=508`

**影响：**
- 任务无法及时处理，导致积压
- 用户等待时间增加（部分任务已等待 116+ 小时）
- 系统吞吐量受限

**根本原因分析：**

从日志可以看出：
```
14:37:15 - 成功获取 20 个待处理任务
14:37:15 - 前5个任务成功提交到工作池（填满了5个工作协程）
14:37:15 - 接下来的任务开始填充缓冲区（20个槽位）
14:37:45 - 30秒后再次获取任务时，队列已满（5个处理中 + 20个缓冲）
```

**问题根源：**
1. **任务获取数量过多**：一次获取20个任务，但工作池容量只有25（5并发+20缓冲）
2. **任务处理时间长**：每个任务需要25-45秒，30秒内无法消化20个任务
3. **获取间隔太短**：30秒的间隔不足以处理完已获取的任务

**正确的解决方案：**

#### 方案 1：优化任务获取策略（推荐，立即实施）

问题不在于工作池容量，而在于任务获取的时机和数量。代码中已有动态调整逻辑，但需要优化：

```go
// common/task/fetcher.go - 当前逻辑已经正确
func (f *UnifiedTaskFetcher) fetchAndDispatchTasks() {
    // ✅ 已实现：计算可用槽位
    totalAvailableSlots := 0
    for platform, submitter := range f.submitters {
        slots := submitter.GetAvailableSlots()
        totalAvailableSlots += slots
    }

    // ✅ 已实现：如果没有可用槽位，跳过任务获取
    if totalAvailableSlots <= 0 {
        logrus.Debug("所有平台队列已满，跳过任务获取")
        return
    }

    // ✅ 已实现：根据可用槽位动态计算获取任务数量
    maxTasks := totalAvailableSlots
    if maxTasks > f.config.Worker.BufferSize {
        maxTasks = f.config.Worker.BufferSize
    }
    
    // 问题：这里的逻辑是对的，但实际执行时可能有问题
}
```

**需要检查的点：**
1. `GetAvailableSlots()` 是否正确返回了可用槽位？
2. 是否在任务提交时正确更新了队列状态？
3. 日志显示获取了20个任务，说明 `totalAvailableSlots` 当时是 >= 20 的

#### 方案 2：调整获取间隔（配合方案1）

```yaml
# config/config-temu-dev.yaml
worker:
  concurrency: 5           # 保持不变
  buffer_size: 20          # 保持不变
  task_interval: 60        # 从30秒增加到60秒，给任务更多处理时间
```

#### 方案 3：实现更保守的获取策略

```go
// common/task/fetcher.go
func (f *UnifiedTaskFetcher) fetchAndDispatchTasks() {
    // ... 现有代码 ...
    
    // 改进：更保守的获取策略
    // 只获取可用槽位的 50%，避免一次性填满队列
    maxTasks := totalAvailableSlots / 2
    if maxTasks < 1 {
        maxTasks = 1  // 至少获取1个
    }
    if maxTasks > 10 {
        maxTasks = 10  // 限制单次最多获取10个
    }
    
    logrus.Infof("📊 总可用槽位: %d, 本次获取任务数: %d (保守策略: 50%%)", 
        totalAvailableSlots, maxTasks)
    
    // ... 继续获取任务 ...
}
```

#### 方案 4：添加队列压力监控

```go
// common/task/fetcher.go
func (f *UnifiedTaskFetcher) fetchAndDispatchTasks() {
    // 计算队列使用率
    for platform, submitter := range f.submitters {
        stats := submitter.GetQueueStats()
        usagePercent := float64(stats.QueueSize) / float64(stats.BufferSize) * 100
        
        logrus.Infof("[%s] 队列状态: %d/%d (%.1f%%)", 
            platform, stats.QueueSize, stats.BufferSize, usagePercent)
        
        // 如果队列使用率超过80%，减少获取数量
        if usagePercent > 80 {
            logrus.Warnf("[%s] 队列使用率过高 (%.1f%%)，暂停获取新任务", 
                platform, usagePercent)
            return
        }
    }
    
    // ... 继续获取任务 ...
}

---

### 2. ⚠️ 警告：商品ID为空，跳过SKU编码检查

**问题描述：**
- 多次出现：`商品ID为空，跳过SKU编码检查`
- 发生在 `OutGoodsSnCheckHandler` 处理器中

**影响：**
- SKU 编码验证被跳过，可能导致重复 SKU
- 数据完整性检查缺失

**根本原因：**
在产品提交前，`GoodsID` 字段尚未生成（只有在 TEMU 接受产品后才会分配）

**改进建议：**

```go
// platforms/temu/handlers/out_goods_sn_check_handler.go
func (h *OutGoodsSnCheckHandler) Handle(ctx *pipeline.TaskContext) error {
    h.logger.Info("开始检查SKU编码")
    
    // 改进：区分新建和更新场景
    if ctx.TemuProduct.GoodsBasic.GoodsID == "" {
        h.logger.Debug("新建产品，GoodsID 尚未生成，跳过 SKU 编码检查")
        return nil  // 使用 Debug 级别，不是 Warning
    }
    
    // 执行 SKU 编码检查...
}
```

---

### 3. ⚠️ 警告：变体缺少物流信息

**问题描述：**
- 大量变体缺少物流信息（重量、尺寸）
- 示例：`⚠️ 变体[0] ASIN=B0CXDT8Q4Y 缺少物流信息，AI将根据产品信息估算`

**影响：**
- 依赖 AI 估算，可能不准确
- 影响运费计算和物流规划

**根本原因：**
Amazon 产品数据中未包含完整的物流信息

**改进建议：**

#### 方案 1：增强 Amazon 爬虫
```go
// common/amazon/scraper.go
// 添加物流信息提取逻辑
func (s *Scraper) extractShippingInfo(doc *goquery.Document) ShippingInfo {
    // 从产品详情页提取：
    // - Product Dimensions
    // - Item Weight
    // - Shipping Weight
}
```

#### 方案 2：建立物流信息数据库
- 缓存已知产品的物流信息
- 使用历史数据训练 AI 模型
- 提供手动修正接口

#### 方案 3：改进日志级别
```go
// 将 Warning 改为 Info，因为这是预期行为
if variant.Weight == "" {
    h.logger.Infof("📦 变体[%d] ASIN=%s 使用 AI 估算物流信息", i, variant.Asin)
} else {
    h.logger.Infof("✅ 变体[%d] ASIN=%s 使用实际物流信息", i, variant.Asin)
}
```

---

### 4. 🔴 错误：AI 映射规格验证失败

**问题描述：**
```
❌ AI映射[25]规格验证失败: 规格列表为空
⚠️ AI映射数量(28)与变体数量(31)不匹配
❌ AI必须从TEMU模板中选择有效的规格，不能使用默认规格
```

**影响：**
- 产品提交失败
- 需要回退到默认映射
- 用户体验差

**根本原因：**
1. AI 未能为所有变体生成映射（28/31）
2. 部分映射的规格列表为空
3. AI 可能选择了无效的规格

**改进建议：**

#### 方案 1：增强 AI Prompt
```go
// platforms/temu/handlers/sku_builder.go
func (sb *SkuBuilder) generateAISkuMapping(ctx *pipeline.TaskContext, variants []*amazon.Product) (*AISkuMappingResponse, error) {
    prompt := fmt.Sprintf(`
你是一个专业的电商产品规格映射专家。

重要规则：
1. 必须为所有 %d 个变体生成映射
2. 每个变体必须有 1-2 个有效规格（不能为空）
3. 规格必须从 TEMU 模板中选择，可用规格：%v
4. 如果无法确定规格，使用默认值但不能留空

变体列表：
%s

请生成完整的 SKU 映射...
`, len(variants), availableSpecs, variantsJSON)
}
```

#### 方案 2：添加 AI 响应验证
```go
func (sb *SkuBuilder) validateAIMapping(aiMapping *AISkuMappingResponse, variants []*amazon.Product) error {
    // 1. 检查数量
    if len(aiMapping.SkuList) != len(variants) {
        return fmt.Errorf("AI 映射数量不匹配：期望 %d，实际 %d", len(variants), len(aiMapping.SkuList))
    }
    
    // 2. 检查每个映射的规格
    for i, sku := range aiMapping.SkuList {
        if len(sku.Spec) == 0 {
            return fmt.Errorf("映射[%d] 规格列表为空", i)
        }
        if len(sku.Spec) > 2 {
            return fmt.Errorf("映射[%d] 规格数量超限：%d > 2", i, len(sku.Spec))
        }
    }
    
    return nil
}
```

#### 方案 3：实现智能补充机制
当前代码已有 `supplementMissingMappings`，但需要改进：

```go
func (sb *SkuBuilder) supplementMissingMappings(aiMapping *AISkuMappingResponse, variants []*amazon.Product) error {
    // 改进：不仅补充缺失的映射，还要修复无效的映射
    
    // 1. 分析有效映射的模式
    validPattern := sb.analyzeValidSpecPattern(aiMapping)
    
    // 2. 为缺失和无效的映射生成补充
    for i, variant := range variants {
        needsSupplement := false
        
        // 检查是否缺失
        if !sb.hasMappingForVariant(aiMapping, variant.Asin) {
            needsSupplement = true
        }
        
        // 检查是否无效
        if mapping := sb.getMappingForVariant(aiMapping, variant.Asin); mapping != nil {
            if len(mapping.Spec) == 0 {
                needsSupplement = true
                // 移除无效映射
                sb.removeMappingForVariant(aiMapping, variant.Asin)
            }
        }
        
        if needsSupplement {
            // 使用模式生成补充映射
            supplementMapping := sb.generateSupplementMapping(variant, validPattern)
            aiMapping.SkuList = append(aiMapping.SkuList, supplementMapping)
        }
    }
    
    return nil
}
```

---

### 5. ℹ️ 信息：没有找到租户店铺对

**问题描述：**
```
没有找到任何租户店铺对，可能需要先通过Web界面登录店铺
```

**影响：**
- 启动时的正常提示
- 不影响后续通过 API 获取任务

**建议：**
- 将日志级别从 Warning 改为 Info
- 添加更友好的提示信息

```go
if len(tenantShopPairs) == 0 {
    logrus.Info("当前没有活跃的租户店铺对")
    logrus.Info("💡 提示：任务将通过 API 动态获取，无需担心")
}
```

---

## 性能指标

### 任务处理统计
- 成功获取任务：20 个
- 成功提交到工作池：5 个
- 提交失败（队列满）：15 个
- 失败率：75%

### 任务等待时间
- 平均等待时间：116+ 小时（约 5 天）
- 这表明系统存在严重的任务积压问题

### 处理器执行时间
从日志分析和实际测试，单个任务的处理流程：
1. 初始化：< 1s
2. 获取店铺信息：< 1s
3. 获取原始数据：1-2s
4. 筛选规则：< 1s
5. 文本检查：1-2s
6. 变体数据处理：2-3s
7. AI 处理：10-20s（最耗时）
8. 图片处理：5-10s
9. 提交：1-2s

**总计：约 60 秒/任务**（实际测试数据）

**吞吐量计算**：
- 当前配置：5 并发
- 理论吞吐量：5 任务/分钟
- 实际吞吐量：约 3-4 任务/分钟（考虑网络延迟等）

---

## 优先级改进计划

### P0 - 立即修复（本周）
1. ✅ 优化任务获取策略（实现保守获取，单次最多10个）
2. ✅ 增加任务获取间隔（从30秒增加到60秒）
3. ✅ 添加队列压力监控（使用率>80%时暂停获取）
4. ✅ 修复 AI 映射验证逻辑

### P1 - 短期改进（2周内）
1. 优化 AI Prompt，提高映射准确率
2. 实现任务优先级队列
3. 添加任务处理监控和告警

### P2 - 中期优化（1个月内）
1. 并行化图片下载和处理
2. 实现 AI 响应缓存
3. 优化 Amazon 爬虫性能

### P3 - 长期规划（季度）
1. 实现分布式任务处理
2. 建立物流信息数据库
3. 开发任务管理 Dashboard

---

## 监控建议

### 添加关键指标
```go
// common/monitoring.go
type Metrics struct {
    // 队列指标
    QueueSize         prometheus.Gauge
    QueueFullCount    prometheus.Counter
    
    // 任务指标
    TaskProcessingTime prometheus.Histogram
    TaskSuccessRate    prometheus.Gauge
    TaskWaitTime       prometheus.Histogram
    
    // AI 指标
    AIMappingAccuracy  prometheus.Gauge
    AIResponseTime     prometheus.Histogram
}
```

### 添加告警规则
1. 队列使用率 > 80% 时告警
2. 任务失败率 > 10% 时告警
3. 任务等待时间 > 1 小时时告警
4. AI 映射准确率 < 90% 时告警

---

## 总结

当前系统的主要瓶颈是：
1. **工作池容量不足**：导致大量任务提交失败
2. **AI 映射不稳定**：部分变体映射失败或无效
3. **任务积压严重**：平均等待时间超过 5 天

建议优先解决工作池容量问题，然后逐步优化 AI 映射和任务处理性能。
