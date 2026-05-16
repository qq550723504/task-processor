import * as React from "react";
import { cva, type VariantProps } from "class-variance-authority";

import { cn } from "@/lib/utils/cn";

const alertVariants = cva(
  "relative grid w-full gap-1 rounded-lg border px-4 py-3 text-sm text-foreground",
  {
    variants: {
      variant: {
        default: "border-border bg-background",
        destructive:
          "border-destructive/50 text-destructive dark:border-destructive [&>svg]:text-destructive",
        success: "border-success/50 bg-success/10 text-success-foreground",
        warning: "border-warning/50 bg-warning/10 text-warning-foreground",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
);

type AlertProps = React.ComponentProps<"div"> &
  VariantProps<typeof alertVariants>;

function Alert({ className, variant, role = "status", ...props }: AlertProps) {
  return (
    <div
      className={cn(alertVariants({ variant }), className)}
      role={role}
      {...props}
    />
  );
}

function AlertTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn("font-medium leading-none tracking-tight", className)}
      {...props}
    />
  );
}

function AlertDescription({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      className={cn("text-sm leading-6", className)}
      {...props}
    />
  );
}

function AlertAction({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      className={cn("mt-2 flex justify-end sm:absolute sm:right-3 sm:top-3 sm:mt-0", className)}
      {...props}
    />
  );
}

export { Alert, AlertAction, AlertDescription, AlertTitle };
