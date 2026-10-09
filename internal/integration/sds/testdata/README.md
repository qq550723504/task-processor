`qualified-free.json` contains the field shapes observed through GETs for the
2026-10-09 controlled SDS test operation
`12752596-6056-4316-9f2f-380c97df9675`.

Unrelated account fields were removed. Merchant IDs and image paths were replaced
with test values. Provider IDs, types, dimensions, Fabric fields, task counts and
the distinction between list and detail responses were retained. No login state,
tokens, signed upload fields or source image bytes are included.

The current `prototypeGroup` exposes `productId` and `id`; it does not expose
`designLayerNum`. The authoritative supported image-region list is `layers`.
This fixture verifies decoding; it is not execution of the formal application.
