# 改进实施指南

## 1. 任务获取策略优化（核心修复）

### 1.1 配置文件修改

```yaml
# config/config-temu-dev.yaml
worker:
  concurrency: 5           # 保持不变，5个并发足够
  buffer_size: 20          # 保持不变，20个缓冲区足够
  task_interval: 60        # 从30秒增加到60秒，给任务更多处理时间
  max_fetch_per_cycle: 10  # 新增：单次最多获取10个任务
  queue_threshold: 80      # 新增：队列使用率阈值（%）
```

### 1.2 任务获取器代码优化

```go
// common/task/fetcher.go

func (f *UnifiedTaskFetcher) fetchAndDispatchTasks() {
    // 检查管理客户端
    if f.managementClient == nil {
        logrus.Debug("管理客户端为空，跳过任务获取")
        return
    }

    // 1. 计算所有平台的总可用槽位和队列压力
    totalAvailableSlots := 0
    shouldSkipFetch := false
    
    for platform, submitter := range f.submitters {
        stats := submitter.GetQueueStats()
        slots := stats.AvailableSlots
        usagePercent := stats.UsagePercent
        
        totalAvailableSlots += slots
        
        logrus.Debugf("[%s] 队列状态: %d/%d (%.1f%%), 可用槽位: %d", 
            platform, stats.QueueSize, stats.BufferSize, usagePercent, slots)
        
        // 如果任何平台的队列使用率超过阈值，暂停获取
        threshold := float64(f.config.Worker.QueueThreshold)
        if threshold <= 0 {
            threshold = 80 // 默认80%
        }
        
        if usagePercent > threshold {
            logrus.Warnf("[%s] ⚠️ 队列使用率过高 (%.1f%% > %.1f%%)，暂停获取新任务", 
                platform, usagePercent, threshold)
            shouldSkipFetch = true
        }
    }

    // 2. 如果队列压力过大，跳过本次获取
    if shouldSkipFetch {
        logrus.Info("🛑 队列压力过大，本次跳过任务获取，等待队列消化")
        return
    }

    // 3. 如果没有可用槽位，跳过任务获取
    if totalAvailableSlots <= 0 {
        logrus.Debug("所有平台队列已满，跳过任务获取")
        return
    }

    // 4. 根据可用槽位动态计算获取任务数量（保守策略）
    // 策略：只获取可用槽位的50%，避免一次性填满队列
    maxTasks := totalAvailableSlots / 2
    
    // 应用配置的最大获取数量限制
    maxFetchPerCycle := f.config.Worker.MaxFetchPerCycle
    if maxFetchPerCycle <= 0 {
        maxFetchPerCycle = 10 // 默认10个
    }
    
    if maxTasks > maxFetchPerCycle {
        maxTasks = maxFetchPerCycle
    }
    
    if maxTasks < 1 {
        maxTasks = 1 // 至少获取1个
    }

    logrus.Infof("📊 总可用槽位: %d, 本次获取任务数: %d (保守策略: 50%%, 上限: %d)", 
        totalAvailableSlots, maxTasks, maxFetchPerCycle)

    // 5. 获取任务
    importTaskClient := f.managementClient.GetImportTaskClient()
    apiTasks, err := importTaskClient.GetPendingAndRetryTasks(
        maxTasks,
        f.config.Management.UserID,
        f.config.Management.StoreIDs,
    )
    if err != nil {
        logrus.Errorf("获取任务失败: %v", err)
        return
    }

    if len(apiTasks) == 0 {
        logrus.Debug("没有待处理任务")
        return
    }

    logrus.Infof("📥 获取到 %d 个待处理任务", len(apiTasks))

    // 6. 分发任务（保持原有逻辑）
    platformCounts := make(map[string]int)
    errorCount := 0
    queueFullCount := 0
    storeClient := f.managementClient.GetStoreClient()

    for _, apiTask := range apiTasks {
        // 获取店铺信息判断平台
        storeInfo, err := storeClient.GetStore(apiTask.StoreID)
        if err != nil {
            logrus.Warnf("获取店铺信息失败: StoreID=%d, Error=%v", apiTask.StoreID, err)
            errorCount++
            continue
        }

        // 转换为内部任务格式
        internalTask := types.Task{
            ID:         fmt.Sprintf("%d", apiTask.ID),
            TenantID:   apiTask.TenantID,
            ProductID:  apiTask.ProductID,
            Platform:   apiTask.Platform,
            Region:     apiTask.Region,
            StoreID:    apiTask.StoreID,
            CategoryID: apiTask.CategoryID,
            CreateTime: apiTask.CreateTime,
            RetryCount: apiTask.RetryCount,
            Priority:   apiTask.Priority,
            Creator:    apiTask.Creator,
        }

        taskData, err := json.Marshal(internalTask)
        if err != nil {
            logrus.Errorf("序列化任务失败: %v", err)
            errorCount++
            continue
        }

        // 根据店铺平台分发任务
        platform := storeInfo.Platform
        submitter, exists := f.submitters[platform]
        if !exists {
            logrus.Warnf("未找到平台处理器: TaskID=%s, StoreID=%d, Platform=%s",
                internalTask.ID, apiTask.StoreID, platform)
            errorCount++
            continue
        }

        // 提交任务
        if err := submitter.SubmitTask(string(taskData)); err != nil {
            // 如果是队列满的错误，记录但不计入错误（任务会在下次获取时重试）
            if err.Error() == "工作队列已满" || err.Error() == "工作池已满" {
                logrus.Debugf("[%s] 队列已满，任务将在下次获取时重试: TaskID=%s", platform, internalTask.ID)
                queueFullCount++
            } else {
                logrus.Errorf("[%s] 提交任务失败: TaskID=%s, Error=%v", platform, internalTask.ID, err)
                errorCount++
            }
        } else {
            platformCounts[platform]++
            logrus.Debugf("[%s] 任务已提交: ID=%s, ProductID=%s", platform, internalTask.ID, internalTask.ProductID)
        }
    }

    // 7. 输出统计信息
    if len(platformCounts) > 0 || errorCount > 0 || queueFullCount > 0 {
        logrus.Infof("✅ 任务分发完成: 成功=%v, 队列满=%d, 错误=%d", 
            platformCounts, queueFullCount, errorCount)
    }
    
    // 8. 如果有任务因队列满而未提交，记录警告
    if queueFullCount > 0 {
        logrus.Warnf("⚠️ 有 %d 个任务因队列满未能提交，将在下次获取时重试", queueFullCount)
    }
}
```

