## Task Processor Framework Phase 51 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit shared queue query clone aggregate ownership`。

也就是继续处理：

- [task_generation_shared_clone.go](/D:/code-task-processor/internal/listingkit/task_generation_shared_clone.go:1)

里现在共同存在的两个 shared clone helpers：

- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

### Why This Before Other Options

#### 1. Follow-up read 这一层已经收干净了

现在：

- routing home 已只保留 pairing dispatch
- pairing home 已只保留 slice clone home dispatch
- item clone home 已只保留 top-level copy 和 shape dispatch

继续在 follow-up read 这一层追求更细对称性，收益已经明显下降。

#### 2. Shared clone helper 才是当前更真实的 aggregate hotspot

当前 shared helper home 仍然同时持有：

- queue query clone
- retry request clone

这不是“为了对称性硬拆”的热点，而是多个 consumer 已经真实依赖的 shared aggregate home。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- shared queue query clone
- existing retry request clone
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader navigation dispatch flow 或 service orchestration。

### Explicitly Not Recommended Next

#### 1. 不建议继续回头深挖 follow-up read clone shape

那会更接近形式对称，而不是解决当前最真实的热点。

#### 2. 不建议直接重开 broader descriptor clone entry / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 51: ListingKit shared queue query clone aggregate ownership`
