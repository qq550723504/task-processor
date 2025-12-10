# Amazon 产品属性映射功能

## 概述

Amazon 平台模块实现了从 1688 产品数据到 Amazon 产品属性的自动映射功能。

## 数据流程

```
1688产品数据 (管理系统)
    ↓
ProductFetcherHandler (获取原始JSON)
    ↓
DataParserHandler (解析JSON为结构化数据)
    ↓
AttributeMapperHandler (映射为Amazon属性)
    ↓
Amazon SP-API (上架到Amazon)
```

## 核心组件

### 1. ProductFetcherHandler
**文件**: `platforms/amazon/handlers/product_fetcher_handler.go`

**功能**: 从管理系统获取1688产品的原始JSON数据

**输入**:
- `Task.ProductID`: 1688产品ID
- `Task.TenantID`: 租户ID
- `Task.StoreID`: 店铺ID

**输出**:
- `raw_json_data`: 1688产品的原始JSON字符串
- `product_id_1688`: 1688产品ID

### 2. DataParserHandler
**文件**: `platforms/amazon/handlers/data_parser_handler.go`

**功能**: 将1688的JSON数据解析为结构化的map数据

**输入**:
- `raw_json_data`: 原始JSON字符串

**输出**:
- `raw_product_data`: 解析后的产品数据 (map[string]interface{})

### 3. AttributeMapperHandler
**文件**: `platforms/amazon/handlers/attribute_mapper_handler.go`

**功能**: 将1688产品数据映射为Amazon所需的属性格式

**输入**:
- `raw_product_data`: 1688产品数据

**输出**:
- `mapped_attributes`: 映射后的Amazon属性
- `product_type`: 产品类型 (PRODUCT/CLOTHING/ELECTRONICS)

## 配置文件

### attribute_mapping.yaml
**路径**: `platforms/amazon/config/attribute_mapping.yaml`

**配置内容**:

#### 1. 产品类型定义
```yaml
product_types:
  PRODUCT:          # 标准产品
  CLOTHING:         # 服装类
  ELECTRONICS:      # 电子产品类
```

#### 2. 属性映射规则
```yaml
attribute_mappings:
  item_name:
    source_fields:
      - subject      # 1688主标题
      - title
    max_length: 200
    required: true
```

#### 3. 值转换规则
```yaml
value_transformations:
  color:
    "红色": "Red"
    "蓝色": "Blue"
```

#### 4. 验证规则
```yaml
validation_rules:
  item_name:
    min_length: 1
    max_length: 200
    pattern: "^[^<>]*$"
```

## 1688 字段映射

### 常见1688字段

| 1688字段 | Amazon字段 | 说明 |
|---------|-----------|------|
| subject | item_name | 产品标题 |
| description | product_description | 产品描述 |
| detailDesc | product_description | 详细描述 |
| price | - | 价格（需单独处理） |
| imageUrl | - | 主图URL |
| images | - | 图片列表 |
| color | color | 颜色 |
| size | size | 尺寸 |
| material | material_type | 材质 |
| weight | item_weight | 重量 |
| supplierName | manufacturer | 供应商名称 |

## 使用示例

### 手动使用Handler

```go
package main

import (
    "task-processor/platforms/amazon"
    "task-processor/platforms/amazon/handlers"
    "task-processor/common/types"
)

func main() {
    // 1. 创建任务上下文
    task := types.Task{
        ProductID: "1688_product_123",
        TenantID:  1,
        StoreID:   100,
    }
    ctx := amazon.NewTaskContext()
    ctx.Task = &task
    
    // 2. 获取1688产品数据
    fetcherHandler := handlers.NewProductFetcherHandler(rawJsonDataClient)
    if err := fetcherHandler.Handle(ctx); err != nil {
        log.Fatal(err)
    }
    
    // 3. 解析JSON数据
    parserHandler := handlers.NewDataParserHandler()
    if err := parserHandler.Handle(ctx); err != nil {
        log.Fatal(err)
    }
    
    // 4. 映射属性
    mapperHandler, _ := handlers.NewAttributeMapperHandler(
        "platforms/amazon/config/attribute_mapping.yaml",
    )
    if err := mapperHandler.Handle(ctx); err != nil {
        log.Fatal(err)
    }
    
    // 5. 获取映射结果
    mappedAttrs, _ := ctx.GetData("mapped_attributes")
    productType, _ := ctx.GetData("product_type")
    
    log.Printf("产品类型: %v", productType)
    log.Printf("映射属性: %+v", mappedAttrs)
}
```

### 集成到Pipeline

```go
// 在 processor.go 的 buildPipeline 方法中
func (p *AmazonProcessor) buildPipeline() *Pipeline {
    pipeline := NewPipeline()
    
    // 添加Handler
    pipeline.AddHandler(handlers.NewProductFetcherHandler(
        p.managementClient.GetRawJsonDataClient(),
    ))
    pipeline.AddHandler(handlers.NewDataParserHandler())
    
    mapperHandler, _ := handlers.NewAttributeMapperHandler(
        "platforms/amazon/config/attribute_mapping.yaml",
    )
    pipeline.AddHandler(mapperHandler)
    
    return pipeline
}
```

## 扩展配置

### 添加新的产品类型

在 `attribute_mapping.yaml` 中添加：

```yaml
product_types:
  BOOKS:
    display_name: "图书"
    required_attributes:
      - item_name
      - brand
      - author
      - isbn
    optional_attributes:
      - publisher
      - publication_date
```

### 添加新的属性映射

```yaml
attribute_mappings:
  author:
    source_fields:
      - author
      - writer
    max_length: 100
    required: true
```

### 添加新的值转换

```yaml
value_transformations:
  size:
    "特小": "XS"
    "小": "S"
    "中": "M"
    "大": "L"
    "特大": "XL"
```

## 注意事项

1. **循环导入问题**: 由于Go的包导入限制，Handler不能直接在processor包中初始化，需要通过接口或运行时动态创建

2. **配置文件路径**: 确保配置文件路径正确，建议使用相对于项目根目录的路径

3. **字段缺失处理**: 如果1688数据中缺少必填字段，会使用配置中的默认值或返回错误

4. **数据验证**: 映射后的数据会经过验证器验证，确保符合Amazon的要求

## 下一步工作

- [ ] 实现图片上传功能
- [ ] 实现价格转换逻辑
- [ ] 实现库存同步
- [ ] 添加单元测试
- [ ] 完善错误处理

## 相关文档

- [Amazon SP-API 文档](https://developer-docs.amazon.com/sp-api/)
- [产品属性定义](https://developer-docs.amazon.com/sp-api/docs/product-type-definitions-api-v2020-09-01-reference)
- [ROADMAP.md](../ROADMAP.md)
