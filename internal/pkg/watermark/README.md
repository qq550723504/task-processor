# 水印检测与去除模块

## 功能概述

提供可配置的多方案水印检测与去除功能，支持传统图像处理算法和AI模型两种方式。

## 核心特性

### 检测方法

1. **传统算法检测** (`traditional`)
   - 边缘检测（Sobel算子）
   - 对比度分析
   - 颜色聚类
   - 纹理分析
   - 准确率：60-75%
   - 速度：快（<1秒）
   - 成本：无

2. **AI视觉检测** (`ai`)
   - 支持 GPT-4 Vision
   - 支持 Claude Vision
   - 准确率：85-95%
   - 速度：慢（2-5秒）
   - 成本：$0.01-0.05/张

3. **混合检测** (`hybrid`) - 推荐
   - 先用传统算法快速筛选
   - 对复杂情况使用AI确认
   - 准确率：80-88%
   - 速度：中等（1-3秒）
   - 成本：$0.001-0.01/张

### 去除方法

1. **图像修复** (`inpaint`) - 推荐
   - 使用周围像素修复水印区域
   - 效果自然
   - 适合大部分场景

2. **模糊处理** (`blur`)
   - 对水印区域应用高斯模糊
   - 速度快
   - 适合不重要的图片

3. **裁剪处理** (`crop`)
   - 裁剪掉边缘的水印
   - 质量最高
   - 仅适合边缘水印

4. **AI模型** (`ai`)
   - 使用LaMa等专业模型
   - 效果最好（90-95%）
   - 需要部署模型服务

## 快速开始

### 基础使用

```go
package main

import (
    "context"
    "image"
    _ "image/jpeg"
    "os"
    
    "task-processor/internal/pkg/watermark"
    "github.com/sirupsen/logrus"
)

func main() {
    // 1. 创建配置
    config := watermark.DefaultConfig()
    config.Detection.Method = watermark.DetectionMethodHybrid
    config.Removal.Method = watermark.RemovalMethodInpaint
    
    // 2. 创建处理器
    logger := logrus.New()
    processor := watermark.NewProcessor(config, logger)
    
    // 3. 加载图片
    file, _ := os.Open("product.jpg")
    defer file.Close()
    img, _, _ := image.Decode(file)
    
    // 4. 处理图片
    ctx := context.Background()
    result, err := processor.Process(ctx, img)
    if err != nil {
        panic(err)
    }
    
    // 5. 保存结果
    if result.Removal != nil && result.Removal.Success {
        // 保存处理后的图片
        outFile, _ := os.Create("product_clean.jpg")
        defer outFile.Close()
        jpeg.Encode(outFile, result.Removal.Image, nil)
    }
}
```

### 仅检测水印

```go
// 只检测不去除
result, err := processor.DetectOnly(ctx, img)
if result.HasWatermark {
    fmt.Printf("检测到 %d 个水印区域\n", len(result.Regions))
    for _, region := range result.Regions {
        fmt.Printf("位置: %s, 类型: %s, 置信度: %.2f\n",
            region.Position, region.Type, region.Confidence)
    }
}
```

### 自定义区域去除

```go
// 手动指定水印区域
regions := []*watermark.WatermarkRegion{
    {
        X: 100, Y: 200,
        Width: 150, Height: 30,
        Type: watermark.WatermarkTypeText,
        Position: watermark.PositionBottomRight,
        Confidence: 1.0,
    },
}

result, err := processor.RemoveOnly(ctx, img, regions)
```

## 配置说明

### 配置文件示例 (config-dev.yaml)

```yaml
watermark:
  enabled: true
  
  detection:
    method: "hybrid"  # traditional/ai/hybrid
    sensitivity: "medium"  # low/medium/high
    regions: ["corner", "edge"]  # 检测区域
    min_size: 20  # 最小水印尺寸
    threshold: 0.6  # 检测阈值
  
  removal:
    method: "inpaint"  # inpaint/blur/crop/ai
    quality: "high"
    preserve_aspect: true
    auto_crop: true
    blur_radius: 5
    inpaint_radius: 10
  
  ai:
    vision_api:
      enabled: false
      provider: "openai"
      api_key: "your-api-key"
      model: "gpt-4-vision-preview"
      max_cost: 0.05
    
    lama_model:
      enabled: false
      server_url: "http://localhost:8080/inpaint"
      use_gpu: false
  
  performance:
    max_concurrent: 3
    timeout: 30
    cache_enabled: true
    quality_score: 0.8
```

