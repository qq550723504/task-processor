# Amazon Marketplace

This directory is the target home for Amazon-as-marketplace behavior.

Owns:

- Amazon publishing rules
- Amazon workspace or editor rules
- Amazon marketplace DTOs and domain models
- Amazon-specific validation and adaptation logic

Does not own:

- Amazon crawling or source extraction
- Generic listing orchestration
- Shared product facts

Near-term migration candidates:

- `internal/amazonlisting` platform-specific publishing logic
- Amazon-target behavior currently mixed into `internal/amazon`
