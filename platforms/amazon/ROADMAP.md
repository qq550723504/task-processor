# Amazon 平台模块开发路线图

## 📍 当前状态

**版本**: v1.0.0  
**完成度**: 95%  
**状态**: ✅ 基础框架完成，可投入使用

---

## 🎯 下一步工作计划

### 阶段 1: 核心功能完善（必须完成）⭐⭐⭐

#### 1.1 产品属性映射逻辑 ⏳

**优先级**: 🔴 高  
**预计工时**: 3-5 天  
**负责模块**: `handlers/`, `utils/`

**任务清单**:
- [ ] 创建属性映射配置文件
  ```yaml
  # config/amazon_attributes.yaml
  product_types:
    PRODUCT:
      required:
        - item_name
        - brand
        - manufacturer
      optional:
        - color
        - size
  ```

- [ ] 实现属性映射器
  ```go
  // utils/attribute_mapper.go
  type AttributeMapper struct {
      config *AttributeConfig
  }
  
  func (m *AttributeMapper) MapAttributes(
      sourceData map[string]interface{},
      productType string,
  ) (map[string]interface{}, error)
  ```

- [ ] 添加属性验证
  ```go
  // utils/attribute_validator.go
  func ValidateAttributes(
      attrs map[string]interface{},
      productType string,
  ) error
  ```

- [ ] 创建属性映射处理器
  ```go
  // handlers/attribute_mapper_handler.go
  type AttributeMapperHandler struct {
      mapper *utils.AttributeMapper
  }
  ```

**文件结构**:
```
platforms/amazon/
├── config/
│   └── attribute_mapping.yaml    # 新增
├── utils/
│   ├── attribute_mapper.go       # 新增
│   └── attribute_validator.go    # 新增
└── handlers/
    └── attribute_mapper_handler.go # 新增
```

---

#### 1.2 图片上传功能 ⏳

**优先级**: 🔴 高  
**预计工时**: 4-6 天  
**负责模块**: `service/`, `api/`

**任务清单**:
- [ ] 实现图片下载器
  ```go
  // service/image_downloader.go
  type ImageDownloader struct {
      httpClient *http.Client
  }
  
  func (d *ImageDownloader) Download(
      ctx context.Context,
      url string,
  ) ([]byte, error)
  ```

- [ ] 实现图片处理器
  ```go
  // service/image_processor.go
  type ImageProcessor struct{}
  
  func (p *ImageProcessor) Resize(
      image []byte,
      width, height int,
  ) ([]byte, error)
  
  func (p *ImageProcessor) ValidateFormat(
      image []byte,
  ) error
  ```

- [ ] 实现 S3 上传器
  ```go
  // service/s3_uploader.go
  type S3Uploader struct {
      s3Client *s3.Client
      bucket   string
  }
  
  func (u *S3Uploader) Upload(
      ctx context.Context,
      key string,
      data []byte,
  ) (string, error)
  ```

- [ ] 创建图片处理 Handler
  ```go
  // handlers/image_handler.go
  type ImageHandler struct {
      downloader *service.ImageDownloader
      processor  *service.ImageProcessor
      uploader   *service.S3Uploader
  }
  ```

**文件结构**:
```
platforms/amazon/
├── service/
│   ├── image_downloader.go    # 新增
│   ├── image_processor.go     # 新增
│   └── s3_uploader.go         # 新增
└── handlers/
    └── image_handler.go       # 新增
```

**依赖**:
```bash
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/disintegration/imaging
```

---

#### 1.3 单元测试 ⏳

**优先级**: 🟡 中  
**预计工时**: 5-7 天  
**目标覆盖率**: 80%+

**任务清单**:
- [ ] API 客户端测试
  ```go
  // api/auth_test.go
  func TestAuthManager_GetAccessToken(t *testing.T)
  func TestAuthManager_RefreshToken(t *testing.T)
  
  // api/listings_test.go
  func TestClient_CreateListing(t *testing.T)
  func TestClient_UpdateListing(t *testing.T)
  ```