### 1.3 工作池添加队列统计方法

```go
// common/worker/pool.go

// GetQueueStats 获取队列统计信息
func (p *Pool) GetQueueStats() QueueStats {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    queueLen := len(p.jobQueue)
    return QueueStats{
        QueueSize:      queueLen,
        BufferSize:     p.bufferSize,
        AvailableSlots: p.bufferSize - queueLen,
        UsagePercent:   float64(queueLen) / float64(p.bufferSize) * 100,
    }
}

type QueueStats struct {
    QueueSize      int
    BufferSize     int
    AvailableSlots int
    UsagePercent   float64
}
```

### 1.4 配置结构体更新

```go
// common/config/config.go

type WorkerConfig struct {
    Concurrency       int `yaml:"concurrency"`        // 并发工作协程数
    BufferSize        int `yaml:"buffer_size"`        // 队列缓冲区大小
    TaskInterval      int `yaml:"task_interval"`      // 任务获取间隔（秒）
    MaxFetchPerCycle  int `yaml:"max_fetch_per_cycle"` // 单次最多获取任务数
    QueueThreshold    int `yaml:"queue_threshold"`    // 队列使用率阈值（%）
}
```

---

## 2. SKU 编码检查优化

### 2.1 改进日志级别

```go
// platforms/temu/handlers/out_goods_sn_check_handler.go

func (h *OutGoodsSnCheckHandler) Handle(ctx *pipeline.TaskContext) error {
    h.logger.Info("开始检查SKU编码")
    
    // 区分新建和更新场景
    if ctx.TemuProduct.GoodsBasic.GoodsID == "" {
        h.logger.Debug("新建产品场景，GoodsID 尚未生成，跳过 SKU 编码检查")
        return nil
    }
    
    // 执行 SKU 编码检查逻辑...
    h.logger.Info("开始验证 SKU 编码唯一性")
    
    // ... 现有检查逻辑 ...
    
    h.logger.Info("SKU 编码检查完成")
    return nil
}
```

---

## 3. AI 映射增强

### 3.1 增强 Prompt

