## Task Processor Framework Phase 54 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit shared retry request task-id and slot clone pairing ownership`。

也就是继续处理：

- [task_generation_retry_request_slice_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_retry_request_slice_clone.go:1)

里现在共同存在的：

- `TaskIDs` slice clone
- `Slots` slice clone

### Why This Before Other Options

#### 1. Retry request slice entry 这一层已经收干净了

现在：

- aggregate home 已只保留 top-level shallow copy
- shape home 已只保留 slice clone home dispatch
- slice clone home 成了 shared retry clone 邻域里剩下最显眼的 mixed home

继续在 shape entry 这一层追求更细对称性，收益已经明显下降。

#### 2. `TaskIDs / Slots` pairing 是下一个最自然的小切口

当前 slice clone home 仍然同时知道：

- `TaskIDs` slice clone
- `Slots` slice clone

这已经是很明确的 ownership hotspot，而且写面很小。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- retry request slice clone
- current direct consumers
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader action execute orchestration 或 navigation dispatch flow。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 retry request aggregate 或 shape home

这些 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader action execute orchestration

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 54: ListingKit shared retry request task-id and slot clone pairing ownership`