- [ ] 服务层测试
  ```go
  // service/listing_service_test.go
  func TestListingService_CreateListing(t *testing.T)
  
  // service/inventory_service_test.go
  func TestInventoryService_UpdateInventory(t *testing.T)
  ```

- [ ] 工具类测试
  ```go
  // utils/validator_test.go
  func TestValidator_ValidateTitle(t *testing.T)
  func TestValidator_ValidatePrice(t *testing.T)
  ```

- [ ] 集成测试
  ```go
  // integration_test.go
  func TestAmazonProcessor_EndToEnd(t *testing.T)
  ```

**文件结构**:
```
platforms/amazon/
├── api/
│   ├── auth_test.go           # 新增
│   ├── listings_test.go       # 新增
│   └── request_test.go        # 新增
├── service/
│   ├── listing_service_test.go    # 新增
│   └── inventory_service_test.go  # 新增
├── utils/
│   ├── validator_test.go      # 新增
│   └── converter_test.go      # 新增
└── integration_test.go        # 新增
```

---

### 阶段 2: 功能增强（可选）⭐⭐

#### 2.1 变体产品支持 ⏳

**优先级**: 🟡 中  
**预计工时**: 5-7 天

**任务清单**:
- [ ] 扩展数据模型
  ```go
  // models.go
  type VariationTheme struct {
      Name       string
      Attributes []string
  }
  
  type VariantProduct struct {
      ParentASIN string
      ChildASINs []string
      Theme      VariationTheme
      Variants   []Variant
  }
  ```

- [ ] 实现变体处理器
  ```go
  // handlers/variant_handler.go
  type VariantHandler struct {
      apiClient *api.Client
  }
  
  func (h *VariantHandler) CreateVariants(
      ctx *TaskContext,
  ) error
  ```

- [ ] 添加变体 API
  ```go
  // api/variants.go
  func (c *Client) CreateVariantListing(
      ctx context.Context,
      req *VariantListingRequest,
  ) (*ListingResponse, error)
  ```

---

#### 2.2 批量上架功能 ⏳

**优先级**: 🟡 中  
**预计工时**: 4-6 天

**任务清单**:
- [ ] 实现批量处理器
  ```go
  // service/batch_processor.go
  type BatchProcessor struct {
      processor  *AmazonProcessor
      maxWorkers int
  }
  
  func (b *BatchProcessor) ProcessBatch(
      ctx context.Context,
      tasks []types.Task,
  ) (*BatchResult, error)
  ```

- [ ] 添加进度跟踪
  ```go
  // service/progress_tracker.go
  type ProgressTracker struct {
      total     int
      completed int
      failed    int
      mutex     sync.RWMutex
  }
  ```

- [ ] 实现批量 API
  ```go
  // api/batch.go
  func (c *Client) BatchCreateListings(
      ctx context.Context,
      requests []*ListingRequest,
  ) ([]*ListingResponse, error)
  ```

---

#### 2.3 产品监控 ⏳

**优先级**: 🟢 低  
**预计工时**: 7-10 天

**任务清单**:
- [ ] 实现价格监控
  ```go
  // service/price_monitor.go
  type PriceMonitor struct {
      apiClient *api.Client
      interval  time.Duration
  }
  
  func (m *PriceMonitor) MonitorPrices(
      ctx context.Context,
      skus []string,
  ) error
  ```

- [ ] 实现库存监控
  ```go
  // service/inventory_monitor.go
  type InventoryMonitor struct {
      apiClient *api.Client
  }
  ```

- [ ] 实现 Buy Box 监控
  ```go
  // service/buybox_monitor.go
  type BuyBoxMonitor struct {
      apiClient *api.Client
  }
  ```

---

## 📅 时间规划

### 第 1 周：产品属性映射
- Day 1-2: 设计属性映射配置
- Day 3-4: 实现映射器和验证器
- Day 5: 集成到处理流程

### 第 2 周：图片上传功能
- Day 1-2: 实现图片下载和处理
- Day 3-4: 实现 S3 上传
- Day 5: 集成测试

