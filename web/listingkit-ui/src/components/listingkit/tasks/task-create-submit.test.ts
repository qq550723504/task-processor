import { describe, expect, it } from "vitest";

import { buildTaskCreateDefaultValues } from "@/components/listingkit/tasks/task-create-form-model";
import { buildTaskCreateSubmission } from "@/components/listingkit/tasks/task-create-submit";

function valuesFor(platforms: string[], sheinStoreId: string) {
  return {
    ...buildTaskCreateDefaultValues({ variant: "default" }),
    text: "Canvas tote",
    platforms,
    sheinStoreId,
  };
}

describe("buildTaskCreateSubmission", () => {
  it("fails closed when a SHEIN request omits its store", async () => {
    await expect(
      buildTaskCreateSubmission({
        values: valuesFor(["shein"], ""),
      }),
    ).resolves.toEqual({
      ok: false,
      message: "选择 SHEIN 平台时，必须选择有效的 SHEIN 店铺。",
    });
  });

  it("includes a positive SHEIN store id", async () => {
    const result = await buildTaskCreateSubmission({
      values: valuesFor(["shein"], "870"),
    });

    expect(result).toMatchObject({
      ok: true,
      request: {
        platforms: ["shein"],
        shein_store_id: 870,
      },
    });
  });

  it("leaves non-SHEIN requests unaffected", async () => {
    const result = await buildTaskCreateSubmission({
      values: valuesFor(["amazon"], ""),
    });

    expect(result).toMatchObject({
      ok: true,
      request: {
        platforms: ["amazon"],
      },
    });
    if (result.ok) {
      expect(result.request).not.toHaveProperty("shein_store_id");
    }
  });
});
