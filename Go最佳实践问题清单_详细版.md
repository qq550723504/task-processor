# Go 最佳实践问题清单 - 详细版

## 📋 问题清单索引

本文档包含所有发现的最佳实践问题的详细清单，按文件和行号组织。

---

## 🔴 Critical Issues - 严重问题

### 1. 文件长度超过300行

#### 1.1 超过800行的文件（最优先）

| 文件路径 | 行数 | 主要问题 | 建议拆分方案 |
|---------|------|--------|-----------|
| internal/platforms/shein/modules/sensitive_word_service.go | 889 | 包含配置、加载、处理、验证、缓存等多个职责 | 拆分为5个文件 |
| internal/platforms/shein/modules/sku_builder.go | 841 | 包含SKU构建、验证、转换等多个职责 | 拆分为4个文件 |
| internal/platforms/shein/modules/attribute_selector_handler.go | 817 | 包含属性选择、验证、映射等多个职责 | 拆分为4个文件 |

#### 1.2 超过700行的文件

| 文件路径 | 行数 | 主要问题 |
|---------|------|--------|
| internal/platforms/shein/modules/publish_product_handler.go | 713 | 包含发布、验证、更新等多个职责 |
| internal/platforms/shein/modules/skc_builder.go | 704 | 包含SKC构建、验证、转换等多个职责 |

#### 1.3 超过500行的文件

| 文件路径 | 行数 | 主要问题 |
|---------|------|--------|
| internal/platforms/temu/handlers/image_upload_processor.go | 561 | 包含上传、验证、处理等多个职责 |
| internal/common/management/impl/image_downloader.go | 565 | 包含下载、缓存、验证等多个职责 |
| internal/platforms/temu/client/api_client.go | 553 | 包含API调用、重试、错误处理等多个职责 |
| internal/common/temu/api_client.go | 553 | 包含API调用、重试、错误处理等多个职责 |
| internal/common/amazon/browser/zipcode.go | 543 | 包含邮编设置、验证、重试等多个职责 |
| internal/platforms/shein/modules/sale_attribute_preparation.go | 518 | 包含属性准备、验证、转换等多个职责 |
| internal/updater/updater.go | 507 | 包含更新检查、下载、安装等多个职责 |

#### 1.4 超过400行的文件

| 文件路径 | 行数 |
|---------|------|
| internal/common/amazon/processor.go | 457 |
| internal/platforms/shein/modules/variant_matcher.go | 457 |
| internal/platforms/temu/handlers/image_validator.go | 464 |
| internal/platforms/temu/handlers/image_dimension_annotator.go | 436 |
| internal/platforms/temu/handlers/variant_json_data_handler.go | 432 |
| internal/platforms/temu/handlers/filter_rule_handler.go | 414 |
| internal/common/shein/impl/product_api.go | 410 |
| internal/platforms/shein/client/impl/product_api.go | 410 |
| internal/common/product/fetcher.go | 418 |
| internal/platforms/amazon/api/listings.go | 371 |
| internal/platforms/temu/handlers/ai_property_mapper.go | 377 |
| internal/platforms/temu/types/product.go | 378 |
| internal/platforms/temu/handlers/sku_builder.go | 372 |
| internal/platforms/temu/handlers/commit_detail_handler.go | 350 |
| internal/platforms/shein/modules/image_processor.go | 344 |
| internal/common/temu/pricing_decision_service.go | 334 |
| internal/platforms/temu/handlers/bullet_points_validator.go | 318 |
| internal/platforms/temu/handlers/sku_item_builder.go | 318 |
| internal/platforms/shein/modules/string_sanitizer.go | 321 |
| internal/platforms/shein/modules/skc_attribute_strategy_handler.go | 356 |
| internal/platforms/shein/modules/sale_attribute_gpt.go | 337 |
| internal/platforms/shein/modules/auto_pricing_handler.go | 326 |
| internal/platforms/shein/modules/sale_attribute_validation.go | 313 |
| internal/platforms/shein/modules/translate_handler.go | 393 |
| internal/platforms/shein/strategy_executor.go | 382 |
| internal/platforms/shein/sync_service.go | 474 |
| internal/platforms/shein/product_monitor_service.go | 491 |
| internal/platforms/amazon/internal/service/attribute_builder.go | 443 |
| internal/common/amazon/extractor/price_extractor.go | 497 |

