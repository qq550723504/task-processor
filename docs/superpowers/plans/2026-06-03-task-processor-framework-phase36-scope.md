## Task Processor Framework Phase 36 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit shared queue/retry clone helper ownership`。

也就是继续处理当前仍然挂在：

- [service_generation_actions.go](/D:/code-task-processor/internal/listingkit/service_generation_actions.go:1)

里的两条 shared helper：

- `cloneGenerationQueueQuery(...)`
- `cloneRetryGenerationTasksRequest(...)`

### Why This Before Other Options

#### 1. Action execute handoff 本地 seams 已经够清楚了

现在 handoff 这一侧已经稳定成：

- branch invocation owner
- branch request-shaping owner
- branch result owner
- local result-dispatch owner
- unified normalization / result-shape / adaptation owner

继续在 handoff 本地追求更细对称性，收益已经明显下降。

#### 2. Shared clone helper 已经成为真实的 multi-consumer hotspot

当前这些 helper 已经被多条路径共同依赖，包括：

- action execute handoff
- action target clone
- review navigation / navigation dispatch

这说明它们不再只是“顺手放在 service 文件里的小工具”，而是已经演变成了明确的 shared ownership seam。

#### 3. 这个切片虽然跨 consumer，但仍然可以保持 bounded

这一步可以只围绕：

- shared clone helper home
- 当前直接 consumer 的路由关系
- outward behavior stability

来做，不需要扩大到 broader navigation or service rewrite。

### Explicitly Not Recommended Next

#### 1. 不建议再回头深挖 action execute handoff 本地 request/result seams

这些 seam 现在已经足够窄，再继续拆更像为了对称性而拆。

#### 2. 不建议直接重开 generic helper framework

下一步应该仍然是 feature-local ownership move，而不是抽出通用 cloning framework。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 36: ListingKit shared queue/retry clone helper ownership`
