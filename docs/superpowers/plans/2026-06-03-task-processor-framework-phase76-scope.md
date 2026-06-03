## Task Processor Framework Phase 76 Scope

### Target

Continue the SHEIN attribute freshness framework line by narrowing:

- [evaluateSheinResolvedAttributeFreshness(...)](/D:/code/task-processor/internal/listingkit/shein_submit_attribute_freshness_evaluation.go:38)

### Why This Slice

After `Phase 75`, the aggregate owner is thinner, but the local evaluation home still directly mixes several responsibilities:

- invalid resolved-attribute detection
- required-attribute reactivation logic
- drift-detail shaping
- outward issue/message shaping

That makes it the clearest remaining mixed-owner node in this new hotspot.

### Desired Outcome

At the end of `Phase 76`:

- the local evaluation home should move closer to `routing-only` or `near-routing-only`
- at least one coherent sub-responsibility should have its own local owner
- boundary coverage should lock the new split
