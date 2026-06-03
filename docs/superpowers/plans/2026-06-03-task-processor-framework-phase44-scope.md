## Task Processor Framework Phase 44 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation descriptor residual pairing ownership`。

也就是继续处理：

- [generation_navigation_descriptor_residual_shape.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_residual_shape.go:1)

里还留着的 `Conditional + Invalidates + DispatchPlan` residual pairing。

### Why This Before Other Options

#### 1. Mixed descriptor shape 已经收干净了

现在：

- follow-up read clone 已有独立 home
- dispatch-plan step clone 已有独立 home
- residual descriptor shape 也已经独立出新的 local home

继续在 mixed descriptor shape 这一层追求更细对称性，收益已经明显下降。

#### 2. Residual pairing 是下一个最自然的小切口

当前 residual shape seam 仍然同时知道：

- conditional clone
- invalidates slice clone
- dispatch-plan clone delegation

这已经是很明确的 residual ownership hotspot，而且写面还足够小。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- residual descriptor pairing
- existing nested clone home dispatch
- current consumer-visible behavior stability

来做，不需要马上扩大到 descriptor builder 或 navigation dispatch flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议继续深挖 follow-up read clone home

那个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader descriptor builder / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 44: ListingKit navigation descriptor residual pairing ownership`
