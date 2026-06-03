## Task Processor Framework Phase 60 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit action target filters platform slice ownership`。

也就是继续处理：

- [generation_filters_clone_shape.go](/D:/code/task-processor/internal/listingkit/generation_filters_clone_shape.go:1)

里的：

- `Platforms` slice clone

### Why This Before Other Options

#### 1. Action target filters aggregate home 已经收干净了

现在：

- aggregate home 已只保留 top-level shallow copy
- shape home 已接住 `Platforms` slice clone

继续在 aggregate home 这层追求更细对称性，收益已经明显下降。

#### 2. `Platforms` final owner 是下一个最自然的小切口

当前 shape home 仍然直接知道：

- `Platforms`

这已经是很明确的 ownership hotspot，而且 direct consumers 很清楚。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- filters platform slice clone shaping
- current `cloneAssetGenerationFilters(...)` consumers
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader action target clone redesign 或 generic clone framework。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 action target impact clone layering

那条线目前已经没有同等级的 mixed final owner 还留着。

#### 2. 不建议直接重开 action target aggregate routing

当前 action target aggregate router 仍然很清晰。

#### 3. 不建议直接重开 action execute orchestration

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 60: ListingKit action target filters platform slice ownership`