```go
// platforms/temu/handlers/sku_builder.go

func (sb *SkuBuilder) generateAISkuMapping(ctx *pipeline.TaskContext, variants []*amazon.Product) (*AISkuMappingResponse, error) {
    // 获取可用规格
    availableSpecs := sb.getAvailableSpecs(ctx)
    
    prompt := fmt.Sprintf(`你是一个专业的电商产品规格映射专家。

【重要规则】
1. 必须为所有 %d 个变体生成映射（一个都不能少）
2. 每个变体必须有 1-2 个有效规格（不能为空，不能超过2个）
3. 规格必须从 TEMU 模板中选择，可用规格如下：
%s

4. 如果变体属性不明确，使用以下默认规格：
   - 颜色相关：使用 "颜色" 规格
   - 尺寸相关：使用 "尺寸" 规格

【变体列表】（共 %d 个）
%s

【输出要求】
- 返回 JSON 格式
- sku_list 数组长度必须等于 %d
- 每个 SKU 的 spec 数组长度必须在 1-2 之间
- 所有字段都不能为空

请生成完整的 SKU 映射。`, 
        len(variants), 
        sb.formatAvailableSpecs(availableSpecs),
        len(variants),
        sb.formatVariants(variants),
        len(variants))
    
    // ... 调用 AI ...
}

func (sb *SkuBuilder) formatAvailableSpecs(specs []SpecInfo) string {
    var builder strings.Builder
    for i, spec := range specs {
        builder.WriteString(fmt.Sprintf("  %d. %s (ID: %s)\n", i+1, spec.SpecName, spec.SpecID))
    }
    return builder.String()
}
```

### 3.2 添加响应验证

```go
// platforms/temu/handlers/sku_builder.go

func (sb *SkuBuilder) validateAIMapping(aiMapping *AISkuMappingResponse, variants []*amazon.Product) error {
    errors := []string{}
    
    // 1. 检查数量
    if len(aiMapping.SkuList) != len(variants) {
        errors = append(errors, fmt.Sprintf(
            "映射数量不匹配：期望 %d 个，实际 %d 个", 
            len(variants), len(aiMapping.SkuList)))
    }
    
    // 2. 检查每个映射
    for i, sku := range aiMapping.SkuList {
        // 检查 ASIN
        if sku.Asin == "" {
            errors = append(errors, fmt.Sprintf("映射[%d] ASIN 为空", i))
        }
        
        // 检查规格数量
        if len(sku.Spec) == 0 {
            errors = append(errors, fmt.Sprintf("映射[%d] 规格列表为空", i))
        } else if len(sku.Spec) > 2 {
            errors = append(errors, fmt.Sprintf(
                "映射[%d] 规格数量超限：%d > 2", i, len(sku.Spec)))
        }
        
        // 检查规格有效性
        for j, spec := range sku.Spec {
            if spec.SpecID == "" {
                errors = append(errors, fmt.Sprintf(
                    "映射[%d] 规格[%d] SpecID 为空", i, j))
            }
            if spec.ParentSpecID == "" {
                errors = append(errors, fmt.Sprintf(
                    "映射[%d] 规格[%d] ParentSpecID 为空", i, j))
            }
        }
    }
    
    // 3. 检查 ASIN 唯一性
    asinMap := make(map[string]int)
    for i, sku := range aiMapping.SkuList {
        if prevIndex, exists := asinMap[sku.Asin]; exists {
            errors = append(errors, fmt.Sprintf(
                "ASIN 重复：%s 出现在映射[%d]和映射[%d]", 
                sku.Asin, prevIndex, i))
        }
        asinMap[sku.Asin] = i
    }
    
    if len(errors) > 0 {
        sb.logger.Errorf("❌ AI 映射验证失败，发现 %d 个问题：", len(errors))
        for _, err := range errors {
            sb.logger.Errorf("  - %s", err)
        }
        return fmt.Errorf("AI 映射验证失败：%s", strings.Join(errors, "; "))
    }
    
    sb.logger.Info("✅ AI 映射验证通过")
    return nil
}
```

### 3.3 改进补充机制

