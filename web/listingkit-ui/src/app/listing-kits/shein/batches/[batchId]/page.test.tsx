import { describe, expect, it, vi } from "vitest";

const redirect = vi.fn();

vi.mock("next/navigation", () => ({
  redirect: (...args: unknown[]) => redirect(...args),
}));

describe("/listing-kits/shein/batches/[batchId] page", () => {
  it("redirects legacy batch pages to the SDS workbench route", async () => {
    const page = await import("@/app/listing-kits/shein/batches/[batchId]/page");

    await page.default({
      params: Promise.resolve({ batchId: "batch-1" }),
    });

    expect(redirect).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
  });
});
