"use client";

import * as React from "react";
import type { EChartsOption } from "echarts";

import { cn } from "@/lib/utils/cn";

export type EChartProps = {
  ariaLabel: string;
  className?: string;
  option: EChartsOption;
};

export function EChart({ ariaLabel, className, option }: EChartProps) {
  const containerRef = React.useRef<HTMLDivElement>(null);
  const chartRef = React.useRef<import("echarts").ECharts | null>(null);
  const optionRef = React.useRef(option);

  React.useEffect(() => {
    optionRef.current = option;
    chartRef.current?.setOption(option, { notMerge: true });
  }, [option]);

  React.useEffect(() => {
    let active = true;
    let observer: ResizeObserver | undefined;

    void import("echarts").then((echarts) => {
      if (!active || !containerRef.current) {
        return;
      }

      const chart = echarts.init(containerRef.current, undefined, { renderer: "svg" });
      chartRef.current = chart;
      chart.setOption(optionRef.current, { notMerge: true });
      observer = new ResizeObserver(() => chart.resize());
      observer.observe(containerRef.current);
    });

    return () => {
      active = false;
      observer?.disconnect();
      chartRef.current?.dispose();
      chartRef.current = null;
    };
  }, []);

  return (
    <div
      ref={containerRef}
      aria-label={ariaLabel}
      className={cn("min-h-72 w-full", className)}
      role="img"
    />
  );
}