```go
// platforms/temu/handlers/sku_builder.go

func (sb *SkuBuilder) supplementMissingMappings(aiMapping *AISkuMappingResponse, variants []*amazon.Product) error {
    // 1. 分析有效映射的规格模式
    validPattern := sb.analyzeValidSpecPattern(aiMapping)
    if len(validPattern) == 0 {
        sb.logger.Warn("无法从现有映射中提取规格模式，使用默认规格")
        validPattern = sb.getDefaultSpecPattern(ctx)
    }
    
    // 2. 创建 ASIN 索引
    mappedAsins := make(map[string]int)
    for i, sku := range aiMapping.SkuList {
        mappedAsins[sku.Asin] = i
    }
    
    // 3. 处理缺失和无效的映射
    supplementCount := 0
    fixedCount := 0
    
    for _, variant := range variants {
        if index, exists := mappedAsins[variant.Asin]; exists {
            // 检查现有映射是否有效
            mapping := &aiMapping.SkuList[index]
            if len(mapping.Spec) == 0 {
                // 修复无效映射
                sb.logger.Warnf("修复无效映射：ASIN=%s", variant.Asin)
                mapping.Spec = sb.cloneSpecPattern(validPattern)
                fixedCount++
            }
        } else {
            // 添加缺失映射
            sb.logger.Warnf("补充缺失映射：ASIN=%s", variant.Asin)
            
            supplementMapping := AIGeneratedSku{
                UniqueID:          variant.Asin,
                Asin:              variant.Asin,
                Spec:              sb.cloneSpecPattern(validPattern),
                Weight:            "",  // 让后续 AI 估算
                Length:            "",
                Width:             "",
                Height:            "",
                VariantAttributes: sb.extractVariantAttributes(variant),
            }
            
            aiMapping.SkuList = append(aiMapping.SkuList, supplementMapping)
            supplementCount++
        }
    }
    
    if supplementCount > 0 || fixedCount > 0 {
        sb.logger.Warnf("⚠️ 映射修复完成：补充 %d 个，修复 %d 个", supplementCount, fixedCount)
        sb.logger.Warn("⚠️ 建议检查 AI 映射生成逻辑")
    }
    
    return nil
}

func (sb *SkuBuilder) cloneSpecPattern(pattern []types.SpecInfo) []types.SpecInfo {
    cloned := make([]types.SpecInfo, len(pattern))
    copy(cloned, pattern)
    return cloned
}

func (sb *SkuBuilder) extractVariantAttributes(variant *amazon.Product) map[string]string {
    attrs := make(map[string]string)
    
    // 从变体标题和属性中提取
    if variant.Color != "" {
        attrs["color"] = variant.Color
    }
    if variant.Size != "" {
        attrs["size"] = variant.Size
    }
    
    return attrs
}

func (sb *SkuBuilder) getDefaultSpecPattern(ctx *pipeline.TaskContext) []types.SpecInfo {
    // 返回最常用的默认规格
    return []types.SpecInfo{
        {
            SpecID:         "default_color",
            SpecName:       "颜色",
            ParentSpecID:   "color_parent",
            ParentSpecName: "颜色分类",
        },
    }
}
```

---

## 4. 物流信息处理优化

### 4.1 改进日志级别

```go
// platforms/temu/handlers/sku_builder.go

func (sb *SkuBuilder) buildSkcsFromAIMapping(...) {
    // ...
    
    for i, aiSku := range aiMapping.SkuList {
        // 检查物流信息完整性
        hasShippingInfo := aiSku.Weight != "" && aiSku.Length != "" && 
                          aiSku.Width != "" && aiSku.Height != ""
        
        if hasShippingInfo {
            sb.logger.Infof("✅ SKU[%d] 使用实际物流信息: weight=%s, length=%s, width=%s, height=%s",
                i, aiSku.Weight, aiSku.Length, aiSku.Width, aiSku.Height)
        } else {
            sb.logger.Infof("📦 SKU[%d] 使用 AI 估算物流信息（部分字段缺失）", i)
            
            // 记录缺失的字段
            missing := []string{}
            if aiSku.Weight == "" { missing = append(missing, "weight") }
            if aiSku.Length == "" { missing = append(missing, "length") }
            if aiSku.Width == "" { missing = append(missing, "width") }
            if aiSku.Height == "" { missing = append(missing, "height") }
            
            sb.logger.Debugf("   缺失字段: %v", missing)
        }
    }
}
```

---

## 5. 监控和告警

### 5.1 添加指标收集

