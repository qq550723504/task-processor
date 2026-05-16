import * as React from "react";

import { cn } from "@/lib/utils/cn";

type SeparatorProps = React.ComponentProps<"div"> & {
  orientation?: "horizontal" | "vertical";
};

function Separator({
  className,
  orientation = "horizontal",
  role = "separator",
  ...props
}: SeparatorProps) {
  return (
    <div
      aria-orientation={orientation}
      className={cn(
        "shrink-0 bg-border",
        orientation === "horizontal" ? "h-px w-full" : "h-full w-px",
        className,
      )}
      role={role}
      {...props}
    />
  );
}

export { Separator };
