import { render, screen, waitFor } from "@testing-library/react";
import type { EChartsOption } from "echarts";
import { afterEach, vi } from "vitest";

const chart = vi.hoisted(() => ({
  dispose: vi.fn(),
  resize: vi.fn(),
  setOption: vi.fn(),
}));

vi.mock("echarts", () => ({
  init: vi.fn(() => chart),
}));

import { EChart } from "@/components/ui/chart";

afterEach(() => {
  vi.clearAllMocks();
  vi.unstubAllGlobals();
});

describe("EChart", () => {
  it("updates the chart and disposes the instance on unmount", async () => {
    let notifyResize: (() => void) | undefined;
    vi.stubGlobal(
      "ResizeObserver",
      class {
        constructor(callback: () => void) {
          notifyResize = callback;
        }

        observe() {}
        disconnect() {}
      },
    );

    const first: EChartsOption = {
      xAxis: { type: "category" },
      series: [{ type: "bar", data: [1] }],
    };
    const second: EChartsOption = {
      xAxis: { type: "category" },
      series: [{ type: "bar", data: [2] }],
    };
    const view = render(<EChart ariaLabel="经营趋势" option={first} />);

    expect(screen.getByRole("img", { name: "经营趋势" })).toBeInTheDocument();
    await waitFor(() =>
      expect(chart.setOption).toHaveBeenCalledWith(first, { notMerge: true }),
    );
    notifyResize?.();
    expect(chart.resize).toHaveBeenCalledTimes(1);

    view.rerender(<EChart ariaLabel="经营趋势" option={second} />);
    await waitFor(() =>
      expect(chart.setOption).toHaveBeenLastCalledWith(second, { notMerge: true }),
    );

    view.unmount();
    expect(chart.dispose).toHaveBeenCalledTimes(1);
  });
});
