# Task 1: Finish SDS product_size propagation into SHEIN publishing size attributes

## Goal

Complete the current refactor slice that carries SDS `product_size` from the ListingKit generation request into the SHEIN publishing package and materializes it as SHEIN preview/draft `size_attribute_list`.

## Existing Working Tree

The checkout already has in-progress edits in:

- `internal/listingkit/assembler.go`
- `internal/listingkit/assembler_test.go`
- `internal/publishing/shein/assembler.go`
- `internal/publishing/shein/model.go`
- `internal/publishing/shein/preview_adapter.go`
- `internal/publishing/shein/size_attribute.go`
- `internal/publishing/shein/size_attribute_test.go`

Treat these as existing user/controller work. Do not revert unrelated changes. Finish and correct this slice.

## Architectural Constraints

- `internal/listingkit` remains a thin facade/orchestration package.
- New SHEIN publishing behavior belongs in `internal/publishing/shein`.
- Do not put size-table parsing, sale-attribute matching, or SHEIN size attribute rules into root `internal/listingkit`.
- Reuse existing local types and helpers where available. Do not introduce a new parsing framework for this narrow JSON table shape.
- Preserve persisted/request JSON compatibility except for intentionally adding the needed field.

## Functional Requirements

- `GenerateRequest.Options.SDS.ProductSize` is copied into the SHEIN `BuildRequest` used by ListingKit assembly.
- SHEIN package assembly parses SDS `product_size`, maps supported apparel measurement headers to SHEIN size attribute IDs, and relates measurements to the resolved SKU sale attribute value IDs.
- The preview payload exposes `SizeAttributeList` so downstream UI/submit normalization can see the result.
- Empty, malformed, unsupported, or unmatched product-size data should be ignored without breaking package assembly.
- The implementation should keep nil safety for package/draft payload handling.

## Testing Requirements

- Add or keep focused tests for:
  - ListingKit request construction copying SDS `ProductSize`.
  - Parsing structured SDS product size into SHEIN size attributes.
  - Preview product copying `SizeAttributeList`.
  - Assembler build applying structured product size into the preview payload.
- Run focused Go tests covering changed packages.
- If broader tests fail for unrelated existing reasons, report exact failures and continue only if the changed slice is verified.

## Verification Commands

Prefer these first:

```powershell
go test ./internal/publishing/shein -run "TestBuildSizeAttributes|TestBuildPreviewProductIncludesSizeAttributeList|TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload" -count=1
go test ./internal/listingkit -run "TestBuildSheinPublishRequestIncludesSDSProductSize|TestBuildSheinPublishRequestForTaskIncludesTaskIdentity" -count=1
go test ./internal/publishing/shein ./internal/listingkit -run "TestBuildSizeAttributes|TestBuildPreviewProductIncludesSizeAttributeList|TestAssemblerBuildAppliesStructuredProductSizeToPreviewPayload|TestBuildSheinPublishRequestIncludesSDSProductSize|TestBuildSheinPublishRequestForTaskIncludesTaskIdentity" -count=1
```

Also run `gofmt` on changed Go files before reporting.

