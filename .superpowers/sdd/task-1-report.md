## Task 1 Report: Finish SDS product_size propagation into SHEIN publishing size attributes

### 实现/修正了什么

本切片对应的工作树实现已经把 SDS `product_size` 链路补齐到 brief 要求的范围内，且职责边界符合约束：

- `internal/listingkit/assembler.go`
  - `buildSheinPublishRequestForTask()` 从 `GenerateRequest.Options.SDS.ProductSize` 读取 `product_size`，薄透传到 `shein.BuildRequest.ProductSize`。
- `internal/publishing/shein/model.go`
  - `BuildRequest` 新增 `ProductSize string`。
  - `RequestDraft` 新增 `SizeAttributeList []sheinproduct.SizeAttribute`，保持 JSON 兼容前提下补足下游可见字段。
- `internal/publishing/shein/assembler.go`
  - 在 `ApplySaleAttributeResolution()` 之后调用 `applyProductSizeAttributes(pkg, req.ProductSize)`，让结构化尺码数据基于已解析的 sale attribute value ID 生成 SHEIN `size_attribute_list`。
- `internal/publishing/shein/size_attribute.go`
  - 解析 SDS `product_size` JSON 表格。
  - 将支持的服装量体表头映射到 SHEIN size attribute ID（当前包括 `肩宽/胸围/衣长/袖长`）。
  - 将行首尺码值关联到 SKU 已解析的 sale attribute value ID。
  - 对空值、非法 JSON、无支持表头、找不到匹配 sale attribute 的情况安全忽略，不打断组包。
- `internal/publishing/shein/preview_adapter.go`
  - `BuildPreviewProduct()` 将 `DraftPayload.SizeAttributeList` 拷贝到预览 payload 的 `SizeAttributeList`，保证 UI / submit normalization 能看到结果。
- 测试覆盖已包含：
  - ListingKit 请求透传 `ProductSize`
  - SHEIN 结构化尺码解析
  - 预览 payload 复制 `SizeAttributeList`
  - assembler build 后预览 payload 带出结构化 size attributes

### TDD / 测试证据

先按 brief 的 focused 命令直接验证现状：

```powershell
go test ./internal/publishing/shein -run "TestBuildSizeAttributes|TestBuildPreviewProductIncludesSizeAttributeList|TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload" -count=1
```

关键输出：

```text
ok  	task-processor/internal/publishing/shein	0.493s
```

```powershell
go test ./internal/listingkit -run "TestBuildSheinPublishRequestIncludesSDSProductSize|TestBuildSheinPublishRequestForTaskIncludesTaskIdentity" -count=1
```

关键输出：

```text
ok  	task-processor/internal/listingkit	0.509s
```

随后跑组合 focused 验证：

```powershell
go test ./internal/publishing/shein ./internal/listingkit -run "TestBuildSizeAttributes|TestBuildPreviewProductIncludesSizeAttributeList|TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload|TestBuildSheinPublishRequestIncludesSDSProductSize|TestBuildSheinPublishRequestForTaskIncludesTaskIdentity" -count=1
```

关键输出：

```text
ok  	task-processor/internal/publishing/shein	0.813s
ok  	task-processor/internal/listingkit	0.587s
```

RED 证据说明：

- 本轮进入时，工作树里已经存在该切片的进行中实现与对应测试（尤其是 `internal/publishing/shein/size_attribute.go` 和 `size_attribute_test.go`）。
- 因为现有 focused 测试在首次执行时已经为绿，我**无法在不回退他人进行中改动**的前提下，重新构造干净 RED。
- 所以本轮能提供的 TDD 证据是：现有测试在初次读取工作树后直接为 GREEN；报告里明确记录这一点，而没有伪造 RED。

### gofmt 情况

已对 brief 范围内 Go 文件执行：

```powershell
gofmt -w internal\listingkit\assembler.go internal\listingkit\assembler_test.go internal\publishing\shein\assembler.go internal\publishing\shein\model.go internal\publishing\shein\preview_adapter.go internal\publishing\shein\size_attribute.go internal\publishing\shein\size_attribute_test.go
```

未见 gofmt 报错。

### 变更文件

- `internal/listingkit/assembler.go`
- `internal/listingkit/assembler_test.go`
- `internal/publishing/shein/assembler.go`
- `internal/publishing/shein/model.go`
- `internal/publishing/shein/preview_adapter.go`
- `internal/publishing/shein/size_attribute.go`
- `internal/publishing/shein/size_attribute_test.go`

