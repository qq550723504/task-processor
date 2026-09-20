// Wire consumer of src2b-acquisition-v1 and SRC-2B2 R1, not Product authority.
export type Attribute = { name: string; value: string };
export type Price = { amount: string; currency: string | null; minQuantity?: string | null };
export type Variant = { sourceID: string | null; sku: string | null; title: string | null; attributes: Attribute[]; price: Price | null };
export type Evidence = {
  schemaVersion: 1; sourceURL: string; offerID: string; title: string | null; description: string | null;
  attributes: Attribute[]; variants: Variant[]; priceFacts: Price[]; images: { url: string; role: string }[];
  capturedAt: string; contentSHA256: string; parserVersion: string;
  warnings: { code: string; field: string }[]; missingFacts: { field: string; reason: string }[];
};
export type CapturePayload = { captureVersion: 1; evidence: Evidence };
