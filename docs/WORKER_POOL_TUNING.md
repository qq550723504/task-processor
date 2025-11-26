# WorkerPool 性能调优指南

## 问题诊断

### 症状：工作池队列已满
```
level=warning msg="工作池队列已满，任务提交失败: TenantID=286, ShopID=508"
level=error msg="[TEMU] 提交任务失败: TaskID=1544751, Error=提交任务到工作池失败: 工作队列已满"
```

### 原因分析
1. **Worker 并发数太少** - 处理速度跟不上任务获取速度
2. **队列缓冲区太小** - 无法容纳足够的待处理任务
3. **任务处理时间长** - 单个任务耗时过长

## 配置优化

### 原始配置（有问题）
```yaml
worker:
  concurrency: 1      # ❌ 只有1个worker
  bufferSize: 1       # ❌ 队列太小
  taskInterval: 30
```

### 推荐配置

#### 开发环境（单worker调试）
```yaml
worker:
  concurrency: 1      # 1个worker，方便调试
  bufferSize: 5       # 队列容纳5个任务，避免队列满
  taskInterval: 30    # 30秒获取一次
```

#### 开发环境（多worker）
```yaml
worker:
  concurrency: 3      # 3个并发worker
  bufferSize: 10      # 队列容纳10个任务
  taskInterval: 30    # 30秒获取一次
```

#### 生产环境（轻负载）
```yaml
worker:
  concurrency: 5      # 5个并发worker
  bufferSize: 20      # 队列容纳20个任务
  taskInterval: 30
```

#### 生产环境（重负载）
```yaml
worker:
  concurrency: 10     # 10个并发worker
  bufferSize: 50      # 队列容纳50个任务
  taskInterval: 30
```

## 配置参数说明

### concurrency（并发数）
- **含义**: 同时处理任务的 worker 数量
- **影响**: 
  - 越大 = 处理速度越快，但消耗更多资源
  - 越小 = 资源占用少，但处理速度慢
- **建议**: 
  - 开发环境: 3-5
  - 生产环境: 5-20（根据服务器性能）

### bufferSize（缓冲区大小）
- **含义**: 任务队列可以容纳的任务数量
- **影响**:
  - 越大 = 可以缓存更多任务，但占用更多内存
  - 越小 = 内存占用少，但容易队列满
- **建议**:
  - 开发环境: 10-20
  - 生产环境: 20-100
- **计算公式**: `bufferSize >= concurrency * 2`

### taskInterval（任务获取间隔）
- **含义**: 多久获取一次新任务（秒）
- **影响**:
  - 越小 = 任务响应越快，但API调用频繁
  - 越大 = API调用少，但任务响应慢
- **建议**:
  - 开发环境: 30-60秒
  - 生产环境: 15-30秒

## 性能调优步骤

### 1. 监控当前状态
观察日志中的关键指标：
```
- 队列满的频率
- 任务处理时间
- Worker 空闲时间
```

### 2. 调整并发数
```yaml
# 如果经常看到"队列已满"
concurrency: 5  # 增加并发数

# 如果 CPU 使用率很高
concurrency: 3  # 减少并发数
```

### 3. 调整缓冲区
```yaml
# 如果经常看到"队列已满"
bufferSize: 20  # 增加缓冲区

# 如果内存占用过高
bufferSize: 10  # 减少缓冲区
```

### 4. 调整获取间隔
```yaml
# 如果任务积压严重
taskInterval: 15  # 更频繁地获取任务

# 如果任务很少
taskInterval: 60  # 减少API调用频率
```

## 优化后的改进

### 代码改进
1. **动态任务获取数量** ✅
   ```go
   // 根据并发数动态计算
   maxTasks := concurrency * 2
   ```

2. **队列满时的优雅处理** ✅
   ```go
   // 队列满时记录警告而不是错误
   // 任务会在下次获取时自动重试
   ```

3. **更好的日志** ✅
   ```go
   logrus.Warnf("工作池队列已满，任务将在下次获取时重试")
   ```

## 监控指标

### 关键指标
1. **队列使用率**: `当前队列长度 / bufferSize`
2. **Worker 利用率**: `忙碌的 worker / 总 worker`
3. **任务等待时间**: 从创建到开始处理的时间
4. **任务处理时间**: 单个任务的平均处理时间

### 理想状态
- 队列使用率: 30-70%
- Worker 利用率: 70-90%
- 任务等待时间: < 5分钟
- 队列满频率: < 5%

## 故障排查

### 问题1: 队列经常满
**症状**: 频繁看到"队列已满"警告

**解决方案**:
1. 增加 `concurrency`
2. 增加 `bufferSize`
3. 检查任务处理是否有性能问题

### 问题2: Worker 经常空闲
**症状**: 很少看到任务处理日志

**解决方案**:
1. 减少 `taskInterval`（更频繁获取）
2. 检查是否真的有待处理任务
3. 检查任务获取逻辑是否正常

### 问题3: 内存占用过高
**症状**: 程序内存使用持续增长

**解决方案**:
1. 减少 `bufferSize`
2. 减少 `concurrency`
3. 检查是否有内存泄漏

### 问题4: CPU 使用率过高
**症状**: CPU 持续 100%

**解决方案**:
1. 减少 `concurrency`
2. 优化任务处理逻辑
3. 增加 `taskInterval`

## 配置示例

### 场景1: 小型部署（单服务器）
```yaml
worker:
  concurrency: 3
  bufferSize: 10
  taskInterval: 30
```

### 场景2: 中型部署（专用服务器）
```yaml
worker:
  concurrency: 10
  bufferSize: 30
  taskInterval: 20
```

### 场景3: 大型部署（集群）
```yaml
worker:
  concurrency: 20
  bufferSize: 100
  taskInterval: 15
```

## 总结

### 快速修复
1. ✅ 增加 `concurrency` 到 3-5
2. ✅ 增加 `bufferSize` 到 10-20
3. ✅ 观察日志，根据实际情况调整

### 长期优化
1. 添加监控指标
2. 实现自动扩缩容
3. 优化任务处理性能

---

**当前配置已优化**:
- concurrency: 1 (保持单worker，方便调试) ✅
- bufferSize: 1 → 5 ✅
- 动态任务获取（根据bufferSize） ✅
- 优雅的队列满处理 ✅

**任务获取逻辑**:
- 一次获取数量 = bufferSize（当前为5个）
- 避免获取过多任务导致队列满
- 单worker也能稳定运行
