# TEMU 自动核价功能实现总结

## 实现内容

参考SHEIN的自动核价实现，为TEMU平台添加了自动拒绝平台报价的功能。

## 新增文件

### 核心功能
1. **common/temu/pricing_api.go** - 核价API接口实现
   - `GetPendingPriceList()` - 获取待核价列表
   - `RejectPrice()` - 拒绝平台报价
   - `AutoRejectAllPendingPrices()` - 自动拒绝所有待核价商品

2. **common/temu/pricing_types.go** - 数据类型定义
   - `PendingPriceListRequest/Response` - 待核价列表
   - `RejectPriceRequest/Response` - 拒绝报价

3. **common/temu/scheduler.go** - 单店铺调度器
   - 定时任务调度
   - Panic恢复机制
   - 优雅启动/停止

4. **common/temu/scheduler_manager.go** - 多店铺管理器
   - 动态添加/移除店铺
   - 统一生命周期管理

### 命令行工具
5. **cmd/temu-pricing/main.go** - 独立运行程序
   - 支持单店铺/多店铺模式
   - 优雅退出处理
   - 命令行参数配置

### 测试和文档
6. **common/temu/pricing_api_test.go** - 单元测试
7. **common/temu/README.md** - 模块使用说明
8. **cmd/temu-pricing/README.md** - 命令行工具说明
9. **docs/TEMU_AUTO_PRICING.md** - 详细功能文档

### 配置更新
10. **config/config-dev.yaml** - 添加TEMU自动核价配置

## API接口说明

### 1. 获取待核价列表
```
POST /mms/marigold/sku/v2/search
```
- 分页获取待核价商品
- `sku_search_type: 2` 表示待核价状态

### 2. 拒绝平台报价
```
POST /mms/marigold/sku/offline
```
- 拒绝单个商品的平台报价
- `operation_source: 1005` 表示价格健康页面

### 3. 重新报价
```
POST /mms/marigold/price/appeal/order/create
```
- 提交新的报价申诉
- `appeal_source: 100` 表示价格健康页面
- 支持多个申诉原因（如 "LOWER_THAN_SIMILAR"）

## 使用方式

### 方式1: 命令行工具
```bash
# 单店铺
go run cmd/temu-pricing/main.go -tenant=1001 -store=2001 -interval=30m

# 多店铺
go run cmd/temu-pricing/main.go -interval=30m
```

### 方式2: 代码集成
```go
managementClient := management.NewClientManager(nil)
schedulerManager := temu.NewSchedulerManager(managementClient, 30*time.Minute)
schedulerManager.AddStore(1001, 2001)
defer schedulerManager.StopAll()
```

## 核心特性

1. **自动化处理**
   - 定时获取待核价列表
   - 自动拒绝所有待核价商品
   - 支持分页处理大量数据

2. **错误处理**
   - Cookie过期自动重新加载
   - 网络错误自动重试（3次）
   - Panic恢复机制

3. **多店铺支持**
   - 统一管理多个店铺
   - 独立调度器实例
   - 动态添加/移除

4. **优雅退出**
   - 支持SIGINT/SIGTERM信号
   - 30秒超时保护
   - 确保任务完整性

## 配置示例

```yaml
temu:
  autoPricing:
    enabled: true
    interval: 1800  # 30分钟
    batchSize: 100
```

## 注意事项

1. **当前实现为自动拒绝所有待核价商品**
2. 需要先在管理系统中配置店铺Cookie
3. 建议间隔时间设置为30分钟以上
4. 生产环境建议使用systemd管理进程

## 支持的操作

1. **获取待核价列表** - 分页获取所有待核价商品
2. **拒绝平台报价** - 拒绝单个商品的平台报价
3. **重新报价** - 提交新的价格申诉
4. **自动拒绝所有待核价** - 批量自动拒绝

## 后续优化方向

1. 添加智能核价规则（类似SHEIN）
2. 支持接受报价功能
3. 添加价格分析和统计
4. 集成到主程序的Pipeline中
5. 添加更详细的监控指标
6. 实现自动重新报价逻辑（基于规则计算目标价格）

## 测试

```bash
# 运行单元测试
go test ./common/temu -v

# 编译命令行工具
go build -o temu-pricing.exe cmd/temu-pricing/main.go

# 运行
./temu-pricing.exe -tenant=1001 -store=2001 -interval=1h
```
