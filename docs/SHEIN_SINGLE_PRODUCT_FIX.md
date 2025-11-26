# SHEIN 单品处理修复

## 问题描述

SHEIN 平台在处理单品（无变体）产品时出现错误：

```
INFO[2025-11-22 23:05:54] 🔍 [SHEIN变体] 从AsinSkuMap获取完成: 有效变体=0, 排除主产品=1, 总数=1
ERRO[2025-11-22 23:05:54] 步骤执行失败 [13/34] [获取所有变体的Json数据]: 没有找到变体ASIN列表
```

## 根本原因

在 `platforms/shein/modules/variant_json_data_handler.go` 中，`Handle` 方法会从 `AsinSkuMap` 中提取变体 ASIN 列表，并排除主产品 ASIN。

对于单品（无变体）的情况：
- `AsinSkuMap` 只包含主产品 ASIN
- 排除主产品后，变体列表为空
- 代码将空列表视为错误，返回 "没有找到变体ASIN列表"

## 解决方案

修改 `Handle` 方法，将空变体列表视为正常的单品情况：

```go
// 如果没有变体（单品情况），初始化空列表并继续
if len(variantAsins) == 0 {
    logrus.Infof("✅ 产品 %s 没有变体（单品），跳过变体数据获取", mainProductAsin)
    emptyVariants := make([]amazon.Product, 0)
    ctx.Variants = &emptyVariants
    return nil
}
```

## 对比 TEMU 平台

TEMU 平台已经正确处理了这种情况：

```go
if len(variantAsins) == 0 {
    h.logger.Info("未发现变体ASIN列表，使用单一产品模式")
    return h.processSingleProduct(ctx)
}
```

## 影响

修复后：
- ✅ 单品产品可以正常处理
- ✅ 不会因为没有变体而报错
- ✅ 后续步骤可以正常执行（使用空的变体列表）
- ✅ 与 TEMU 平台行为保持一致

## 测试建议

测试以下场景：
1. 单品产品（AsinSkuMap 只包含主产品）
2. 有变体的产品（AsinSkuMap 包含主产品 + 多个变体）
3. 大量变体的产品（>100 个变体，应该触发限制）

## 文件修改

- `platforms/shein/modules/variant_json_data_handler.go`
  - 修改 `Handle` 方法，处理空变体列表情况
