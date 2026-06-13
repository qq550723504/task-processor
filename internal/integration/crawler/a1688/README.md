# 1688 Crawler

This directory is the target home for 1688-as-source crawling adapters.

Owns:

- 1688 page or payload acquisition
- source-specific extraction and parsing
- raw result shaping for downstream sourcing

Does not own:

- marketplace publishing rules
- listing-task orchestration
- reusable product normalization

The normalized handoff belongs in `internal/product/sourcing`.
