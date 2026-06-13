# Product Sourcing Inventory

> Status: initial inventory for moving source-input ownership toward `internal/product/sourcing` and `internal/integration/crawler/*`.

## 1. Purpose

This document turns the approved target architecture into a concrete sourcing migration entrypoint.

It focuses on Amazon and 1688 source flows because:

- both sources already exist in the codebase,
- both still rely on legacy crawler package roots,
- `internal/product/sourcing` exists as a target skeleton but is still nearly empty,
- broad crawler moves would be too risky without a thinner first slice.

## 2. Approved Target Split

The intended steady-state split is:

```text
internal/integration/crawler/amazon   -> source access, browser/fetch runtime, raw extraction
internal/integration/crawler/a1688    -> source access, browser/fetch runtime, raw extraction
internal/product/sourcing             -> normalization, reusable source contracts, handoff to product facts
internal/listing/* or marketplace/*   -> downstream usage of normalized sourced facts
```

## 3. Current Evidence

### 3.1 Legacy crawler roots are still the real owners

Current size snapshot:

- `internal/crawler/amazon`: 111 files
- `internal/crawler/alibaba1688`: 39 files

This confirms that the target skeleton exists, but the real implementation mass still lives in legacy crawler roots.

### 3.2 `internal/product/sourcing` is still only a skeleton

Current contents:

- `internal/product/sourcing/doc.go`
- `internal/product/sourcing/README.md`

There is not yet a real sourcing contract, normalization service, or handoff model living there.

### 3.3 Existing code already mixes adapter and sourcing ownership

Observed examples:

- `internal/productenrich/enrich/scraper_adapter.go`
  - uses `internal/crawler/alibaba1688`
  - converts raw 1688 crawler output into `productenrich.ScrapedData`
  - this is closer to normalized sourcing handoff than to crawler runtime ownership
- `internal/infra/productcrawler/crawler_repository_impl.go`
  - uses `internal/crawler/amazon`
  - wraps Amazon crawler access behind product-facing fetch behavior
  - this mixes source adapter wiring and product-facing sourcing handoff
- `internal/processor/crawler_processor.go`
  - coordinates crawl task execution and downstream product fetch behavior
  - this is runtime/task orchestration, not pure crawler ownership

## 4. Ownership Classification

### 4.1 Adapter-side concerns

These should trend toward `internal/integration/crawler/*`:

- browser automation
- page fetch behavior
- anti-bot adaptation
- source-specific parsing
- raw extraction result assembly

### 4.2 Sourcing-side concerns

These should trend toward `internal/product/sourcing`:

- source result normalization
- conversion from raw source payloads into reusable product facts
- source-to-product handoff contracts
- enrichment-ready intermediate models
- reusable mapping from source variants/images/specs into product-facing structures

### 4.3 Downstream consumers

These should remain outside crawler adapters and outside product sourcing:

- listing preview / workflow orchestration
- marketplace publishing payload rules
- runtime worker bootstrapping

## 5. Recommended First Extraction Slice

Do not start by moving the whole Amazon crawler tree.

Start with the smallest slices that already represent sourcing ownership:

### Slice A: 1688 normalized handoff

Current status: first behavior-preserving extraction started.

Primary candidate:

- `internal/productenrich/enrich/scraper_adapter.go`

Why this is a good first slice:

- it already sits above raw crawler extraction,
- it already converts source output into reusable product-oriented structures,
- it is much smaller and safer than moving the underlying crawler packages first.

Target direction:

- move the normalization logic toward `internal/product/sourcing`,
- keep the actual 1688 crawling implementation in crawler/integration ownership,
- leave a thin adapter in the old location if downstream imports still need it.

Implemented first step:

- `internal/product/sourcing/a1688_scraped_data.go` now owns conversion from `crawler/alibaba1688/model.Product1688` into `productenrich.ScrapedData`;
- `internal/productenrich/enrich/scraper_adapter.go` now only invokes the 1688 crawler and delegates normalized handoff conversion;
- coverage for variant dimensions, variants, fallback images, and variant price mapping now lives with `internal/product/sourcing`.

Implemented second step:

