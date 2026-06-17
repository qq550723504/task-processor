# Amazon Crawler

This directory is the target home for Amazon-as-source crawling adapters.

Current implementation:

- `processor.go` adapts the legacy Amazon crawler processor behind a small source interface.
- The adapter owns raw crawler invocation for single and batch requests.
- `NewLegacyCrawlSource` owns concrete legacy processor construction so app/bootstrap packages do not import the legacy crawler root directly.
- Product-facing request planning remains in `internal/product/sourcing`.

Owns:

- page fetch and browser automation
- source-specific parsing and extraction
- raw crawler result assembly

Does not own:

- Amazon marketplace publishing rules
- listing submission or preview orchestration
- normalized product sourcing handoff logic

The normalized handoff belongs in `internal/product/sourcing`.
