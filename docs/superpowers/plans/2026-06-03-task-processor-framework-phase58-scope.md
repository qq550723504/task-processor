## Task Processor Framework Phase 58 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit action target impact final slice ownership`。

也就是继续处理：

- [task_generation_action_impact_slice_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_impact_slice_clone.go:1)

里现在共同存在的：

- `Platforms` slice clone
- `QualityGrades` slice clone
- `States` slice clone

### Why This Before Other Options

#### 1. Action target impact aggregate home 和 shape home 都已经收干净了

现在：

- aggregate home 已只保留 top-level shallow copy
- shape home 已只保留 slice-clone home dispatch

继续在这两层追求更细对称性，收益已经明显下降。

#### 2. `Platforms / QualityGrades / States` 是下一个最自然的小切口

当前 slice-clone home 仍然同时知道：

- `Platforms`
- `QualityGrades`
- `States`

这已经是很明确的 ownership hotspot，而且 direct consumers 很清楚。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- impact final slice clone shaping
- current `cloneAssetGenerationActionImpact(...)` consumers
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader action target clone redesign 或 generic clone framework。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 shared retry request clone layering

那条线目前已经没有同等级的 mixed final owner 还留着。

#### 2. 不建议直接重开 action target aggregate routing

当前 aggregate 和 shape 两层都已经是清晰 router。

#### 3. 不建议直接重开 action execute orchestration

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 58: ListingKit action target impact final slice ownership`
