import { describe, expect, it } from "vitest";

import {
  buildTaskCreateDefaultValues,
  schema,
} from "@/components/listingkit/tasks/task-create-form-model";

function valuesFor(platforms: string[], sheinStoreId: string) {
  return {
    ...buildTaskCreateDefaultValues({ variant: "default" }),
    text: "Canvas tote",
    platforms,
    sheinStoreId,
  };
}

describe("task create form schema", () => {
  it.each(["", "0", "-1", "1.5", "870x"])(
    "rejects SHEIN store id %j",
    (sheinStoreId) => {
      const result = schema.safeParse(valuesFor(["shein"], sheinStoreId));

      expect(result.success).toBe(false);
      if (!result.success) {
        expect(result.error.flatten().fieldErrors.sheinStoreId).toContain(
          "选择 SHEIN 平台时，必须选择有效的 SHEIN 店铺。",
        );
      }
    },
  );

  it("accepts a positive integer SHEIN store id", () => {
    expect(schema.safeParse(valuesFor(["shein"], "870")).success).toBe(true);
  });

  it("does not require a SHEIN store for non-SHEIN platforms", () => {
    expect(schema.safeParse(valuesFor(["amazon"], "")).success).toBe(true);
  });
});
