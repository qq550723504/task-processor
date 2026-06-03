## Task Processor Framework Phase 48 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation descriptor follow-up read routing pairing ownership`。

也就是继续处理：

- [generation_navigation_descriptor_followup_read_routing.go](/D:/code/task-processor/internal/listingkit/generation_navigation_descriptor_followup_read_routing.go:1)

里现在还留着的两个 local responsibilities pairing。

### Why This Before Other Options

#### 1. Clone-shape pairing 已经收干净了

现在：

- clone-shape home 已只保留 pairing home dispatch
- residual shape 已有独立 home
- follow-up read routing 现在成了当前链路里下一个最明显的 mixed home

继续在 clone-shape 这一层追求更细对称性，收益已经明显下降。

#### 2. Follow-up read routing pairing 是下一个最自然的小切口

当前 follow-up read routing home 仍然同时知道：

- follow-up read slice clone orchestration
- follow-up read item clone home dispatch

这已经是很明确的 ownership hotspot，而且写面很小。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- follow-up read routing pairing
- existing item clone home
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader descriptor clone entry 或 navigation dispatch flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 clone-shape pairing home

这个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader descriptor clone entry / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 48: ListingKit navigation descriptor follow-up read routing pairing ownership`
