## Task Processor Framework Phase 71 Scope

### Recommended Next Direction

After closing the current ListingKit action-target mutation line, the recommended next direction is:

1. `Discover a different real hotspot elsewhere in ListingKit`

### Why This Direction Now

Because the current action-target mutation line has already reached a practical stopping point:

- broader homes are routing-only or near-routing-only
- local homes are semantically coherent
- boundary coverage is broad
- further decomposition would mostly optimize symmetry, not reduce real complexity

That means the next useful framework phase should begin only after a fresh hotspot-discovery pass elsewhere in the codebase.

### What To Look For

The next candidate should have the same properties that justified the previous line:

- a broader home still directly mixing multiple distinct responsibilities
- weak or missing ownership guardrails
- repeated business semantics living in one place with poor locality
- clear maintenance or change-risk payoff from decomposition

### What Not To Do

- do not continue splitting the current action-target mutation line without new evidence
- do not create phases just to preserve numbering momentum
- do not introduce generic abstractions only for symmetry

### Desired Outcome

At the end of the next discovery step:

- either a new high-signal hotspot is identified and ranked
- or the framework work should pause and the current line should be treated as complete
