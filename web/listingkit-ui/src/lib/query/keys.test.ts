import { describe, expect, it } from "vitest";

import { listingKitKeys } from "@/lib/query/keys";

describe("listingKitKeys", () => {
  it("uses an object key for SDS product filters", () => {
    expect(
      listingKitKeys.sdsProducts({
        keyword: "shirt",
        page: 2,
        size: 24,
        shipmentArea: "US",
        categoryId: 123,
        sortField: "sales",
      }),
    ).toEqual([
      "listingkit",
      "sds",
      "products",
      {
        keyword: "shirt",
        page: 2,
        size: 24,
        shipmentArea: "US",
        categoryId: 123,
        sortField: "sales",
      },
    ]);
  });
});
