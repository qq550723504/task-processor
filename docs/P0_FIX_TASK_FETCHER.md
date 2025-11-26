# P0 修复：任务获取策略优化

## 问题根源

当前系统的问题不是工作池容量不足，而是**任务获取策略不当**：

1. **一次性获取过多任务**：获取20个任务，但工作池只能容纳25个（5并发+20缓冲）
2. **获取间隔太短**：30秒间隔，但单个任务需要25-45秒处理
3. **缺少队列压力监控**：没有检查队列使用率就继续获取任务

## 解决方案

### 核心思路

**关键数据**：单个任务处理时间约 **60秒**

**当前配置**：
- 并发数：5个工作协程
- 缓冲区：20个槽位
- 获取间隔：30秒
- 理论吞吐量：5个任务/分钟

**问题分析**：
- 30秒获取一次，每次获取20个任务
- 但5个工作协程在30秒内只能处理 2.5 个任务（5 × 30/60）
- 导致队列快速填满，后续任务无法提交

**优化策略**：
- **保守获取**：只获取可用槽位的50%，避免一次性填满队列
- **压力监控**：队列使用率>80%时暂停获取
- **限制上限**：单次最多获取5个任务（匹配并发数）
- **增加间隔**：从30秒增加到90秒（1.5倍任务处理时间）

---

## 代码修改

### 1. 配置文件修改

```yaml
# config/config-temu-dev.yaml
worker:
  concurrency: 5           # 保持不变（5个工作协程）
  buffer_size: 20          # 保持不变（20个缓冲槽位）
  task_interval: 90        # 从30秒增加到90秒（1.5倍任务处理时间）
  max_fetch_per_cycle: 5   # 新增：单次最多获取5个（匹配并发数）
  queue_threshold: 75      # 新增：队列使用率阈值（更保守）
```

**配置说明**：
- `task_interval: 90`：每90秒获取一次任务
  - 单个任务需要60秒，90秒内5个工作协程可以处理 7.5 个任务
  - 留有余量，避免队列积压
  
- `max_fetch_per_cycle: 5`：单次最多获取5个任务
  - 匹配并发数，避免一次性填满队列
  - 5个任务需要60秒处理完，下次获取时队列已有空位
  
- `queue_threshold: 75`：队列使用率超过75%时暂停获取
  - 20个槽位 × 75% = 15个任务
  - 留有5个槽位的安全余量

### 2. 配置结构体更新

```go
// common/config/config.go

type WorkerConfig struct {
    Concurrency       int `yaml:"concurrency"`
    BufferSize        int `yaml:"buffer_size"`
    TaskInterval      int `yaml:"task_interval"`
    MaxFetchPerCycle  int `yaml:"max_fetch_per_cycle"`  // 新增
    QueueThreshold    int `yaml:"queue_threshold"`      // 新增
}
```

### 3. 工作池添加统计方法

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

// QueueStats 队列统计信息
type QueueStats struct {
    QueueSize      int     // 当前队列中的任务数
    BufferSize     int     // 队列总容量
    AvailableSlots int     // 可用槽位数
    UsagePercent   float64 // 使用率（%）
}
```

### 4. 任务提交器接口更新

```go
// common/task/interfaces.go

type TaskSubmitter interface {
    SubmitTask(taskData string) error
    GetAvailableSlots() int
    GetQueueStats() QueueStats  // 新增方法
}

// QueueStats 队列统计信息（与 worker.QueueStats 保持一致）
type QueueStats struct {
    QueueSize      int
    BufferSize     int
    AvailableSlots int
    UsagePercent   float64
}
```

### 5. 任务获取器核心优化

```go
// common/task/fetcher.go

