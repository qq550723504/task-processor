## Task Processor Framework Phase 46 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation descriptor clone shape routing ownership`。

也就是继续处理：

- [generation_navigation_descriptor_clone_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_clone_shape.go:1)

里现在还留着的 clone-shape orchestration。

### Why This Before Other Options

#### 1. Residual content ownership 已经收干净了

现在：

- residual pairing 已有独立 home
- dispatch-plan delegation 也已有独立 home
- follow-up read clone 也已有独立 home

继续在 residual content 这一层追求更细对称性，收益已经明显下降。

#### 2. Clone-shape routing 是下一个最自然的小切口

当前 descriptor clone-shape seam 仍然同时知道：

- residual shape dispatch
- follow-up read slice clone
- follow-up read clone home dispatch

这已经是很明确的 orchestration hotspot，而且写面很小。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- descriptor clone-shape routing
- existing residual / follow-up clone homes
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader descriptor builder 或 navigation dispatch flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 residual pairing seam

那个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader descriptor builder / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 46: ListingKit navigation descriptor clone shape routing ownership`
