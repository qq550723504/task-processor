import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import SdsBatchPage from "@/app/listing-kits/sds/batches/[batchId]/page";

vi.mock(
  "@/components/listingkit/shein-studio/shein-studio-batch-page-shell",
  () => ({
    SheinStudioBatchPageShell: ({ batchId }: { batchId: string }) => (
      <div>batch shell: {batchId}</div>
    ),
  }),
);

describe("/listing-kits/sds/batches/[batchId] page", () => {
  it("renders the dedicated batch editor route", () => {
    render(<SdsBatchPage params={{ batchId: "batch-1" }} />);

    expect(screen.getByText("batch shell: batch-1")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "最近批次" }),
    ).not.toBeInTheDocument();
  });
});