- `internal/integration/crawler/a1688/processor.go` now owns the raw 1688 crawler invocation adapter;
- `internal/productenrich/enrich/scraper_adapter.go` now depends on the integration adapter instead of directly constructing the legacy Alibaba 1688 processor;
- the legacy `internal/crawler/alibaba1688` import is now kept at the 1688 integration boundary for this product-enrichment source path.

### Slice B: Amazon product-facing crawler repository

Current status: first behavior-preserving extraction started.

Primary candidate:

- `internal/infra/productcrawler/crawler_repository_impl.go`

Why this is a good second slice:

- it is one of the clearest seams between Amazon source crawling and product-facing consumption,
- it can be split into:
  - adapter/runtime-facing crawler dependency construction,
  - product-facing sourcing handoff behavior.

Target direction:

- keep direct crawler access near `integration/crawler/amazon`,
- move product-facing source fetch and normalization concerns toward `internal/product/sourcing`,
- keep runtime bootstrap and task workers separate from both.

Implemented first step:

- `internal/product/sourcing/amazon_crawl_requests.go` now owns conversion from `product.FetchRequest` plus product IDs into raw Amazon crawler `model.ProductRequest` values;
- `internal/infra/productcrawler/crawler_repository_impl.go` now delegates single and batch crawler request planning to `sourcing.AmazonCrawlRequestPlanner`;
- Amazon processor invocation remains in the infra adapter, so this step does not move browser/fetch/runtime ownership.

Implemented second step:

- `internal/integration/crawler/amazon/processor.go` now owns the raw Amazon crawler invocation adapter for single and batch requests;
- `internal/infra/productcrawler/crawler_repository_impl.go` now delegates request execution through the integration adapter instead of calling the legacy Amazon processor directly;
- the old repository constructor still accepts the legacy processor to keep bootstrap call sites stable while the target package starts carrying real adapter code.

Implemented third step:

- `internal/infra/productcrawler/crawler_repository_impl.go` now accepts `integration/crawler/amazon.Source` instead of concrete `*crawler/amazon.AmazonProcessor`;
- legacy Amazon default-zipcode policy moved to `integration/crawler/amazon.ZipcodePolicy`;
- `internal/infra/productcrawler` no longer imports the legacy Amazon crawler package directly.

Implemented fourth step:

- `internal/product/sourcing/amazon_crawl_requests.go` no longer imports `internal/product`, so both `internal/product` and `internal/infra/productcrawler` can reuse the planner without an import cycle;
- `internal/product/product_fetcher.go` now uses `sourcing.AmazonCrawlRequestPlanner` for local crawler URL and zipcode planning;
- explicit and configured-default zipcode behavior is covered by product fetcher tests.

Implemented fifth step:

- `internal/product/sourcing/amazon_source_fetcher.go` now owns the product-side execution of planned Amazon source fetches;
- `internal/product/product_fetcher.go` now delegates crawler fetch execution to `sourcing.AmazonSourceFetcher`;
- `internal/product` remains responsible for cache orchestration while source planning and source execution live in `internal/product/sourcing`.

Implemented sixth step:

- `sourcing.AmazonSourceFetcher` now also owns batch source execution through optional batch-capable sources;
- `internal/infra/productcrawler/crawler_repository_impl.go` now delegates both single and batch source execution to `sourcing.AmazonSourceFetcher`;
- `internal/integration/crawler/amazon.Processor` exposes URL/zipcode and batch context methods so it can be used as a source adapter by product sourcing.

Implemented seventh step:

- `sourcing.AmazonCrawlRequestPlanner.ResolveZipcode` now exposes Amazon source zipcode resolution as reusable sourcing behavior;
- `internal/app/crawler/fetcher/remote_fetcher.go` now reuses the sourcing zipcode planner for remote crawler API payloads;
- configured Amazon default zipcodes are now honored by the remote crawler API fetch path instead of being duplicated only in local crawler planning.

Implemented eighth step:

- `internal/product/sourcing/amazon_source_platform.go` now owns source-platform to crawler-platform mapping for Amazon-backed product sources;
- `internal/app/crawler/fetcher/distributed_fetcher.go` now delegates SHEIN/TEMU to Amazon crawler queue mapping and crawler-source support checks to `product/sourcing`;
- distributed crawler task construction remains in app runtime ownership, while source identity rules move toward product sourcing.

