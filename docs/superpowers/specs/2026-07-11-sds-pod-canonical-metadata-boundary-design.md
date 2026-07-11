# SDS POD Canonical Metadata Boundary Design

## Status

Approved design direction for the next ListingKit ownership-reduction slice.

## Goal

Move deterministic SDS POD source-to-canonical metadata mapping out of root internal/listingkit into internal/product/sourcing/sdspod while preserving current production behavior except for the approved anonymous successful variant product-image union refinement described below.

ListingKit remains the compatibility and orchestration shell. The new package owns platform-neutral SDS POD normalization for canonical titles, identity attributes, Studio style metadata, rendered mockup images, variant image assignment, and field traces.

## Problem

internal/listingkit/sds_canonical_metadata.go currently owns approximately 330 lines of deterministic source normalization:

- selecting trusted SDS product and variant identity;
- promoting SDS SKU, size, and color into canonical attributes;
- applying the Studio style dimension;
- converting rendered mockups into canonical images;
- assigning variant-specific images;
- recording canonical source traces.

These rules do not require ListingKit task state, repositories, remote clients, Temporal, HTTP, or persistence ordering. Their presence in ListingKit is therefore an ownership problem rather than merely a file-size problem.

One compatibility concern is mixed into the current logic: canonical variant image lookup recognizes a source SDS SKU recovered from a historically decorated SHEIN supplier SKU. That target-specific parsing must not move into a platform-neutral source package.

## Prior Art and Reuse

Use two mature architecture patterns:

- an anti-corruption layer in ListingKit converts legacy DTOs and compatibility conventions into a narrow input;
- a functional core in internal/product/sourcing/sdspod applies deterministic canonical transformations.

Reuse canonical.Product, canonical.Attribute, canonical.Image, and canonical.FieldTrace. Reuse current behavior as the characterization oracle except for the approved anonymous successful variant product-image union refinement. Do not introduce a mapping framework, generic rule engine, reflection-based copier, or dependency injection container.

## Target Architecture

~~~text
internal/listingkit
  -> retains SDSSyncSummary and SDSSyncOptions compatibility DTOs
  -> prepares platform-neutral canonical metadata input
  -> prepares historical variant lookup keys
  -> delegates deterministic mutation

internal/product/sourcing/sdspod
  -> owns SDS POD source identity normalization
  -> owns canonical title/attribute/style mutation
  -> owns rendered mockup normalization
  -> owns canonical variant image assignment
  -> owns SDS POD canonical field traces
  -> does not know ListingKit, SHEIN, runtime, or remote APIs
~~~

## Package API

Create internal/product/sourcing/sdspod with this public contract:

~~~go
package sdspod

type CanonicalMetadata struct {
	ProductName   string
	ProductSKU    string
	VariantSKU    string
	VariantSize   string
	VariantColor  string
	StyleName     string
	Attributes    map[string]string
	MockupURLs    []string
	Variants      []VariantMetadata
	VariantLookup []VariantLookup
}

type VariantMetadata struct {
	SKU        string
	Color      string
	Status     string
	MockupURLs []string
}

type VariantLookup struct {
	CanonicalVariantIndex int
	Keys                  []string
}

func ApplyCanonical(product *canonical.Product, metadata CanonicalMetadata) bool
~~~

CanonicalVariantIndex addresses the existing product.Variants slice directly. ListingKit builds lookup keys for each index before calling the pure package. This avoids copying canonical variants into a second model and preserves deterministic assignment order.

The input is passed by value. ApplyCanonical does not retain slices or maps supplied by the caller.

## ListingKit Compatibility Adapter

Root ListingKit keeps one small adapter with these responsibilities:

1. Convert SDSSyncSummary into product identity, default mockups, and VariantMetadata values.
2. Convert the relevant SDSSyncOptions fields into ProductName, StyleName, and flattened Attributes.
3. Build VariantLookup values from the existing canonical variants.
4. Preserve historical lookup keys in their existing order:
   - source_sds_sku attribute;
   - source SDS SKU recovered from a decorated SHEIN supplier SKU;
   - canonical variant SKU;
   - Color attribute;
   - color attribute.
5. Call sdspod.ApplyCanonical and return its changed result.