func (f *UnifiedTaskFetcher) fetchAndDispatchTasks() {
    if f.managementClient == nil {
        logrus.Debug("管理客户端为空，跳过任务获取")
        return
    }

    // ========== 第1步：检查队列压力 ==========
    totalAvailableSlots := 0
    shouldSkipFetch := false
    
    for platform, submitter := range f.submitters {
        stats := submitter.GetQueueStats()
        totalAvailableSlots += stats.AvailableSlots
        
        logrus.Debugf("[%s] 队列状态: %d/%d (%.1f%%), 可用: %d", 
            platform, stats.QueueSize, stats.BufferSize, 
            stats.UsagePercent, stats.AvailableSlots)
        
        // 检查队列使用率
        threshold := float64(f.config.Worker.QueueThreshold)
        if threshold <= 0 {
            threshold = 80 // 默认80%
        }
        
        if stats.UsagePercent > threshold {
            logrus.Warnf("[%s] ⚠️ 队列使用率过高 (%.1f%% > %.1f%%)，暂停获取", 
                platform, stats.UsagePercent, threshold)
            shouldSkipFetch = true
        }
    }

    // ========== 第2步：决定是否跳过本次获取 ==========
    if shouldSkipFetch {
        logrus.Info("🛑 队列压力过大，本次跳过任务获取，等待队列消化")
        return
    }

    if totalAvailableSlots <= 0 {
        logrus.Debug("所有平台队列已满，跳过任务获取")
        return
    }

    // ========== 第3步：计算获取数量（保守策略）==========
    // 策略1：只获取可用槽位的50%
    maxTasks := totalAvailableSlots / 2
    
    // 策略2：应用配置的上限
    maxFetchPerCycle := f.config.Worker.MaxFetchPerCycle
    if maxFetchPerCycle <= 0 {
        maxFetchPerCycle = 5 // 默认5个（匹配并发数）
    }
    if maxTasks > maxFetchPerCycle {
        maxTasks = maxFetchPerCycle
    }
    
    // 策略3：至少获取1个
    if maxTasks < 1 {
        maxTasks = 1
    }

    logrus.Infof("📊 可用槽位: %d, 本次获取: %d (策略: 50%%, 上限: %d)", 
        totalAvailableSlots, maxTasks, maxFetchPerCycle)

    // ========== 第4步：获取任务 ==========
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

    // ========== 第5步：分发任务 ==========
    platformCounts := make(map[string]int)
    errorCount := 0
    queueFullCount := 0
    storeClient := f.managementClient.GetStoreClient()

    for _, apiTask := range apiTasks {
        storeInfo, err := storeClient.GetStore(apiTask.StoreID)
        if err != nil {
            logrus.Warnf("获取店铺信息失败: StoreID=%d, Error=%v", apiTask.StoreID, err)
            errorCount++
            continue
        }

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

        platform := storeInfo.Platform
        submitter, exists := f.submitters[platform]
        if !exists {
            logrus.Warnf("未找到平台处理器: TaskID=%s, Platform=%s",
                internalTask.ID, platform)
            errorCount++
            continue
        }

        if err := submitter.SubmitTask(string(taskData)); err != nil {
            if err.Error() == "工作队列已满" || err.Error() == "工作池已满" {
                logrus.Debugf("[%s] 队列已满，任务将重试: TaskID=%s", platform, internalTask.ID)
                queueFullCount++
            } else {
                logrus.Errorf("[%s] 提交失败: TaskID=%s, Error=%v", platform, internalTask.ID, err)
                errorCount++
            }
        } else {
            platformCounts[platform]++
        }
    }

    // ========== 第6步：输出统计 ==========
    logrus.Infof("✅ 任务分发完成: 成功=%v, 队列满=%d, 错误=%d", 
        platformCounts, queueFullCount, errorCount)
    
    if queueFullCount > 0 {
        logrus.Warnf("⚠️ 有 %d 个任务因队列满未提交，将在下次获取时重试", queueFullCount)
    }
}
```

---

## 实施步骤

### 步骤1：更新配置文件
```bash
# 编辑 config/config-temu-dev.yaml
# 添加新的配置项
```

### 步骤2：更新代码
```bash
# 1. 更新 common/config/config.go - 添加新字段
# 2. 更新 common/worker/pool.go - 添加 GetQueueStats 方法
# 3. 更新 common/task/interfaces.go - 添加 GetQueueStats 到接口
# 4. 更新 common/task/fetcher.go - 优化获取逻辑
```

### 步骤3：测试验证
```bash
# 1. 编译并运行
go build -o temu-web.exe ./cmd/temu-web

# 2. 观察日志，验证：
#    - 任务获取数量是否符合预期（<=10）
#    - 队列使用率是否被监控
#    - 队列满时是否暂停获取
```

---

## 预期效果

### 修复前（问题场景）
```
时间轴：单个任务需要 60 秒处理

