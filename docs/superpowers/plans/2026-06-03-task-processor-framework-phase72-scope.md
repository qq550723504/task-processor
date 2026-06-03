## Task Processor Framework Phase 72 Scope

### Target

Continue the new SHEIN freshness framework line by narrowing:

- [evaluateSheinSaleAttributeFreshnessResolution(...)](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:30)

### Why This Slice

After `Phase 71`, the aggregate owner is already thinner, but the resolution home still directly mixes several responsibilities:

- template-presence issue detection
- invalid sale-attribute collection
- repair retry routing
- issue sorting and outward message shaping

That makes it the clearest remaining mixed-owner node in this new hotspot.

### Desired Outcome

At the end of `Phase 72`:

- the resolution home should move closer to `routing-only` or `near-routing-only`
- at least one coherent sub-responsibility should have its own local owner
- boundary coverage should lock the new split so the mixed-owner shape does not regrow
