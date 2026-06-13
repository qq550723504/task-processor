# Amazon Crawler

This directory is the target home for Amazon-as-source crawling adapters.

Owns:

- page fetch and browser automation
- source-specific parsing and extraction
- raw crawler result assembly

Does not own:

- Amazon marketplace publishing rules
- listing submission or preview orchestration
- normalized product sourcing handoff logic

The normalized handoff belongs in `internal/product/sourcing`.
