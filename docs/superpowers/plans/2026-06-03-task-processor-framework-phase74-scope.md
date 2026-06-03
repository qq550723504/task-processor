## Task Processor Framework Phase 74 Scope

### Target

Review whether the remaining base-check logic inside:

- [evaluateSheinSaleAttributeFreshnessResolution(...)](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:28)

still justifies another refactor phase.

### Why This Scope

After `Phase 73`, the resolution layer is already close to routing-only:

- invalid-state / repair routing has its own home
- message shaping has its own home

The remaining logic is small and semantically coherent enough that another phase may be cosmetic rather than high-signal.

### Desired Outcome

At the end of `Phase 74`:

- either a real remaining hotspot is identified in this SHEIN freshness line
- or this sub-line is declared practically complete and the next work should move to another hotspot
