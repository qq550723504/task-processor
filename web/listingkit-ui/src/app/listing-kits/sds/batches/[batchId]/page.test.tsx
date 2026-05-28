import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import SdsBatchPage from "@/app/listing-kits/sds/batches/[batchId]/page";

vi.mock(
  "@/components/listingkit/shein-studio/shein-studio-batch-page-shell",
  () => ({
    SheinStudioBatchPageShell: ({ batchId }: { batchId: string }) => (
      <div>
        <a href="/listing-kits/sds">返回最近批次首页</a>
        <a href={`/listing-kits/sds/new?targetBatchId=${batchId}`}>去 SDS 选品并加入当前批次</a>
        <h1>批次工作台</h1>
        <div>batch shell: {batchId}</div>
      </div>
    ),
  }),
);

describe("/listing-kits/sds/batches/[batchId] page", () => {
  it("renders the dedicated batch editor route", async () => {
    render(
      await SdsBatchPage({
        params: Promise.resolve({ batchId: "batch-1" }),
      }),
    );

    expect(
      screen.getByRole("link", { name: "返回最近批次首页" }),
    ).toHaveAttribute("href", "/listing-kits/sds");
    expect(
      screen.getByRole("link", { name: "去 SDS 选品并加入当前批次" }),
    ).toHaveAttribute("href", "/listing-kits/sds/new?targetBatchId=batch-1");
    expect(screen.getByRole("heading", { name: "批次工作台" })).toBeInTheDocument();
    expect(screen.getByText("batch shell: batch-1")).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "最近批次" }),
    ).not.toBeInTheDocument();
  });
});
