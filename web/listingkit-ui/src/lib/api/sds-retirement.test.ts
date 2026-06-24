import { describe, expect, it, vi } from "vitest";

import {
  confirmSDSRetirementRun,
  createSDSRetirementRun,
  getSDSRetirementRun,
  retrySDSRetirementRun,
  updateSDSRetirementSelection,
} from "@/lib/api/sds-retirement";

const apiRequest = vi.fn();

vi.mock("@/lib/api/client", () => ({
  apiRequest: (...args: unknown[]) => apiRequest(...args),
}));

describe("sds retirement api helpers", () => {
  it("sends create, update, and confirm requests to the expected endpoints", async () => {
    apiRequest.mockResolvedValue({ ok: true });

    await createSDSRetirementRun({
      platform: "shein",
      store_id: 869,
      parent_product_id: 101,
      prototype_group_id: 301,
      variant_id: 201,
      selected_variant_ids: [201],
    });
    await updateSDSRetirementSelection("run-1", [
      {
        item_id: "item-1",
        selected: true,
        site_selection: '[{"site_abbr":"US","store_type":1}]',
      },
    ]);
    await confirmSDSRetirementRun("run-1");
    await getSDSRetirementRun("run-1");
    await retrySDSRetirementRun("run-1");

    expect(apiRequest.mock.calls).toEqual([
      [
        "/sds/retirements",
        {
          method: "POST",
          body: {
            platform: "shein",
            store_id: 869,
            parent_product_id: 101,
            prototype_group_id: 301,
            variant_id: 201,
            selected_variant_ids: [201],
          },
        },
      ],
      [
        "/sds/retirements/run-1/items",
        {
          method: "PATCH",
          body: {
            items: [
              {
                item_id: "item-1",
                selected: true,
                site_selection: '[{"site_abbr":"US","store_type":1}]',
              },
            ],
          },
        },
      ],
      [
        "/sds/retirements/run-1/confirm",
        {
          method: "POST",
          body: {},
        },
      ],
      [
        "/sds/retirements/run-1",
        {
          method: "GET",
        },
      ],
      [
        "/sds/retirements/run-1/retry",
        {
          method: "POST",
          body: {},
        },
      ],
    ]);
  });
});
