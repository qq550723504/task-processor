## Task Processor Framework Phase 56 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit action target impact clone aggregate ownership`。

也就是继续处理：

- [task_generation_action_target_clone.go](/D:/code/task-processor/internal/listingkit/task_generation_action_target_clone.go:1)

里现在共同存在的：

- `Platforms` slice clone
- `QualityGrades` slice clone
- `States` slice clone

### Why This Before Other Options

#### 1. Shared retry request clone 这条线已经基本收干净了

现在：

- aggregate home 已只保留 top-level shallow copy
- shape home 已只保留 slice clone home dispatch
- slice home 已只保留 pairing home dispatch
- pairing home 已只保留 final clone home dispatch
- final homes 已各自只保留单一 slice clone

继续在 retry request clone 这条线上追求更细对称性，收益已经明显下降。

#### 2. `cloneAssetGenerationActionImpact(...)` 是下一个最自然的小切口

当前 impact clone 仍然同时知道：

- `Platforms`
- `QualityGrades`
- `States`

这已经是很明确的 aggregate ownership hotspot，而且 direct consumers 很清楚。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- `cloneAssetGenerationActionImpact(...)`
- `task_generation_action_target_clone_shape.go`
- current direct consumers
- current consumer-visible clone behavior stability

来做，不需要马上扩大到 broader action execution、navigation dispatch flow 或 generic clone framework。

### Explicitly Not Recommended Next

#### 1. 不建议继续深挖 shared retry request clone final homes

`TaskIDs` 与 `Slots` 现在都已经是单一、直接的 final owner 了。

#### 2. 不建议回头重开 queue query clone 或 navigation descriptor clone 邻域

这些 ownership 问题已经在前面阶段收得比较干净。

#### 3. 不建议直接重开 broader action execute orchestration

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 56: ListingKit action target impact clone aggregate ownership`