The SHEIN compatibility parser remains in ListingKit for this slice. Moving or removing that historical convention requires a separate design because it affects persisted and previously generated supplier SKUs.

ListingKit does not retain title, identity, style, image construction, image equality, fallback, or field-trace policy after the migration.

## Canonical Mapping Rules

The new package preserves these rules exactly except for the intentional image refinement documented in the Images section.

### Title

- Prefer metadata.ProductName.
- Ignore empty names.
- Do not rewrite an equal trimmed title.
- When changed, write the current title trace:
  - source type: canonical.SourceDerived;
  - detail: SDS design product detail;
  - confidence: 0.96;
  - inferred: false;
  - needs review: false.

### Identity attributes

Apply non-empty values for:

- caller-provided Attributes;
- sku;
- product_sku;
- variant_sku;
- variant_size;
- variant_color.

Explicit metadata identity values retain the current precedence over matching caller-provided attribute values. When any identity attribute changes, write the existing attributes field trace with detail SDS design product identity and confidence 0.96.

### Studio style

Apply non-empty StyleName to every canonical variant using the existing Studio AI style attribute key and trace:

- source type: canonical.SourceDerived;
- detail: SDS studio AI style dimension;
- confidence: 0.94;
- inferred: false;
- needs review: false.

The canonical attribute key is exactly `ai_style` and is covered by compatibility tests.

### Images

- Trim and de-duplicate URLs while preserving first-seen order.
- Mark the first image primary and later images gallery.
- Ignore failed variant results and variants with no mockups.
- Prefer variant result images for product.Images when available.
- Otherwise use default mockups, falling back to the first successful variant result.
- Assign variant images by normalized VariantLookup keys.
- Normalize lookup keys with `strings.ToLower(strings.TrimSpace(value))`; ignore empty normalized keys.
- Fall back to default images when no variant-specific image set matches.
- Preserve the existing image trace with detail SDS rendered mockup images and confidence 0.98.
- Image equality continues to compare trimmed URL and role only.
- Reapplying identical metadata returns false and does not reorder images.

**Intentional behavior refinement:** Legacy behavior used top-level default
mockups, or the first successful variant image group, when every successful
variant lacked a non-empty normalized SKU or Color key. The refactored behavior
unions all successful variant images into `product.Images` even when those
variants have no lookup key. Per-variant assignment still requires a matching
lookup key and otherwise uses the existing default fallback.

## Data Flow

~~~text
SDSSyncSummary + SDSSyncOptions + canonical.Product
  -> ListingKit converts legacy DTOs
  -> ListingKit derives compatibility lookup keys by canonical variant index
  -> sdspod.CanonicalMetadata
  -> sdspod.ApplyCanonical
       -> identity attributes
       -> Studio style
       -> rendered/default/variant images
       -> trusted title
       -> field traces
  -> unchanged canonical.Product contract continues through ListingKit workflow
~~~

## Dependency Rules

internal/product/sourcing/sdspod may import:

- Go standard library;
- task-processor/internal/catalog/canonical.

It must not import:

- internal/listingkit;
- internal/marketplace or internal/publishing;
- internal/sds runtime, client, design, workflow, or usecase packages;
- internal/app, internal/platform, internal/infra, HTTP, Gin, GORM, Temporal, or external SDKs.

Add a package-local AST import guard. The guard must scan all production Go files in the subpackage.

## Error Handling and Compatibility

ApplyCanonical returns only bool and must remain nil-safe.

- A nil product returns false.
- Empty metadata produces no change.
- Empty or duplicate URLs are ignored.
- Failed variant metadata is ignored.
- Invalid lookup indexes are ignored.
- Empty lookup keys are ignored.
- A lookup with no matching rendered variant images falls back exactly as current behavior.
- No new logging, error values, warnings, or public API responses are introduced.

The ListingKit adapter remains responsible for legacy DTO nil handling and historical SHEIN supplier SKU compatibility.

## Testing Strategy

### Characterization before movement

Preserve or add exact tests for:

- trusted title overwrite and no-op behavior;
- product and variant identity precedence;
- existing Studio attributes plus SDS identity attributes;
- style dimension propagation;
- default mockup promotion;
- multi-variant image aggregation and de-duplication;
- variant-specific assignment by SKU and color;
- failed variant exclusion;
- default-image fallback;
- repeated application idempotency;
- exact trace detail, confidence, and flags.