---

### 2. Goroutine缺少Panic Recovery

#### 2.1 缺少panic recovery的goroutine列表

| 文件路径 | 行号 | 问题描述 | 修复状态 |
|---------|------|--------|--------|
| internal/platforms/shein/modules/mark_variant_publish_success_handler.go | 274 | 异步更新状态，无panic recovery | ❌ 未修复 |
| internal/platforms/shein/modules/save_publish_result_handler.go | 382 | 异步更新状态，无panic recovery | ❌ 未修复 |
| internal/platforms/shein/modules/image_processor.go | 116 | 等待结果，无panic recovery | ❌ 未修复 |
| internal/common/worker/pool.go | 89 | 等待完成，无panic recovery | ❌ 未修复 |
| internal/goroutine/manager.go | 165 | 等待完成，无panic recovery | ❌ 未修复 |
| internal/platforms/temu/handlers/build_spu_handler.go | 63 | 并行执行，无panic recovery | ❌ 未修复 |
| internal/utils/shutdown.go | 91 | 等待钩子完成，无panic recovery | ❌ 未修复 |
| internal/updater/updater.go | 398 | 强制终止进程，有panic recovery | ✅ 已修复 |
| internal/platforms/shein/modules/auto_pricing_handler.go | 33 | 启动定时任务，有panic recovery | ✅ 已修复 |
| internal/platforms/shein/task_handler.go | 243 | 异步执行，有panic recovery | ✅ 已修复 |
| internal/platforms/shein/modules/sensitive_word_service.go | 882 | 异步保存配置，有panic recovery | ✅ 已修复 |
| internal/memory/shop_pause_manager.go | 245 | 启动清理任务，有panic recovery | ✅ 已修复 |
| internal/common/task/fetcher.go | 321 | 异步更新状态，有panic recovery | ✅ 已修复 |
| internal/common/amazon/browser/browser_pool.go | 286 | 异步重新创建，有panic recovery | ✅ 已修复 |
| internal/common/amazon/browser/browser_pool.go | 411 | 启动健康检查，有panic recovery | ✅ 已修复 |

---

### 3. 错误处理：使用%v而不是%w

#### 3.1 错误包装问题列表

