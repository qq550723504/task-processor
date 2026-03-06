# TEMU Handlers 目录结构

本目录包含 TEMU 平台的所有处理器（handlers），已按功能模块重新组织。

## 目录结构

```
handlers/
├── ai/              # AI 相关功能 (13个文件)
│   ├── content_rewriter.go
│   ├── prompt_builder.go
│   ├── property_mapper_*.go
│   ├── service.go
│   └── sku_mapping_*.go
│
├── category/        # 类目处理 (3个文件)
│   ├── handler.go
│   ├── recommend_handler.go
│   └── disclaim_handler.go
│
├── common/          # 通用基础 (5个文件)
│   ├── base_handler.go
│   ├── init_handler.go
│   ├── models.go
│   ├── temu_handler.go
│   └── types.go
│
├── filter/          # 过滤规则 (10个文件)
│   ├── prohibited_items_*.go
│   ├── rule_*.go
│   └── sensitive_words_filter.go
│
├── image/           # 图片处理 (20个文件)
│   ├── processor.go
│   ├── validator.go
│   ├── upload_*.go
│   ├── dimension_*.go
│   └── vision_detector.go
│
├── product/         # 产品相关 (27个文件)
│   ├── submit_*.go
│   ├── description_validator*.go
│   ├── name_*.go
│   ├── spu_*.go
│   └── price_*.go
│
├── property/        # 属性处理 (24个文件)
│   ├── validator*.go
│   ├── mapper_*.go
│   ├── pipeline.go
│   ├── orchestrator.go
│   └── *_stage.go
│
├── sku/             # SKU 相关 (14个文件)
│   ├── builder.go
│   ├── mapping_*.go
│   ├── variant_*.go
│   └── price_calculator.go
│
├── spec/            # 规格处理 (4个文件)
│   ├── resolver_service.go
│   ├── dimension_*.go
│   └── query_adapter.go
│
├── store/           # 店铺信息 (2个文件)
│   ├── id_handler.go
│   └── info_handler.go
│
├── template/        # 模板查询 (2个文件)
│   ├── cost_handler.go
│   └── query_handler.go
│
└── validation/      # 验证规则 (10个文件)
    ├── rule_*.go
    ├── bullet_points_*.go
    └── text_*.go
```

## 模块说明

### AI (13个文件)
- AI 内容重写
- AI 属性映射
- AI SKU 映射
- OpenAI 服务集成

### Category (3个文件)
- 类目推荐
- 类目处理
- 类目免责声明

### Common (5个文件)
- 基础处理器
- 通用类型定义
- 初始化处理

### Filter (10个文件)
- 违禁品检测
- 敏感词过滤
- 筛选规则管理

### Image (20个文件)
- 图片验证
- 图片上传
- 尺寸标注
- 白边填充
- Vision API 检测

### Product (27个文件)
- 产品提交
- 产品验证
- SPU 构建
- 价格处理
- 产品描述优化

### Property (24个文件)
- 属性验证
- 属性映射
- 属性管道
- 多阶段处理

### SKU (14个文件)
- SKU 构建
- 变体处理
- 规格映射
- 价格计算

### Spec (4个文件)
- 规格解析
- 维度选择
- 规格查询

### Store (2个文件)
- 店铺信息查询
- 店铺 ID 处理

### Template (2个文件)
- 成本模板
- 模板查询

### Validation (10个文件)
- 验证规则引擎
- 文本检查
- 要点验证

## 注意事项

所有文件仍然属于同一个 `package handlers`，这意味着：
- 包内的类型和函数可以直接相互引用
- 不需要修改 import 语句
- 保持向后兼容性

## 重构历史

- 2024-XX-XX: 将 140+ 个文件按功能模块重新组织到 12 个子目录
- 目的：提高代码可维护性和可读性
- 方法：使用 smartRelocate 工具批量移动文件，保持包结构不变
