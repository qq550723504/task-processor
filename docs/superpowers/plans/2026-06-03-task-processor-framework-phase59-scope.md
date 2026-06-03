## Task Processor Framework Phase 59 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit action target filters clone aggregate ownership`。

也就是继续处理：

- [generation_overview.go](/D:/code/task-processor/internal/listingkit/generation_overview.go:282)

里的：

- `cloneAssetGenerationFilters(...)`

### Why This Before Other Options

#### 1. Action target impact clone 这条线已经基本收干净了

现在：

- aggregate home 已只保留 top-level shallow copy
- shape home 已只保留 slice-clone home dispatch
- final slice home 已只保留 final local home dispatch
- `Platforms / QualityGrades / States` 已各自有清晰 final owner

继续在 impact clone 这条线上追求更细对称性，收益已经明显下降。

#### 2. `cloneAssetGenerationFilters(...)` 是下一个最自然的小切口

当前 filters clone 仍然同时知道：

- top-level shallow copy
- `Platforms` slice clone

这已经是很明确的 aggregate ownership hotspot，而且 direct consumers 很清楚。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- `cloneAssetGenerationFilters(...)`
- current direct consumers in action target and review navigation paths
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader action target clone redesign 或 generic clone framework。

### Explicitly Not Recommended Next

#### 1. 不建议继续深挖 action target impact final homes

`Platforms`、`QualityGrades`、`States` 现在都已经是单一、直接的 final owner 了。

#### 2. 不建议回头重开 shared retry request clone layering

那条线目前也已经没有同等级的 mixed final owner 还留着。

#### 3. 不建议直接重开 action execute orchestration

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 59: ListingKit action target filters clone aggregate ownership`