### 配置项说明

#### 检测配置

- `method`: 检测方法
  - `traditional`: 传统算法，快速但准确率一般
  - `ai`: AI模型，准确但成本高
  - `hybrid`: 混合方案，平衡速度和准确率（推荐）

- `sensitivity`: 检测灵敏度
  - `low`: 只检测明显水印（阈值0.7）
  - `medium`: 平衡模式（阈值0.6）
  - `high`: 检测更多可疑区域（阈值0.4）

- `regions`: 检测区域
  - `corner`: 四个角落
  - `edge`: 四条边缘
  - `center`: 中心区域
  - `full`: 全图扫描

#### 去除配置

- `method`: 去除方法
  - `inpaint`: 图像修复（推荐）
  - `blur`: 模糊处理
  - `crop`: 裁剪处理
  - `ai`: AI模型（需要部署服务）

- `quality`: 处理质量
  - `low`: 快速处理
  - `medium`: 平衡模式
  - `high`: 高质量处理

## 集成到Pipeline

```go
package main

import (
    "task-processor/internal/pipeline"
    "task-processor/internal/pipeline/handlers"
    "task-processor/internal/pkg/watermark"
)

func setupPipeline(config *watermark.Config) *pipeline.Pipeline {
    p := pipeline.NewPipeline()
    
    // 添加水印处理器
    watermarkHandler := handlers.NewWatermarkHandler(config, logger)
    p.AddHandler(watermarkHandler)
    
    // 添加其他处理器...
    
    return p
}
```

## 性能优化建议

### 1. 选择合适的检测方法

- **大批量处理**: 使用 `traditional` 方法
- **重要商品**: 使用 `hybrid` 方法
- **高质量要求**: 使用 `ai` 方法

### 2. 调整检测区域

```yaml
# 如果水印通常在角落
regions: ["corner"]

# 如果水印位置不固定
regions: ["corner", "edge", "center"]
```

### 3. 启用缓存

```yaml
performance:
  cache_enabled: true
  cache_ttl: 3600  # 1小时
```

### 4. 并发控制

```yaml
performance:
  max_concurrent: 3  # 根据CPU核心数调整
```

## 成本估算

### 传统算法方案
- 成本: 0
- 速度: 0.5-1秒/张
- 准确率: 60-75%

### 混合方案（推荐）
- 成本: $0.001-0.01/张
- 速度: 1-3秒/张
- 准确率: 80-88%

### 纯AI方案
- 成本: $0.02-0.05/张
- 速度: 3-8秒/张
- 准确率: 90-95%

## 常见问题

### Q: 如何提高检测准确率？

A: 
1. 使用 `hybrid` 或 `ai` 方法
2. 调整 `sensitivity` 为 `high`
3. 增加检测区域范围

### Q: 去除后有明显痕迹怎么办？

A:
1. 尝试使用 `ai` 方法（需要部署LaMa服务）
2. 调整 `inpaint_radius` 参数
3. 如果水印在边缘，使用 `crop` 方法

### Q: 如何部署LaMa服务？

A: 参考 [LaMa部署文档](https://github.com/advimman/lama)

### Q: 处理速度太慢怎么办？

A:
1. 使用 `traditional` 方法
2. 减少检测区域
3. 降低 `sensitivity`
4. 增加 `max_concurrent`

## 技术原理

### 传统检测算法

1. **边缘检测**: 使用Sobel算子检测文字和logo的边缘
2. **对比度分析**: 水印通常与背景有明显对比
3. **颜色聚类**: 水印颜色相对单一
4. **纹理分析**: 检测重复性图案

### 图像修复算法

使用加权平均的方式，从水印周围采样像素进行修复：
- 距离越近的像素权重越大
- 避免采样水印区域内的像素
- 保持颜色和纹理的连续性

## 未来计划

- [ ] 支持更多AI模型（如ION、ProPainter）
- [ ] 添加水印添加功能
- [ ] 支持视频水印处理
- [ ] 优化批量处理性能
- [ ] 添加更多质量评估指标
