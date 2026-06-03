## Task Processor Framework Phase 49 Scope Recommendation

### Recommendation

下一步更值得做的是 `ListingKit navigation descriptor follow-up read slice clone ownership`。

也就是继续处理：

- [generation_navigation_descriptor_followup_read_routing_pairing.go](/D:/code-task-processor/internal/listingkit/generation_navigation_descriptor_followup_read_routing_pairing.go:1)

里现在还留着的 `slice orchestration + item clone home dispatch`。

### Why This Before Other Options

#### 1. Follow-up read routing home 已经收干净了

现在：

- routing home 已只保留 pairing home dispatch
- item clone 已有独立 home
- pairing home 成了当前链路里下一个最明显的 mixed home

继续在 routing 这一层追求更细对称性，收益已经明显下降。

#### 2. Slice clone ownership 是下一个最自然的小切口

当前 pairing home 仍然同时知道：

- follow-up read slice orchestration
- follow-up read item clone home dispatch

这已经是很明确的 ownership hotspot，而且写面很小。

#### 3. 这个切片仍然足够 bounded

下一步可以只围绕：

- follow-up read slice clone
- existing item clone home
- current consumer-visible behavior stability

来做，不需要马上扩大到 broader descriptor clone entry 或 navigation dispatch flow 本身。

### Explicitly Not Recommended Next

#### 1. 不建议回头重开 follow-up read routing home

这个 ownership 问题已经解决了。

#### 2. 不建议直接重开 broader descriptor clone entry / navigation dispatch flow

那会显著扩大写面，超出当前最自然的小切片。

### Proposed Next Phase Name

建议下一阶段命名为：

`Phase 49: ListingKit navigation descriptor follow-up read slice clone ownership`
