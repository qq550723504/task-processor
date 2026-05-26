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
    expect(onSelect).toHaveBeenCalledWith(item);
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
  });
});
