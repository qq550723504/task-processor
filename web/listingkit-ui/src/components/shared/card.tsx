import { HTMLAttributes } from "react";

import { cn } from "@/lib/utils/cn";

export function Card({
  className,
  ...props
}: HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-zinc-200 bg-white shadow-sm shadow-zinc-950/5",
        className,
      )}
      {...props}
    />
  );
}
