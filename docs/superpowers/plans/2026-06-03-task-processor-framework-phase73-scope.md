## Task Processor Framework Phase 73 Scope

### Target

Continue the SHEIN freshness framework line by narrowing:

- [evaluateSheinSaleAttributeFreshnessResolution(...)](/D:/code/task-processor/internal/listingkit/shein_submit_sale_attribute_freshness_evaluation.go:30)

### Why This Slice

After `Phase 72`, the invalid-state and repair logic has its own local home.

The remaining mixed ownership is now concentrated around:

- base issue detection
- outward issue aggregation
- final message shaping

That is a cleaner, smaller, and more coherent next cut than jumping elsewhere inside the file.

### Desired Outcome

At the end of `Phase 73`:

- resolution home should move closer to `routing-only` or `near-routing-only`
- message shaping or issue aggregation should have a distinct local owner
- boundary coverage should lock the new split
