import * as React from "react";

import { cn } from "@/lib/utils/cn";

function Checkbox({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type="checkbox"
      className={cn(
        "size-4 shrink-0 rounded border border-input accent-primary outline-none transition-shadow focus-visible:ring-ring/50 focus-visible:ring-[3px] disabled:cursor-not-allowed disabled:opacity-50",
        className,
      )}
      {...props}
    />
  );
}

export { Checkbox };
