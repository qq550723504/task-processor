## Task Processor Framework Phase 77 Scope

### Target

Continue the SHEIN attribute freshness framework line by narrowing:

- [evaluateSheinResolvedAttributeFreshness(...)](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:12)

### Why This Slice

After `Phase 76`, collection logic has its own local home.

The remaining mixed ownership is now concentrated around:

- drift-detail shaping
- outward issue aggregation
- final message assembly

That makes a message-shape split the clearest next cut.

### Desired Outcome

At the end of `Phase 77`:

- the local evaluation home should move closer to `near-routing-only`
- message shaping should have a distinct local owner
- boundary coverage should lock the new split