| 文件路径 | 行号 | 错误代码 | 修复方案 |
|---------|------|--------|--------|
| internal/platforms/shein/modules/json_map.go | 68 | `fmt.Errorf("无法解析JSON数据: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/sensitive_words_processor.go | 113 | `fmt.Errorf("敏感词清理失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/supplier_info_handler.go | 40 | `fmt.Errorf("删除过期Cookie失败: %v, ...")` | 改为 `%w` |
| internal/platforms/shein/modules/mark_variant_publish_success_handler.go | 106 | `fmt.Errorf("转换任务ID失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/mark_variant_publish_success_handler.go | 178 | `fmt.Errorf("创建产品导入映射关系失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/mark_variant_publish_success_handler.go | 200 | `fmt.Errorf("转换任务ID失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/mark_variant_publish_success_handler.go | 245 | `fmt.Errorf("创建产品导入映射关系失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/mark_variant_publish_success_handler.go | 264 | `fmt.Errorf("解析任务ID失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/image_processor.go | 147 | `fmt.Errorf("主图上传失败: %v", uploadResults[0].err)` | 改为 `%w` |
| internal/platforms/shein/modules/image_processor.go | 253 | `fmt.Errorf("图片排序验证失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/image_processor.go | 317 | `fmt.Errorf("下载图片失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/image_processor.go | 323 | `fmt.Errorf("解码图片失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/modules/image_processor.go | 343 | `fmt.Errorf("编码图片失败: %v", err)` | 改为 `%w` |
| internal/platforms/shein/client/impl/image_api.go | 91 | `fmt.Errorf("重新编码图片失败: %v", err)` | 改为 `%w` |
| internal/platforms/temu/handlers/cost_template_handler.go | 140 | `fmt.Errorf("发送请求失败: %v", err)` | 改为 `%w` |
| internal/platforms/temu/handlers/out_goods_sn_check_handler.go | 191 | `fmt.Errorf("发现SKU编码重复，请检查并修改重复的编码: %v", duplicateErrors)` | 改为 `%w` |
| internal/platforms/temu/handlers/template_query_handler.go | 266 | `fmt.Errorf("发送请求失败: %v", err)` | 改为 `%w` |
| internal/platforms/temu/handlers/text_check_handler.go | 91 | `fmt.Errorf("发送请求失败: %v", err)` | 改为 `%w` |
| internal/platforms/amazon/internal/service/schema_manager.go | 131 | `fmt.Errorf("数据验证失败: %v", errors)` | 改为 `%w` |
| internal/common/shein/impl/image_api.go | 91 | `fmt.Errorf("重新编码图片失败: %v", err)` | 改为 `%w` |

---

### 4. Context使用不当

#### 4.1 不当使用context.Background()的位置

| 文件路径 | 行号 | 问题代码 | 修复方案 |
|---------|------|--------|--------|
| internal/platforms/temu/handlers/image_dimension_annotator.go | 451 | `ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)` | 应接收context参数 |
| internal/platforms/temu/handlers/sku_ai_mapping_single.go | 177 | `aiCtx := context.Background()` | 应接收context参数 |
| internal/platforms/temu/handlers/ai_property_mapper.go | 224 | `ctx := context.Background()` | 应接收context参数 |
| internal/platforms/shein/modules/category_manager.go | 96 | `ctx := context.Background()` | 应接收context参数 |
| internal/platforms/shein/modules/category_manager.go | 180 | `ctx := context.Background()` | 应接收context参数 |
| internal/platforms/shein/modules/translate_handler.go | 263 | `context.Background()` | 应接收context参数 |
| internal/platforms/temu/handlers/ai_content_rewriter.go | 260 | `ctx := context.Background()` | 应接收context参数 |
| internal/platforms/amazon/internal/service/dynamic_template.go | 72 | `context.Background()` | 应接收context参数 |
| internal/common/processor/base_processor.go | 43 | `ctx := context.Background()` | 应接收context参数 |
| internal/common/processor/base_processor.go | 122 | `ctx := context.Background()` | 应接收context参数 |
| internal/common/amazon/browser/browser_pool.go | 110 | `ctx := context.Background()` | 应接收context参数 |
| internal/common/management/impl/image_downloader.go | 289 | `ctx, cancel := context.WithTimeout(context.Background(), d.timeout)` | 应接收context参数 |
| internal/common/management/impl/image_downloader.go | 337 | `ctx, cancel := context.WithTimeout(context.Background(), d.timeout)` | 应接收context参数 |

---

## 🟠 High Priority Issues - 高优先级问题

### 5. 日志安全：输出敏感信息

#### 5.1 日志输出敏感信息的位置

| 文件路径 | 行号 | 敏感信息类型 | 修复方案 |
|---------|------|-----------|--------|
| internal/common/auth/client_credentials.go | 74 | ClientID, TenantID | 使用脱敏函数 |
| internal/common/auth/token_store.go | 59 | Token | 使用脱敏函数 |
| internal/auth/client_credentials.go | 74 | ClientID, TenantID | 使用脱敏函数 |
| internal/auth/token_store.go | 59 | Token | 使用脱敏函数 |

---

### 6. 缺少导出函数的Godoc注释

#### 6.1 缺少注释的导出函数示例（部分）

| 文件路径 | 函数名 | 行号 | 修复状态 |
|---------|--------|------|--------|
| internal/memory/cookie_manager.go | SetCookie | - | ❌ 未修复 |
| internal/memory/cookie_manager.go | GetCookie | - | ❌ 未修复 |
| internal/memory/shop_pause_manager.go | PauseShop | - | ❌ 未修复 |
| internal/memory/shop_pause_manager.go | IsShopPaused | - | ❌ 未修复 |
| internal/memory/daily_count_manager.go | IncrementCount | - | ❌ 未修复 |
| internal/memory/daily_count_manager.go | GetCount | - | ❌ 未修复 |
| internal/memory/relisting_queue_manager.go | Push | - | ❌ 未修复 |
| internal/memory/relisting_queue_manager.go | Pop | - | ❌ 未修复 |

**总计**: 150+个导出函数缺少注释

---

### 7. 缺少包注释的文件

#### 7.1 缺少包注释的文件列表（部分）

| 文件路径 | 包名 | 修复状态 |
|---------|------|--------|
| cmd/task/main.go | main | ❌ 未修复 |
| internal/memory/manager.go | memory | ❌ 未修复 |
| internal/updater/updater.go | updater | ❌ 未修复 |
| internal/platforms/temu/task_handler.go | temu | ❌ 未修复 |
| internal/platforms/temu/types/product.go | types | ❌ 未修复 |
| internal/platforms/temu/types/errors.go | types | ❌ 未修复 |
| internal/platforms/temu/task_submitter.go | temu | ❌ 未修复 |
| internal/platforms/temu/product_monitor_service.go | temu | ❌ 未修复 |
| internal/platforms/temu/sync_service.go | temu | ❌ 未修复 |

**总计**: 80+个文件缺少包注释

---

### 8. Goroutine退出条件不完善

#### 8.1 缺少context.Done()处理的goroutine

| 文件路径 | 行号 | 问题描述 | 修复方案 |
|---------|------|--------|--------|
| internal/memory/shop_pause_manager.go | 245 | 定时清理任务无context.Done()处理 | 添加case <-ctx.Done() |
| internal/common/amazon/browser/browser_pool.go | 411 | 健康检查任务无context.Done()处理 | 添加case <-ctx.Done() |

---

## 🟡 Medium Priority Issues - 中优先级问题

### 9. 切片容量预分配不足

#### 9.1 切片操作没有预分配容量的位置

| 文件路径 | 行号 | 问题代码 | 修复方案 |
|---------|------|--------|--------|
| internal/platforms/shein/modules/sensitive_word_service.go | - | `var items []Item; for ... append(...)` | 使用make预分配 |
| internal/platforms/amazon/internal/service/attribute_builder.go | - | `var attributes []Attribute; for ... append(...)` | 使用make预分配 |

**总计**: 20+处切片操作需要优化

---

### 10. Context作为结构体字段

#### 10.1 将context存储为字段的结构体

| 文件路径 | 结构体名 | 行号 | 修复方案 |
|---------|---------|------|--------|
| internal/platforms/shein/task_handler.go | TaskContext | 56 | 移除Context字段，通过参数传递 |
| internal/platforms/temu/handlers/pipeline.go | TaskContext | - | 移除Context字段，通过参数传递 |

---

## 🟢 Low Priority Issues - 低优先级问题

### 11. 变量命名规范

#### 11.1 可改进的变量命名

| 文件路径 | 行号 | 当前命名 | 建议命名 |
|---------|------|--------|--------|
| - | - | `k, v` (map遍历) | `key, value` 或 `key, cookieInfo` |
| - | - | `i, item` (循环) | `index, item` 或 `idx, item` |

---

## ✅ Passed Checks - 通过检查

### 12. 包名规范

**检查结果**: ✅ **完全符合规范**
- 所有包名都是小写
- 包名简洁明确
- 无下划线或大写字母

### 13. HTTP客户端配置

**检查结果**: ✅ **基本符合规范**
- `internal/common/auth/client_credentials.go:40` - Timeout: 30秒 ✅
- `internal/auth/client_credentials.go:30` - Timeout: 30秒 ✅

---

## 📊 修复优先级矩阵

```
优先级高 ↑
         │
    🔴   │  🔴  🔴  🔴
    影响 │  文件 Goroutine Context
    大   │  长度 panic    使用
         │
    🟠   │  🟠  🟠  🟠
         │  日志 注释  包注释
         │  安全 缺失  缺失
         │
    🟡   │  🟡  🟡
         │  切片 Context
         │  预分 字段
         │
    🟢   │  🟢
         │  变量
         │  命名
         │
         └─────────────────→ 修复难度
           低              高
```

---

## 🎯 修复建议

### 快速修复（1-2小时）
1. 全局替换 `%v` 为 `%w` 在错误处理中
2. 为所有goroutine添加panic recovery
3. 移除日志中的敏感信息

### 中期修复（1-2周）
1. 添加缺少的包注释
2. 添加缺少的导出函数注释
3. 完善Goroutine退出条件

### 长期改进（2-4周）
1. 拆分超过300行的文件
2. 修复Context使用问题
3. 优化切片初始化

