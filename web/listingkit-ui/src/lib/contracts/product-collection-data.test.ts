import { describe, expect, it } from "vitest";
import { parseCollectionResponse } from "./product-collection";

describe("data-service original collection references", () => {
  const id = "0511e1d2-b555-4970-b323-3b629a901901";
  const createdAt = "2026-10-09T12:00:00Z";
  for (const kind of ["amazon_data", "custom_dataset"]) {
    it(`reads ${kind} without masquerading as another source`, () => {
      expect(parseCollectionResponse("batches", { items: [{ id, name: "Data delivery", kind, revision: 1, count: 1, createdAt }], total: 1 })).not.toBeNull();
      expect(parseCollectionResponse("items", { items: [{ id, batchId: id, revision: 1, createdAt, source: { productKey: "fixture", publicationId: id, version: "1", operationId: id, kind } }], total: 1 })).not.toBeNull();
    });
  }
  it("keeps unknown producer kinds rejected", () => {
    expect(parseCollectionResponse("batches", { items: [{ id, name: "Unknown", kind: "legacy_amazon", revision: 1, count: 1, createdAt }], total: 1 })).toBeNull();
  });
});
