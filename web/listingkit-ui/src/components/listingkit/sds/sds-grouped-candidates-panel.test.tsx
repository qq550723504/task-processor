import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SDSGroupedCandidatesPanel } from "@/components/listingkit/sds/sds-grouped-candidates-panel";

describe("SDSGroupedCandidatesPanel", () => {
  it("renders grouped candidates and routes select/remove actions", () => {
    const onRemove = vi.fn();
    const onSelect = vi.fn();
    const item = {
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
      printableWidth: 1000,
      printableHeight: 1000,
      selectedVariantIds: [11],
    };

    render(
      <SDSGroupedCandidatesPanel
        activeSelection={item}
        baselineStatuses={{
          "1:21:11:layer-a:11": {
            reason: "",
            status: "ready",
          },
        }}
        items={[item]}
        onRemove={onRemove}
        onSelect={onSelect}
      />,
    );

    expect(screen.getByText("批量候选池")).toBeInTheDocument();
    expect(screen.getByText("1 款候选")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "当前已选" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "移除" }));
    expect(onRemove).toHaveBeenCalledWith(item);

    fireEvent.click(screen.getByRole("button", { name: "当前已选" }));
    expect(onSelect).toHaveBeenCalledWith(item, {
      reason: "",
      status: "ready",
    });
    expect(screen.getByText("Baseline 已就绪")).toBeInTheDocument();
  });

  it("shows loading and missing baseline states", () => {
    const item = {
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
      selectedVariantIds: [11],
    };

    const { rerender } = render(
      <SDSGroupedCandidatesPanel
        baselineStatuses={{}}
        items={[item]}
        onRemove={() => {}}
        onSelect={() => {}}
      />,
    );

    expect(screen.getByText("Baseline 检查中")).toBeInTheDocument();
    expect(
      screen.getByText("正在检查 baseline 状态..."),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "回选并等待" })).toBeInTheDocument();

    rerender(
      <SDSGroupedCandidatesPanel
        baselineStatuses={{
          "1:21:11:layer-a:11": {
            reason: "baseline missing",
            status: "missing",
          },
        }}
        items={[item]}
        onRemove={() => {}}
        onSelect={() => {}}
      />,
    );

    expect(screen.getByText("Baseline 缺失")).toBeInTheDocument();
    expect(screen.getByText("baseline missing")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "回选并预热" })).toBeInTheDocument();
  });

  it("shows failed baseline reasons inline", () => {
    const item = {
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
      selectedVariantIds: [11],
    };

    render(
      <SDSGroupedCandidatesPanel
        baselineStatuses={{
          "1:21:11:layer-a:11": {
            reason: "sync failed upstream",
            status: "failed",
          },
        }}
        items={[item]}
        onRemove={() => {}}
        onSelect={() => {}}
      />,
    );

    expect(screen.getByText("Baseline 异常")).toBeInTheDocument();
    expect(screen.getByText("sync failed upstream")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "回选并重试" })).toBeInTheDocument();
  });

  it("offers add-to-batch actions for ready candidates", () => {
    const onAddToBatch = vi.fn();
    const onCreateBatch = vi.fn();
    const item = {
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
      selectedVariantIds: [11],
    };

    render(
      <SDSGroupedCandidatesPanel
        activeBatchId="batch-1"
        activeBatchLabel="Retro Cherries"
        baselineStatuses={{
          "1:21:11:layer-a:11": {
            reason: "",
            status: "ready",
          },
        }}
        items={[item]}
        onAddToBatch={onAddToBatch}
        onCreateBatch={onCreateBatch}
        onRemove={() => {}}
        onSelect={() => {}}
        recentBatches={[
          { id: "batch-1", title: "Retro Cherries" },
          { id: "batch-2", title: "Second Batch" },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "加入当前批次 · Retro Cherries" }));
    expect(onAddToBatch).toHaveBeenCalledWith(item, "batch-1");

    fireEvent.click(screen.getByRole("button", { name: "加入其他批次" }));
    fireEvent.click(screen.getByRole("button", { name: "Second Batch" }));
    expect(onAddToBatch).toHaveBeenCalledWith(item, "batch-2");

    fireEvent.click(screen.getByRole("button", { name: "新建批次并加入" }));
    expect(onCreateBatch).toHaveBeenCalledWith(item);
  });

  it("surfaces a bulk warm action for missing and failed candidates", () => {
    const onWarmAll = vi.fn();
    const items = [
      {
        productId: 1,
        parentProductId: 1,
        variantId: 11,
        prototypeGroupId: 21,
        layerId: "layer-a",
        productName: "Product A",
        variantLabel: "M · black",
        selectedVariantIds: [11],
      },
      {
        productId: 2,
        parentProductId: 2,
        variantId: 22,
        prototypeGroupId: 32,
        layerId: "layer-b",
        productName: "Product B",
        variantLabel: "L · white",
        selectedVariantIds: [22],
      },
    ];

    render(
      <SDSGroupedCandidatesPanel
        baselineStatuses={{
          "1:21:11:layer-a:11": {
            reason: "missing baseline",
            status: "missing",
          },
          "2:32:22:layer-b:22": {
            reason: "sync failed upstream",
            status: "failed",
          },
        }}
        isWarmingAll={false}
        items={items}
        onRemove={() => {}}
        onSelect={() => {}}
        onWarmAll={onWarmAll}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "批量预热 2 款" }));
    expect(onWarmAll).toHaveBeenCalledWith(items);
  });

  it("highlights candidates that were just warmed successfully", () => {
    const item = {
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
      selectedVariantIds: [11],
    };

    render(
      <SDSGroupedCandidatesPanel
        baselineStatuses={{
          "1:21:11:layer-a:11": {
            reason: "",
            status: "ready",
          },
        }}
        items={[item]}
        onRemove={() => {}}
        onSelect={() => {}}
        recentlyWarmedSelectionIds={["1:21:11:layer-a:11"]}
      />,
    );

    expect(
      screen.getByText("baseline 刚预热完成，现在可以直接加入 grouped 批量上品。"),
    ).toBeInTheDocument();
  });

  it("shows a warm summary banner after bulk warmup", () => {
    const item = {
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
      selectedVariantIds: [11],
    };

    render(
      <SDSGroupedCandidatesPanel
        baselineStatuses={{
          "1:21:11:layer-a:11": {
            reason: "",
            status: "ready",
          },
        }}
        items={[item]}
        onRemove={() => {}}
        onSelect={() => {}}
        warmSummary={{ failedCount: 1, successCount: 2 }}
      />,
    );

    expect(
      screen.getByText("本次批量预热完成：成功 2 款，失败 1 款。失败项可以继续单独重试。"),
    ).toBeInTheDocument();
  });
});