Implemented ninth step:

- `internal/product/sourcing/source_request.go` now owns the reusable source-request shape used to derive variant source requests;
- local, remote API, and distributed variant fetch paths now preserve source-scoped fields such as explicit zipcode when fetching variant products;
- app runtime code still owns cache checks, task IDs, priorities, and queue submission, while source request inheritance lives in `product/sourcing`.

Implemented tenth step:

- `internal/app/crawler/distributed.CrawlRequest` now carries an optional `zipcode` field for crawler workers;
- distributed single and variant task construction now forwards the inherited source zipcode into the published crawl message;
- the distributed client only emits `zipcode` when non-empty, keeping legacy no-zipcode task payloads unchanged.
- `model.Task` and `processor.CrawlerProcessor` now consume the distributed `zipcode` payload and pass it into `product.FetchRequest`.

Implemented eleventh step:

- `internal/product/source_request.go` now owns conversion between `product.FetchRequest` and `sourcing.SourceRequest`;
- local, remote API, and distributed variant fetch paths now reuse the same conversion helper instead of maintaining per-fetcher field-copy code;
- `internal/product/sourcing` remains free of an import back to `internal/product`, keeping the dependency direction one-way.

Implemented twelfth step:

- `processor.CrawlerProcessor` now resolves worker-side crawler platforms through `sourcing.CrawlerPlatformForSource`;
- legacy distributed tasks that only carry `platform=shein.crawler` or `platform=temu.crawler` now fall back to Amazon-backed crawler identity consistently with distributed request construction;
- `model.Task` remains a plain transport/domain model and does not import product sourcing rules.

Implemented thirteenth step:

- `internal/processor.NewCrawlerProcessor` no longer requires the legacy `*crawler/amazon.AmazonProcessor` argument;
- `internal/processor.CrawlerProcessor` now depends on `product.ProductFetcher` plus runtime publisher/submitter collaborators, matching its actual behavior;
- the deprecated `internal/app/processor` compatibility layer still accepts the old argument while forwarding to the new constructor.

Implemented fourteenth step:

- `internal/app/consumer` shared runtime state now exposes crawl capability through `runner.CrawlSource` instead of concrete `*crawler/amazon.AmazonProcessor`;
- consumer and bootstrap product-fetcher builders now accept the neutral crawl-source interface already used by the fetcher factory;
- concrete Amazon processor creation remains at the bootstrap/creator edge while platform modules and runtime context depend on crawl capability.

Implemented fifteenth step:

- `internal/app/bootstrap/resources.SharedResources` now exposes the shared crawler dependency as `runner.CrawlSource`;
- `internal/app/bootstrap` app service wiring no longer imports the legacy Amazon crawler package directly;
- concrete Amazon processor construction remains inside the shared-resource factory, keeping old crawler runtime creation at a single bootstrap edge.

Implemented sixteenth step:

- `internal/app/consumer.AmazonCrawlerCreator` now returns `runner.CrawlSource` instead of concrete `*crawler/amazon.AmazonProcessor`;
- bootstrap still constructs the concrete Amazon processor, but consumer registry contracts now depend only on crawl capability;
- this leaves `GetSharedAmazonProcessor` as an explicit compatibility escape hatch rather than the default shared dependency shape.

## 6. What To Avoid

Do not:

- move all `internal/crawler/amazon` files in one PR
- move raw crawler extraction and downstream listing behavior in the same change
- let new Amazon or 1688 source normalization keep accumulating in `internal/listingkit`
- treat marketplace packages as the owner of crawler extraction logic

## 7. Near-term Success Criteria

This sourcing track is moving in the right direction when:

- new Amazon or 1688 source-adapter work lands under `internal/integration/crawler/*`
- new normalization and source-handoff logic lands under `internal/product/sourcing`
- legacy crawler roots begin shrinking without forcing a one-shot rename
- downstream listing and marketplace flows depend on normalized sourced facts instead of raw crawler package internals
