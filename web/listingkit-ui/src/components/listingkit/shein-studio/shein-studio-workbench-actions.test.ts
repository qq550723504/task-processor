import { describe, expect, it, vi } from "vitest";

import { clearWorkbenchTaskRecoveryAlerts } from "@/components/listingkit/shein-studio/shein-studio-workbench-actions";
import type { SheinStudioWorkbenchController } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";

describe("shein studio workbench actions", () => {
  it("clears task recovery alerts before retrying failed itemized work", () => {
    const setField = vi.fn();

    clearWorkbenchTaskRecoveryAlerts({
      setField: setField as SheinStudioWorkbenchController["setField"],
    });

    expect(setField.mock.calls).toEqual([
      ["generationError", ""],
      ["generationWarning", ""],
      ["creatingError", ""],
      ["creatingWarning", ""],
    ]);
  });
});