14:37:15 - 获取 20 个任务
14:37:15 - 提交 5 个到工作协程（开始处理）
14:37:15 - 提交 15 个到缓冲区（队列：15/20）
14:37:45 - 再次获取 20 个任务（30秒后）
           此时只处理了 2.5 个任务（5 × 30/60）
           队列状态：17.5/20（几乎满了）
14:37:45 - 提交失败 15+ 个（队列满）❌
```

### 修复后（理想场景）
```
时间轴：单个任务需要 60 秒处理

14:37:15 - 检查队列：可用 25 个槽位（5工作+20缓冲）
14:37:15 - 获取 5 个任务（50% 策略，上限 5）
14:37:15 - 提交 5 个成功（全部进入工作协程）✅
           队列状态：5/20（25%）

14:38:45 - 90秒后再次检查
           此时已处理 7.5 个任务（5 × 90/60）
           队列状态：0/20（已清空）
14:38:45 - 获取 5 个任务
14:38:45 - 提交 5 个成功 ✅
           队列状态：5/20（25%）

14:40:15 - 90秒后再次检查
           队列状态：0/20（已清空）
14:40:15 - 继续获取 5 个任务 ✅

稳定运行：每90秒获取5个任务，队列使用率保持在 25% 左右
```

---

## 监控指标

修复后应该观察到：
- ✅ 队列满错误数量：0
- ✅ 任务提交成功率：100%
- ✅ 队列使用率：保持在 80% 以下
- ✅ 任务等待时间：显著减少

---

## 注意事项

1. **不要增加工作池容量**：5并发+20缓冲已经足够，问题在于获取策略

2. **保守策略的权衡**：
   - 优点：队列稳定，不会出现队列满的情况
   - 缺点：吞吐量略有下降（但更稳定）
   - 实际吞吐量：约 3.3 个任务/分钟（5个任务/90秒）

3. **可调整参数**（根据实际任务处理时间）：
   
   **如果任务处理时间 = 60秒**（当前情况）：
   - `task_interval: 90` （1.5倍任务时间）
   - `max_fetch_per_cycle: 5` （匹配并发数）
   - `queue_threshold: 75` （保守阈值）
   
   **如果任务处理时间 = 45秒**（优化后）：
   - `task_interval: 60` （1.3倍任务时间）
   - `max_fetch_per_cycle: 5-8`
   - `queue_threshold: 80`
   
   **如果任务处理时间 = 30秒**（大幅优化后）：
   - `task_interval: 45` （1.5倍任务时间）
   - `max_fetch_per_cycle: 8-10`
   - `queue_threshold: 80`

4. **计算公式**：
   ```
   理论吞吐量 = 并发数 / 任务处理时间
   安全获取间隔 = 任务处理时间 × 1.5
   安全获取数量 = 并发数（或更少）
   ```

---

## 后续优化方向

### 短期优化（提升吞吐量）
如果需要提高处理速度，应该优化任务处理时间，而不是增加获取频率：

1. **并行化图片下载**：当前可能是串行下载，改为并行可节省 10-20 秒
2. **优化 AI 调用**：
   - 缓存相似产品的 AI 响应
   - 批量处理多个变体
   - 使用更快的 AI 模型
3. **优化 Amazon 爬虫**：
   - 使用缓存的产品数据
   - 并行获取变体信息

**目标**：将任务处理时间从 60 秒降低到 30-40 秒

### 中期优化（智能调度）
1. **自适应获取间隔**：
   ```go
   // 根据实际处理速度动态调整
   avgProcessingTime := calculateAverageProcessingTime()
   nextInterval := avgProcessingTime * 1.5
   ```

2. **任务优先级队列**：
   - 高优先级任务优先处理
   - VIP 客户的任务优先级更高

3. **智能获取策略**：
   ```go
   // 根据队列状态和历史数据预测
   if queueUsage < 30% && recentSuccessRate > 95% {
       // 可以适当增加获取数量
       maxTasks = concurrency * 1.5
   }
   ```

### 长期优化（架构升级）
1. **分布式任务处理**：多个实例共同处理任务
2. **任务类型分离**：简单任务和复杂任务分开处理
3. **流式处理**：边获取边处理，减少等待时间
