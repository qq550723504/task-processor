# PAY-013 ListingKit Asset Isolation Design

## Goal

Make ListingKit image uploads private and tenant-scoped so an authenticated
tenant can upload, read, and delete only its own assets. The change is limited
to ListingKit upload APIs and their local/S3 storage adapters.

## Scope

Included:

- `POST /uploads/images`, uploaded-image reads, and uploaded-image deletes.
- ListingKit local and S3 image-upload stores.
- The persisted uploaded-image metadata mapping.
- ListingKit UI proxying and imgproxy source URLs.
- Image MIME, magic-byte, size, decode, and allowed-format validation.

Excluded:

- Non-ListingKit product-image publishers and unrelated S3 clients.
- Direct-to-S3 browser upload.
- Subscription pricing, storage quota policy, or billing-ledger changes.

## Design

### Authoritative ownership

The authenticated ListingKit request context is the only source of the tenant.
The service derives its numeric legacy tenant ID before allocating a key or
consulting metadata. Callers never supply a tenant ID or a storage key as an
authority boundary.

Each saved upload receives a server-generated key under a tenant namespace:

```text
listingkit/tenants/<legacy-tenant-id>/uploads/<uuid>.<extension>
```

The server stores the resolved key and tenant ID in `UploadedImageRecord`.
Read and delete look up the record by both its opaque upload ID and the
current tenant. A missing, foreign, or malformed identifier returns the same
not-found response and never reaches the object store.

### Private delivery

S3 objects are stored without public ACLs. The ListingKit backend remains the
single reader: an authenticated GET route loads the tenant-owned record,
opens the object, validates its stored media metadata, and streams it with a
safe response content type. Browser and imgproxy consumers use that protected
ListingKit route rather than direct bucket URLs or arbitrary external sources.

Local development follows the same record and ownership checks, so local and
S3 behavior remain equivalent.

### Upload validation and atomicity

Before storage, the service enforces the existing byte limit, rejects empty
payloads, detects content from bytes rather than trusting the multipart
header, decodes the image, and allows only the explicitly supported raster
formats. The supplied filename only contributes a sanitized extension after
the detected format is known.

The service first writes the private object, then saves metadata. If metadata
save fails, it deletes the just-written object. If object deletion fails, the
operation returns an error with enough structured logging for reconciliation;
it never reports a successful upload. No URL is returned until both writes
succeed.

### Idempotent deletion

Delete resolves the tenant-owned metadata record first. It removes the object
and then marks/removes metadata in a repeat-safe transaction boundary. A
second delete receives the same successful absent-result response and cannot
refund usage twice. Object-not-found during cleanup is treated as already
deleted, while unrelated object-store failures leave metadata recoverable for
retry.

## Error contract

- Invalid image input: HTTP 400 with `invalid_request`.
- Missing or foreign upload ID: HTTP 404 with `uploaded_image_not_found`.
- Storage or metadata failure: HTTP 500 with a stable upload/delete failure
  code and no object key, bucket name, or cross-tenant detail.

## Tests and acceptance

Focused tests must prove:

1. Tenant A cannot read or delete tenant B's uploaded image, and the storage
   adapter is not called after the metadata ownership lookup rejects it.
2. A mismatched multipart `Content-Type` cannot bypass magic-byte/decode
   validation.
3. Repeated deletion is idempotent and does not run usage-refund logic twice.
4. Metadata persistence failure removes the newly stored object; an upload
   never leaves a successful record or response when either write fails.
5. Local and S3 adapters generate only server-owned tenant-scoped keys and
   never publish direct public object URLs.

The PAY-013 checkbox is updated only after the targeted ListingKit API,
service, storage, and UI proxy test suites pass.