```go
// common/monitoring/metrics.go

package monitoring

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // 队列指标
    QueueSize = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "worker_queue_size",
            Help: "当前队列中的任务数量",
        },
        []string{"platform"},
    )
    
    QueueUsagePercent = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "worker_queue_usage_percent",
            Help: "队列使用率（百分比）",
        },
        []string{"platform"},
    )
    
    QueueFullCount = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "worker_queue_full_total",
            Help: "队列满导致的任务提交失败次数",
        },
        []string{"platform"},
    )
    
    // 任务指标
    TaskProcessingDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "task_processing_duration_seconds",
            Help:    "任务处理耗时（秒）",
            Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600},
        },
        []string{"platform", "status"},
    )
    
    TaskWaitDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "task_wait_duration_seconds",
            Help:    "任务等待时间（秒）",
            Buckets: []float64{60, 300, 600, 1800, 3600, 7200, 14400, 28800},
        },
        []string{"platform"},
    )
    
    // AI 指标
    AIMappingAccuracy = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "ai_mapping_accuracy",
            Help: "AI 映射准确率（0-1）",
        },
        []string{"platform"},
    )
    
    AIResponseDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "ai_response_duration_seconds",
            Help:    "AI 响应耗时（秒）",
            Buckets: []float64{1, 2, 5, 10, 20, 30, 60},
        },
        []string{"platform", "operation"},
    )
)

// RecordQueueStats 记录队列统计
func RecordQueueStats(platform string, stats QueueStats) {
    QueueSize.WithLabelValues(platform).Set(float64(stats.QueueSize))
    QueueUsagePercent.WithLabelValues(platform).Set(stats.UsagePercent)
}

// RecordQueueFull 记录队列满事件
func RecordQueueFull(platform string) {
    QueueFullCount.WithLabelValues(platform).Inc()
}
```

### 5.2 在工作池中集成监控

```go
// common/worker/pool.go

func (p *Pool) Submit(job processor.WorkerJob) error {
    p.mu.RLock()
    defer p.mu.RUnlock()

    if p.closed {
        return ErrPoolClosed
    }

    // 记录队列统计
    stats := p.GetQueueStats()
    monitoring.RecordQueueStats(job.Platform, stats)

    select {
    case p.jobQueue <- job:
        return nil
    default:
        logrus.Warnf("工作池队列已满，任务提交失败: TenantID=%s, ShopID=%s", job.TenantID, job.ShopID)
        
        // 记录队列满事件
        monitoring.RecordQueueFull(job.Platform)
        
        return ErrQueueFull
    }
}
```

---

## 6. 配置建议

### 6.1 生产环境配置

```yaml
# config/config-temu-prod.yaml
worker:
  concurrency: 20          # 生产环境更高并发
  buffer_size: 100         # 更大的缓冲区
  retry_attempts: 5        # 更多重试次数
  retry_delay: 3           # 更长的重试延迟
  
monitoring:
  enabled: true
  port: 9090
  
alerts:
  queue_usage_threshold: 80      # 队列使用率告警阈值
  task_failure_rate_threshold: 10  # 任务失败率告警阈值（%）
  task_wait_time_threshold: 3600   # 任务等待时间告警阈值（秒）
```

### 6.2 开发环境配置

```yaml
# config/config-temu-dev.yaml
worker:
  concurrency: 5
  buffer_size: 20
  retry_attempts: 3
  retry_delay: 2
  
logging:
  level: debug             # 开发环境使用 debug 级别
  format: text             # 便于阅读
```

---

## 实施步骤

### 第一阶段（本周）
1. ✅ 修改配置文件，增加任务获取间隔和限制
2. ✅ 优化任务获取策略（保守获取，队列压力监控）
3. ✅ 添加 `GetQueueStats` 方法
4. ✅ 修复 SKU 编码检查的日志级别
5. ✅ 添加 AI 映射验证逻辑

### 第二阶段（下周）
1. 增强 AI Prompt
2. 改进补充映射机制
3. 添加基础监控指标
4. 优化物流信息日志

### 第三阶段（两周后）
1. 实现完整的监控系统
2. 添加告警规则
3. 性能测试和调优
4. 文档更新

---

## 测试计划

### 单元测试
```go
// common/worker/pool_test.go

func TestSubmitWithRetry(t *testing.T) {
    // 测试重试机制
}

func TestQueueStats(t *testing.T) {
    // 测试队列统计
}
```

### 集成测试
```go
// platforms/temu/handlers/sku_builder_test.go

func TestAIMappingValidation(t *testing.T) {
    // 测试 AI 映射验证
}

func TestSupplementMissingMappings(t *testing.T) {
    // 测试补充映射机制
}
```

### 压力测试
- 模拟 100 个并发任务
- 测试队列满时的重试机制
- 验证监控指标准确性
