## Task Processor Framework Phase 47 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation descriptor clone-shape pairing ownership`。

也就是继续处理：

- [generation_navigation_descriptor_clone_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_clone_shape.go:1)

里现在还留着的两个 local dispatch pairing。

### Why This Before Other Options

#### 1. Clone-shape content routing 已经收干净了

现在：

- residual shape 已有独立 home
- follow-up read routing 也已有独立 home
- clone-shape home 现在主要只剩两个 local dispatch 的 pairing

继续在 content routing 这一层追求更细对称性，收益已经明显下降。

#### 2. Clone-shape pairing 是下一个最自然的小切口

当前 clone-shape home 仍然同时知道：

- residual shape dispatch
- follow-up read routing dispatch

这已经是很明确的 orchestration hotspot，而且写面很小。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- clone-shape pairing
- existing local clone homes
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader descriptor builder 或 navigation dispatch flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 residual shape 或 follow-up read routing home

这两个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader descriptor builder / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 47: ListingKit navigation descriptor clone-shape pairing ownership`