### New package tests

Move deterministic expectations into internal/product/sourcing/sdspod. Prefer table-driven cases and compare full canonical values where practical, not only counts.

### ListingKit facade tests

Keep focused tests proving:

- SDSSyncSummary and SDSSyncOptions are converted correctly;
- historical decorated SHEIN supplier SKU lookup still assigns the same images;
- existing workflow entrypoints receive the same canonical product except for
  the approved anonymous successful variant product-image union refinement;
- public DTOs and JSON tags remain unchanged.

### Boundary tests

The package-local guard rejects all forbidden imports. Existing catalog, asset, product-sourcing, and ListingKit architecture tests continue to run.

### Verification

Run:

~~~powershell
go test ./internal/product/sourcing/sdspod -count=1
go test ./internal/listingkit -run "TestApplySDSSyncMetadataToCanonical|TestRunStandardProductWorkflow" -count=1
go test ./internal/product/sourcing/... ./internal/catalog/... ./internal/listingkit/... ./tests/... -count=1
go test ./... -count=1
~~~

The existing working-tree go.work.sum modification is out of scope and must not be staged or changed by this work.

## Migration Sequence

1. Add the sdspod models, ApplyCanonical contract, and dependency guard using TDD.
2. Move title and identity attribute rules with exact trace tests.
3. Move Studio style mutation with exact attribute and trace tests.
4. Move mockup conversion, variant image indexing, assignment, fallback, equality, and idempotency rules.
5. Add the ListingKit DTO/lookup adapter.
6. Switch applySDSSyncMetadataToCanonical to delegate to sdspod.
7. Remove duplicated deterministic policy from ListingKit.
8. Run focused and repository-wide verification.
9. Update the active ListingKit boundary checkpoint.

No algorithm change other than the approved anonymous successful variant
product-image union refinement may be combined with package movement. If any
other current behavior appears wrong, capture it as a follow-up rather than
correcting it during extraction.

## Non-Goals

- Moving SDSSyncSummary, SDSSyncOptions, or public JSON DTOs.
- Refactoring SDS baseline validation.
- Refactoring SDS remote sync, browser/login, polling, or rendered-image collection.
- Changing SDS-to-SHEIN payload image policy.
- Changing Temporal workflows or persistence ordering.
- Changing Studio batch orchestration.
- Renaming broad SDS package trees.
- Removing historical decorated SHEIN supplier SKU compatibility.
- Adding new product facts, image policy, or business behavior beyond the
  approved anonymous successful variant product-image union refinement.

## Risks and Mitigations

### Variant image behavior drift

The current lookup mixes source and historical target compatibility keys.

Mitigation: ListingKit constructs ordered lookup keys by canonical variant index; exact facade tests lock the historical decorated SKU path.

### Trace drift

A mechanical move can accidentally alter confidence, detail strings, or review flags.

Mitigation: new tests compare complete FieldTrace values for title, attributes, style, and images.

### Slice aliasing

Reusing input or result slices could let later mutations affect canonical state.

Mitigation: ApplyCanonical copies assigned image slices and does not retain caller-owned input collections.

### SDS production regression

Canonical metadata feeds active SDS POD workflows.

Mitigation: public DTOs and workflow entrypoint contracts remain unchanged;
canonical output changes only for the approved anonymous successful variant
product-image union refinement. Run complete ListingKit and SDS-related
workflow tests before completion.

## Success Criteria

The slice is complete when:

- internal/product/sourcing/sdspod owns deterministic SDS POD canonical mapping;
- the new package imports only standard library and canonical;
- ListingKit retains only DTO adaptation, historical lookup-key compatibility, orchestration, and changed-result propagation;
- title, identity attributes, style dimension, images, traces, precedence,
  fallback, and idempotency are unchanged except for the approved anonymous
  variant product-union refinement; per-variant lookup and fallback remain
  unchanged;
- SDS remote sync, baseline validation, SHEIN payload mapping, Temporal, persistence, and public DTO files are untouched;
- focused, boundary, ListingKit, and repository tests pass;
- the active ListingKit checkpoint records the new ownership boundary.
