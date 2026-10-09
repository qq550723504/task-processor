import { expect, it } from "vitest";
import {
  observationListSchema,
  observationRecordSchema,
  observationTracksSchema,
} from "./store-observations";
export const fixtureID = "d6f6ca0a-27e2-4c4a-b1aa-4505110ae635";
export const fixtureRecord = {
  storeId: fixtureID,
  syncId: fixtureID,
  id: "spu-a",
  observedAt: "2026-10-09T01:00:00Z",
  product: {
    id: "spu-a",
    skcs: [
      {
        id: "skc-a",
        sellerCode: "",
        title: "Shoe",
        imageUrl: "",
        site: "shein-us",
        siteStatus: null,
        skus: [
          {
            id: "sku-a",
            sellerSku: "",
            prices: [{ currency: "USD", value: "0.00", special: "" }],
            costs: [],
            inventory: [],
          },
        ],
      },
    ],
  },
};
export const fixtureOrderRecord = {
  storeId: fixtureID,
  syncId: fixtureID,
  id: "order-a",
  observedAt: "2026-10-09T01:00:00Z",
  order: {
    id: "order-a",
    site: "shein-us",
    status: 2,
    stockMode: null,
    type: null,
    tag: 1,
    reasons: [4],
    items: [],
    packages: [{ id: "", waybill: "", carrier: "Carrier", label: "label-a" }],
    amount: null,
    supplyCost: null,
    createdAt: "",
    updatedAt: "",
    issuedAt: "",
    needDeliveryAt: "",
    handoverAt: "",
    expectedCollectAt: "",
  },
};
it("retains optional package metadata without inventing a package number", () => {
  expect(
    observationRecordSchema.parse(fixtureOrderRecord).order?.packages,
  ).toEqual(fixtureOrderRecord.order.packages);
});
it("retains explicit stale saved observations and their original time", () => {
  const value = observationRecordSchema.parse({
    ...fixtureOrderRecord,
    stale: true,
  });
  expect(value.observedAt).toBe(fixtureOrderRecord.observedAt);
  expect(value.order?.packages).toEqual(fixtureOrderRecord.order.packages);
  expect(value).toHaveProperty("stale", true);
});
it("preserves absent inventory and zero price without fabricating quantities", () => {
  const value = observationRecordSchema.parse(fixtureRecord);
  expect(value.product?.skcs[0].skus[0].inventory).toEqual([]);
  expect(value.product?.skcs[0].skus[0].prices[0].value).toBe("0.00");
});
it("rejects sensitive fields, duplicate ownership projections and mutable platform instructions", () => {
  expect(
    observationRecordSchema.safeParse({ ...fixtureRecord, address: "private" })
      .success,
  ).toBe(false);
  expect(
    observationRecordSchema.safeParse({
      ...fixtureRecord,
      order: { id: "other" },
    }).success,
  ).toBe(false);
  expect(
    observationTracksSchema.safeParse([
      { carrier: "", waybill: "", nodes: [], raw: { address: "private" } },
    ]).success,
  ).toBe(false);
});
it("requires whole-scope summaries and explicit coverage instead of page-derived totals", () => {
  expect(
    observationListSchema.safeParse({
      items: [fixtureRecord],
      next: "",
      summary: {
        total: 100,
        active: 0,
        offShelf: 0,
        today: 0,
        todayUnknown: 0,
        pending: 0,
        transit: 0,
        exceptional: 0,
        unknown: 100,
      },
      syncs: [],
      latest: [],
      complete: false,
    }).success,
  ).toBe(true);
  expect(
    observationListSchema.safeParse({
      items: [fixtureRecord],
      next: "",
      total: 1,
    }).success,
  ).toBe(false);
});