### 第 3 周：单元测试
- Day 1-2: API 层测试
- Day 3-4: 服务层测试
- Day 5: 集成测试

### 第 4 周：变体产品支持（可选）
- Day 1-2: 数据模型设计
- Day 3-4: 实现变体处理
- Day 5: 测试和文档

---

## 🔧 开发建议

### 1. 开发流程

```
1. 创建功能分支
   git checkout -b feature/attribute-mapping

2. 实现功能
   - 编写代码
   - 添加测试
   - 更新文档

3. 代码审查
   - 运行测试: go test ./...
   - 检查覆盖率: go test -cover ./...
   - 代码格式化: go fmt ./...

4. 合并到主分支
   git merge feature/attribute-mapping
```

### 2. 测试策略

```go
// 单元测试
func TestFunction(t *testing.T) {
    // Arrange
    input := "test"
    expected := "result"
    
    // Act
    result := Function(input)
    
    // Assert
    if result != expected {
        t.Errorf("expected %s, got %s", expected, result)
    }
}

// Mock 测试
func TestWithMock(t *testing.T) {
    mockClient := &MockAPIClient{}
    mockClient.On("CreateListing", mock.Anything).Return(nil)
    
    service := NewService(mockClient)
    err := service.DoSomething()
    
    assert.NoError(t, err)
    mockClient.AssertExpectations(t)
}
```

### 3. 性能优化

- 使用连接池
- 实现批量操作
- 添加缓存机制
- 并发控制

---

## 📊 进度跟踪

### 必须完成（阶段 1）

| 任务 | 状态 | 完成度 | 预计完成 |
|------|------|--------|----------|
| 产品属性映射 | ⏳ 待开始 | 0% | Week 1 |
| 图片上传功能 | ⏳ 待开始 | 0% | Week 2 |
| 单元测试 | ⏳ 待开始 | 0% | Week 3 |

### 可选增强（阶段 2）

| 任务 | 状态 | 完成度 | 预计完成 |
|------|------|--------|----------|
| 变体产品支持 | ⏳ 待开始 | 0% | Week 4 |
| 批量上架功能 | ⏳ 待开始 | 0% | Week 5 |
| 产品监控 | ⏳ 待开始 | 0% | Week 6-7 |

---

## 🎯 里程碑

### Milestone 1: 核心功能完善 (v1.1.0)
- ✅ 基础框架
- ⏳ 产品属性映射
- ⏳ 图片上传
- ⏳ 单元测试 80%+

**目标日期**: 3 周后

### Milestone 2: 功能增强 (v1.2.0)
- ⏳ 变体产品支持
- ⏳ 批量上架功能

**目标日期**: 5 周后

### Milestone 3: 生产优化 (v1.3.0)
- ⏳ 产品监控
- ⏳ 性能优化
- ⏳ 完整文档

**目标日期**: 7 周后

---

## 📚 参考资料

### Amazon SP-API 文档
- [Listings Items API](https://developer-docs.amazon.com/sp-api/docs/listings-items-api-v2021-08-01-reference)
- [Product Type Definitions](https://developer-docs.amazon.com/sp-api/docs/product-type-definitions-api-v2020-09-01-reference)
- [Uploads API](https://developer-docs.amazon.com/sp-api/docs/uploads-api-v2020-11-01-reference)

### 技术文档
- [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/docs/)
- [Go Testing](https://golang.org/pkg/testing/)
- [Testify Mock](https://github.com/stretchr/testify)

---

## 💡 最佳实践

### 1. 代码质量
- 保持文件 < 300 行
- 单一职责原则
- 完整的错误处理
- 添加日志记录

### 2. 测试覆盖
- 单元测试覆盖率 > 80%
- 集成测试关键流程
- Mock 外部依赖

### 3. 文档维护
- 更新 README
- 添加代码注释
- 记录 API 变更

---

## 🔄 持续改进

### 每周回顾
- 检查进度
- 更新路线图
- 调整优先级

### 每月总结
- 功能完成情况
- 性能指标
- 用户反馈

---

**最后更新**: 2025-12-05  
**维护者**: Task Processor Team
