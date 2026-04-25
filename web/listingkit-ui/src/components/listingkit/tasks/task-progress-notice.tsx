import { Clock3, LoaderCircle } from "lucide-react";

import { Card } from "@/components/shared/card";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

function noticeCopy(status?: string) {
  if (status === "processing") {
    return {
      title: "Generation is still running",
      description:
        "Preview, queue, and review actions will fill in as child tasks finish. Status refreshes automatically every 5 seconds.",
      icon: LoaderCircle,
      iconClassName: "animate-spin text-sky-600",
    };
  }

  if (status === "pending") {
    return {
      title: "Waiting to start",
      description:
        "The task has been accepted, but generation planning has not started yet. Status refreshes automatically every 5 seconds.",
      icon: Clock3,
      iconClassName: "text-zinc-500",
    };
  }

  return undefined;
}

export function TaskProgressNotice({
  task,
}: {
  task?: ListingKitTaskResult | null;
}) {
  const copy = noticeCopy(task?.status);
  if (!copy) {
    return null;
  }

  const Icon = copy.icon;

  return (
    <Card className="border-sky-200 bg-sky-50/60 p-4">
      <div className="flex items-start gap-3">
        <Icon className={`mt-0.5 h-5 w-5 ${copy.iconClassName}`} />
        <div className="space-y-1">
          <p className="text-sm font-semibold text-zinc-950">{copy.title}</p>
          <p className="text-sm leading-6 text-zinc-600">{copy.description}</p>
        </div>
      </div>
    </Card>
  );
}
