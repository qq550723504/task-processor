import { describe, expect, it } from "vitest";

import { selectListingKitMockPayload } from "@/app/api/listing-kits/proxy-mock";
import type { ListingKitMockBundle } from "@/app/api/listing-kits/mock-types";

const bundle = {
  action: { kind: "action" },
  createTask: { kind: "create" },
  dispatch: { kind: "dispatch" },
  preview: { kind: "preview" },
  queue: { kind: "queue" },
  reviewPreview: { kind: "reviewPreview" },
  reviewSession: { kind: "reviewSession" },
  taskResult: { kind: "taskResult" },
} as unknown as ListingKitMockBundle;

describe("selectListingKitMockPayload", () => {
  it("selects action payloads for execute posts", () => {
    expect(
      selectListingKitMockPayload({
        bundle,
        method: "POST",
        path: ["tasks", "demo", "generation-actions", "execute"],
      }),
    ).toBe(bundle.action);
  });

  it("selects dispatch payloads for other posts", () => {
    expect(
      selectListingKitMockPayload({
        bundle,
        method: "POST",
        path: ["tasks", "demo", "generation-navigation", "dispatch"],
      }),
    ).toBe(bundle.dispatch);
  });

  it("selects task result payloads for task detail routes", () => {
    expect(
      selectListingKitMockPayload({
        bundle,
        method: "GET",
        path: ["tasks", "demo"],
      }),
    ).toBe(bundle.taskResult);
  });
});
