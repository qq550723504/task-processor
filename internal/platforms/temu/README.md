# TEMU平台产品同步调用链架构

## 架构概览

TEMU平台产品同步功能采用分层架构设计，参考SHEIN平台实现，确保代码的可维护性和扩展性。

## 目录结构

```
internal/platforms/temu/
├── api/
│   └── product/
│       └── types.go                    # TEMU API数据类型定义
├── repo/
│   ├── interfaces.go                   # 数据访问层接口定义
│   └── client/
│       └── client_manager.go           # 客户端管理器
├── service/
│   └── scheduler/
│       ├── product_sync_types.go       # 产品同步服务接口
│       ├── inventory_sync_types.go     # 库存同步服务接口
│       ├── price_sync_types.go         # 价格同步服务接口
│       └── factory.go                  # 服务工厂
└── scheduler/
    └── product_task.go                 # 产品同步任务实现
```

## 调用链流程

### 1. 任务层 (Task Layer)
- **文件**: `scheduler/product_task.go`
- **职责**: 任务调度和执行控制
- **主要组件**: `ProductSyncTask`

### 2. 服务层 (Service Layer)
- **文件**: `service/scheduler/product_sync_types.go`
- **职责**: 业务逻辑处理
- **主要接口**: `ProductSyncService`
- **核心方法**:
  - `FetchProductList()` - 获取产品列表
  - `ConvertProducts()` - 转换产品格式
  - `SaveProducts()` - 保存产品数据

### 3. 数据访问层 (Repository Layer)
- **文件**: `repo/interfaces.go`
- **职责**: 外部API调用和数据管理
- **主要接口**:
  - `ProductAPIInterface` - 产品API接口
  - `InventoryManager` - 库存管理接口
  - `PriceManager` - 价格管理接口

### 4. API层 (API Layer)
- **文件**: `api/product/types.go`
- **职责**: TEMU API数据结构定义
- **主要类型**:
  - `ProductListRequest/Response` - 产品列表请求/响应
  - `ProductListItem` - 产品信息
  - `InventoryInfo` - 库存信息
  - `PriceInfo/CostInfo` - 价格/成本信息

## 执行流程

```
ProductSyncTask.Execute()
    ↓
ProductSyncService.FetchProductList()
    ↓
ProductAPIInterface.ListProducts()
    ↓
ProductSyncService.ConvertProducts()
    ↓
InventoryManager.GetInventoryInfo()
PriceManager.GetPriceInfo()
    ↓
ProductSyncService.SaveProducts()
    ↓
ManagementClient.BatchCreateOrUpdate()
```

## 扩展功能

### 库存同步
- **接口**: `InventorySyncService`
- **配置**: `InventorySyncConfig`
- **支持批量更新和自动同步**

### 价格同步
- **接口**: `PriceSyncService`
- **配置**: `PriceSyncConfig`
- **支持多种定价策略**

## 依赖注入

通过`ServiceFactory`统一管理服务创建，支持依赖注入和配置管理。

## 待实现

所有接口和类型已定义完成，具体实现需要根据TEMU API文档进行开发：

1. 实现`ProductAPIInterface`的具体API调用
2. 实现`ProductSyncService`的业务逻辑
3. 实现库存和价格管理器
4. 添加错误处理和重试机制
5. 添加监控和日志记录