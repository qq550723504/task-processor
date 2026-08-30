"use client";

import * as React from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { cn } from "@/lib/utils/cn";

type VirtualItemKey = string | number | bigint;

export type VirtualListProps<TItem> = {
  ariaLabel: string;
  children: (item: TItem, index: number) => React.ReactNode;
  className?: string;
  estimateSize: number;
  getItemKey?: (item: TItem, index: number) => VirtualItemKey;
  height: number;
  items: TItem[];
  overscan?: number;
};

export function VirtualList<TItem>({
  ariaLabel,
  children,
  className,
  estimateSize,
  getItemKey,
  height,
  items,
  overscan = 4,
}: VirtualListProps<TItem>) {
  "use no memo";

  const parentRef = React.useRef<HTMLDivElement>(null);
  // TanStack Virtual intentionally keeps interior mutable state; this component
  // is opted out of React Compiler memoization by the directive above.
  // eslint-disable-next-line react-hooks/incompatible-library
  const virtualizer = useVirtualizer({
    count: items.length,
    estimateSize: () => estimateSize,
    getItemKey: (index) => getItemKey?.(items[index], index) ?? index,
    getScrollElement: () => parentRef.current,
    initialRect: { height, width: 0 },
    overscan,
  });

  return (
    <div
      ref={parentRef}
      aria-label={ariaLabel}
      className={cn("overflow-auto", className)}
      role="list"
      style={{ height }}
    >
      <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((virtualItem) => (
          <div
            key={virtualItem.key}
            ref={virtualizer.measureElement}
            className="absolute left-0 top-0 w-full"
            data-index={virtualItem.index}
            role="listitem"
            style={{
              transform: `translateY(${virtualItem.start}px)`,
            }}
          >
            {children(items[virtualItem.index], virtualItem.index)}
          </div>
        ))}
      </div>
    </div>
  );
}
