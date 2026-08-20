# 1688 Crawler

This directory is the target home for 1688-as-source crawling adapters.

Owns:

- 1688 page or payload acquisition
- source-specific extraction and parsing
- raw result shaping for downstream sourcing
- the adapter that hides the legacy `internal/crawler/alibaba1688` processor
- `SnapshotFromLegacyProduct`, which converts the legacy crawler DTO into the
  product-owned `internal/product/sourcing.Alibaba1688ProductSnapshot`

Does not own:

- marketplace publishing rules
- listing-task orchestration
- reusable product normalization

The normalized handoff belongs in `internal/product/sourcing`. This adapter
may depend inward on that narrow snapshot contract; product sourcing must not
depend back on this integration package or on the legacy crawler model.

Current adapter:

- `processor.go` wraps a 1688 crawler source behind `Processor`.
- `NewLegacyProcessor` is the only constructor that should create the legacy Alibaba 1688 processor for product-enrichment source access.
