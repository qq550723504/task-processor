import { ReactNode } from "react";

import { Card } from "@/components/shared/card";

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: ReactNode;
}) {
  return (
    <Card className="p-6">
      <div className="space-y-2">
        <h3 className="text-base font-semibold text-zinc-950">{title}</h3>
        <p className="text-sm leading-6 text-zinc-600">{description}</p>
      </div>
      {action ? <div className="mt-4">{action}</div> : null}
    </Card>
  );
}