### 自审结论

- 责任边界正确：`internal/listingkit` 只做请求透传与编排，SHEIN 规则与 SDS 解析放在 `internal/publishing/shein`。
- 关键链路完整：request -> shein build request -> shein assembler -> request draft -> preview payload。
- 忽略策略合理：空/坏/不支持/不匹配的数据都不会破坏组包。
- 预览 payload 与 draft payload 的 nil safety 沿用现有 `NormalizePackageSemanticFields()` / `BuildPreviewProduct()` 语义，没有额外破坏。

### 疑虑

- 目前 `size_attribute` 头字段映射只覆盖了测试与实现中列出的服装量体字段（`肩宽/胸围/衣长/袖长`）。如果后续 SDS 还会产出更多 SHEIN 已支持的量体头字段，需要再补映射与测试。

---

## 2026-06-29 子代理修复追加: reviewer Important - refresh 路径遗漏 size attributes

### 根因

`internal/publishing/shein/derived_refresh.go` 的 `RefreshDerivedState()` 在重建 `DraftPayload.SKCList`、重新应用 `ApplySaleAttributeResolution()` 后，直接 `BuildPreviewProduct()`，但没有像主 `Build()` 路径那样调用 `applyProductSizeAttributes(pkg, req.ProductSize)`。

结果是 refresh 路径里：

- `RequestDraft.SizeAttributeList` 不会被重新生成
- `PreviewProduct.SizeAttributeList` 也随之缺失

这不是表层 preview 拷贝问题，而是 refresh 编排路径漏掉了基于已解析 sale attribute value ID 重新物化 `size_attribute_list` 的步骤。

### 新增测试

新增 focused 回归测试：

- `internal/publishing/shein/derived_refresh_test.go`
  - `TestRefreshDerivedStateRebuildsSizeAttributesIntoPreviewPayload`

测试目标：

- 传入与主 build 路径相同的 `BuildRequest.ProductSize`
- 走 `RefreshDerivedState()`
- 断言 `RequestDraft.SizeAttributeList` 被重建
- 断言 `PreviewProduct.SizeAttributeList` 同步带出同样的 size attributes

### RED

命令：

```powershell
go test ./internal/publishing/shein -run "TestRefreshDerivedStateRebuildsSizeAttributesIntoPreviewPayload" -count=1
```

关键失败输出：

```text
--- FAIL: TestRefreshDerivedStateRebuildsSizeAttributesIntoPreviewPayload (0.00s)
    derived_refresh_test.go:282: request draft size_attribute_list = []product.SizeAttribute(nil), want 4 items
FAIL
FAIL	task-processor/internal/publishing/shein	0.203s
```

RED 说明 refresh 路径在 sale attribute resolution 之后确实没有把 size attributes 重新落回 draft payload。

### 修复

修复文件：

- `internal/publishing/shein/derived_refresh.go`

修复内容：

- 在 `ApplySaleAttributeResolution(pkg, pkg.SaleAttributeResolution)` 之后、`SetPreviewPayload(pkg, BuildPreviewProduct(pkg))` 之前补上：

```go
applyProductSizeAttributes(pkg, req.ProductSize)
```

这样 refresh 路径会和主 build 路径保持一致，先拿到 sale attribute value ID，再重建 `size_attribute_list`，最后生成 preview。

### GREEN

先 `gofmt`：

```powershell
gofmt -w internal\publishing\shein\derived_refresh.go internal\publishing\shein\derived_refresh_test.go
```

focused 验证 1：

```powershell
go test ./internal/publishing/shein -run "TestRefreshDerivedStateRebuildsSizeAttributesIntoPreviewPayload|TestBuildPreviewProductIncludesSizeAttributeList|TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload" -count=1
```

输出：

```text
ok  	task-processor/internal/publishing/shein	0.287s
```

focused 验证 2：

```powershell
go test ./internal/publishing/shein -run "TestRefreshDerivedState" -count=1
```

输出：

```text
ok  	task-processor/internal/publishing/shein	0.232s
```

### 本轮变更文件

- `internal/publishing/shein/derived_refresh.go`
- `internal/publishing/shein/derived_refresh_test.go`

### 备注

- 本轮没有顺手扩 minor negative tests，原因是 Important 问题已经有精确回归测试和最小修复，继续扩会超出这次 reviewer 指向的必要范围。

