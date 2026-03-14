# 工具函数提取重构文档

## 概述

将项目中重复实现的代码提取为通用工具函数，提高代码复用性和可维护性。

## 新增工具模块

### 1. 图片处理工具 (`internal/pkg/utils/image_utils.go`)

#### 核心功能

**Base64转换**
- `ImageToBase64(img, format, quality)` - 通用转换
- `ImageToBase64JPEG(img, quality)` - JPEG格式
- `ImageToBase64PNG(img)` - PNG格式
- `Base64ToImage(base64Str)` - base64转图片

**字节数组转换**
- `ImageToBytes(img, format, quality)` - 图片转字节
- `BytesToImage(data)` - 字节转图片

**其他工具**
- `LoadImageFromReader(r)` - 从Reader加载
- `GetImageFormat(data)` - 获取格式
- `GetImageSize(img)` - 获取尺寸

#### 替换位置

| 原位置 | 原函数 | 新函数 |
|--------|--------|--------|
| `watermark/detector_ai.go` | `imageToBase64` | `utils.ImageToBase64JPEG` |
| `watermark/remover_ai.go` | `imageToBase64`, `base64ToImage` | `utils.ImageToBase64PNG`, `utils.Base64ToImage` |
| `temu/handlers/image/vision_detector.go` | 内联实现 | `utils.ImageToBase64PNG` |

#### 优势

- 统一管理图片编码/解码逻辑
- 支持多种格式（JPEG、PNG）
- 自动格式检测
- 质量控制
- 错误处理完善

### 2. JSON处理工具 (`internal/pkg/utils/json_utils.go`)

#### 核心功能

**序列化（不转义HTML）**
- `MarshalWithoutHTMLEscape(v)` - 基础序列化
- `MarshalIndentWithoutHTMLEscape(v, prefix, indent)` - 带缩进
- `MarshalPretty(v)` - 美化输出

**反序列化**
- `UnmarshalStrict(data, v)` - 严格模式（禁止未知字段）

**字符串转换**
- `ToJSONString(v)` - 转JSON字符串
- `ToJSONStringPretty(v)` - 美化字符串

**工具函数**
- `IsValidJSON(str)` - 验证JSON
- `CompactJSON(data)` - 压缩JSON

#### 替换位置

项目中有多处重复实现 `marshalWithoutHTMLEscape`：

| 文件 | 状态 |
|------|------|
| `temu/handlers/sku/utils.go` | ✅ 已替换 |
| `temu/handlers/sku/ai_mapping_utils_helper.go` | 待替换 |
| `temu/handlers/product/submit_error_analyzer.go` | 待替换 |
| `temu/handlers/product/submit_utils.go` | 待替换 |
| `temu/handlers/product/save_publish_result_handler.go` | 待替换 |
| `temu/handlers/product/save_handler.go` | 待替换 |
| `shein/service/publish/handler_service.go` | 待替换 |
| `shein/service/product/attribute/template_service.go` | 待替换 |

#### 优势

- 避免 `&` 被转义为 `\u0026`
- 避免 `<` 被转义为 `\u003c`
- 统一JSON处理逻辑
- 减少重复代码

## 其他可提取的模式

### 1. 图片编码/解码

**当前状态**: 多处使用 `image.Decode(bytes.NewReader(data))`

**建议**: 已在 `image_utils.go` 中提供 `BytesToImage` 函数

**待替换位置**:
- `temu/handlers/image/padding_processor.go`
- `temu/handlers/image/dimension_annotator.go`
- `shein/service/product/image/processor_service.go`
- `shein/repo/image_repo.go`
- `amazon/core/service/image_processor.go`

### 2. HTTP请求构建

**当前状态**: 多处重复 `http.NewRequestWithContext` 模式

**建议**: 可以创建 `http_utils.go` 提供：
- `NewJSONRequest(ctx, method, url, body)` - JSON请求
- `NewFormRequest(ctx, method, url, data)` - 表单请求
- `DoRequest(req)` - 统一请求执行

### 3. 文件读写

**当前状态**: 多处使用 `os.ReadFile` 和 `os.WriteFile`

**建议**: 已有 `file_utils.go`，可以增强：
- `ReadJSONFile(path, v)` - 读取JSON文件
- `WriteJSONFile(path, v)` - 写入JSON文件
- `AppendToFile(path, data)` - 追加文件

## 迁移指南

### 图片Base64转换

**之前**:
```go
buf := new(bytes.Buffer)
if err := png.Encode(buf, img); err != nil {
    return "", err
}
base64Str := base64.StdEncoding.EncodeToString(buf.Bytes())
```

**之后**:
```go
base64Str, err := utils.ImageToBase64PNG(img)
if err != nil {
    return "", err
}
```

### JSON序列化（不转义HTML）

**之前**:
```go
var buf bytes.Buffer
encoder := json.NewEncoder(&buf)
encoder.SetEscapeHTML(false)
encoder.SetIndent("", "  ")
if err := encoder.Encode(v); err != nil {
    return nil, err
}
result := buf.Bytes()
if len(result) > 0 && result[len(result)-1] == '\n' {
    result = result[:len(result)-1]
}
return result, nil
```

**之后**:
```go
return utils.MarshalIndentWithoutHTMLEscape(v, "", "  ")
```

## 收益

1. **代码复用**: 减少重复代码约200+行
2. **易于维护**: 修改只需要改一处
3. **统一标准**: 所有地方使用相同的实现
4. **功能完善**: 工具函数提供更多功能和错误处理
5. **测试友好**: 集中测试工具函数即可

## 后续计划

1. 继续替换其他文件中的重复实现
2. 添加单元测试
3. 创建HTTP请求工具
4. 增强文件操作工具
5. 添加更多图片处理功能（压缩、裁剪、水印等）
