## Task Processor Framework Phase 53 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit shared retry request slice clone ownership`。

也就是继续处理：

- [task_generation_retry_request_clone_shape.go](/D:/code-task-processor/internal/listingkit/task_generation_retry_request_clone_shape.go:1)

里现在还共同存在的：

- `TaskIDs` slice clone
- `Slots` slice clone

### Why This Before Other Options

#### 1. Retry request aggregate 这一层已经收干净了

现在：

- retry request clone home 已只保留 top-level shallow copy
- retry request shape home 成了 shared retry clone 邻域里剩下最显眼的 mixed home

继续在 aggregate 这一层追求更细对称性，收益已经明显下降。

#### 2. Slice clone pairing 是下一个最自然的小切口

当前 retry request shape home 仍然同时知道：

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

#### 1. 不建议回头重开 shared retry request aggregate home

这个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader action execute orchestration

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 53: ListingKit shared retry request slice clone ownership`