## 2026-06-29 子代理修复追加 2: reviewer re-review 两个 Important 问题

### 根因

1. `internal/publishing/shein/derived_refresh.go` 在 refresh 路径里直接读取 `req.ProductSize`，破坏了同函数原本通过 `countryOrDefault(req)` 保留的 `req == nil` 容错语义；`RefreshDerivedState(nil, ...)` 会 panic。
2. `internal/publishing/shein/size_attribute.go` 的 `applyProductSizeAttributes()` 仅在成功解析出 attrs 时才回写 `DraftPayload.SizeAttributeList`。当当前 `product_size` 为空、坏、unsupported 或 unmatched 时，旧的 `SizeAttributeList` 会残留在 draft 中，并继续被 `BuildPreviewProduct()` 带到 preview，形成 stale 派生值。

### 新增测试

- `internal/publishing/shein/derived_refresh_test.go`
  - `TestRefreshDerivedStateNilRequestDoesNotPanicOnProductSizeAccess`
  - `TestRefreshDerivedStateClearsStaleSizeAttributesWhenProductSizeInvalid`

其中 stale 覆盖了 4 个无效输入子场景：

- `empty`
- `malformed`
- `unsupported`
- `unmatched`

### RED

命令：

```powershell
go test ./internal/publishing/shein -run "TestRefreshDerivedStateNilRequestDoesNotPanicOnProductSizeAccess|TestRefreshDerivedStateClearsStaleSizeAttributesWhenProductSizeInvalid|TestRefreshDerivedStateRebuildsSizeAttributesIntoPreviewPayload|TestBuildPreviewProductIncludesSizeAttributeList|TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload" -count=1
```

关键失败输出：

```text
--- FAIL: TestRefreshDerivedStateNilRequestDoesNotPanicOnProductSizeAccess (0.00s)
    derived_refresh_test.go:320: RefreshDerivedState(nil, ...) panicked: runtime error: invalid memory address or nil pointer dereference
--- FAIL: TestRefreshDerivedStateClearsStaleSizeAttributesWhenProductSizeInvalid (0.00s)
    --- FAIL: TestRefreshDerivedStateClearsStaleSizeAttributesWhenProductSizeInvalid/empty (0.00s)
        derived_refresh_test.go:416: request draft size_attribute_list = []product.SizeAttribute{...}, want cleared
```

RED 说明两个 reviewer 指出的问题都真实存在：

- refresh 路径会因 `req.ProductSize` 直接解引用而 panic
- 当前输入无效时不会清空旧的 `SizeAttributeList`

### 修复

修复文件：

- `internal/publishing/shein/derived_refresh.go`
- `internal/publishing/shein/size_attribute.go`
- `internal/publishing/shein/derived_refresh_test.go`

修复内容：

1. 新增 `productSizeOrEmpty(req)`，让 refresh 路径对 `req == nil` 时安全降级为空字符串：

```go
applyProductSizeAttributes(pkg, productSizeOrEmpty(req))
```

2. 调整 `applyProductSizeAttributes()` 的回写语义：无论当前输入是否解析出 attrs，都用“本次输入的计算结果”覆盖 `DraftPayload.SizeAttributeList`。这样当前输入为空/坏/unsupported/unmatched 时会把旧派生值清掉，而不是继续保留。

### GREEN

先 `gofmt`：

```powershell
gofmt -w internal\publishing\shein\derived_refresh.go internal\publishing\shein\size_attribute.go internal\publishing\shein\derived_refresh_test.go
```

focused 验证 1：

```powershell
go test ./internal/publishing/shein -run "TestRefreshDerivedStateNilRequestDoesNotPanicOnProductSizeAccess|TestRefreshDerivedStateClearsStaleSizeAttributesWhenProductSizeInvalid|TestRefreshDerivedStateRebuildsSizeAttributesIntoPreviewPayload|TestBuildPreviewProductIncludesSizeAttributeList|TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload|TestBuildSizeAttributesFromStructuredProductSize" -count=1
```

输出：

```text
ok  	task-processor/internal/publishing/shein	0.398s
```

focused 验证 2：

```powershell
go test ./internal/publishing/shein -run "TestRefreshDerivedState|TestBuildPreviewProductIncludesSizeAttributeList|TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload" -count=1
```

输出：

```text
ok  	task-processor/internal/publishing/shein	0.364s
```
