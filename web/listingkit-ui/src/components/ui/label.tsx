import * as React from "react";

import { cn } from "@/lib/utils/cn";

function Label({ className, ...props }: React.ComponentProps<"label">) {
  return (
    <label
      className={cn(
        "text-sm font-medium text-foreground",
        className,
      )}
      {...props}
    />
  );
}

export { Label };
