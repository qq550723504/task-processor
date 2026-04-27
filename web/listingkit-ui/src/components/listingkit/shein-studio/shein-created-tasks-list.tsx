"use client";

import { useRouter } from "next/navigation";

import { Button } from "@/components/shared/button";
import type { SheinStudioCreatedTask } from "@/lib/types/shein-studio";

export function SheinCreatedTasksList({
  tasks,
}: {
  tasks: SheinStudioCreatedTask[];
}) {
  const router = useRouter();

  if (tasks.length === 0) {
    return null;
  }

  return (
    <div className="rounded-[1.25rem] border border-emerald-200 bg-emerald-50 px-4 py-4">
      <div className="text-sm font-semibold text-emerald-900">
        SHEIN data generated
      </div>
      <p className="mt-1 text-sm leading-6 text-emerald-800">
        Open a workspace to review generated SHEIN data, readiness checks, and draft
        submission actions.
      </p>
      <div className="mt-3 grid gap-3">
        {tasks.map((task) => (
          <div
            className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-emerald-200/80 bg-white/80 px-4 py-3"
            key={task.id}
          >
            <div className="space-y-1">
              <div className="text-sm font-semibold text-zinc-950">
                {task.title}
              </div>
              <div className="text-xs text-zinc-500">{task.id}</div>
            </div>
            <div className="flex gap-2">
              <Button
                onClick={() => router.push(`/listing-kits/${task.id}/status`)}
                tone="secondary"
              >
                Open status
              </Button>
              <Button
                onClick={() =>
                  router.push(
                    `/listing-kits/${task.id}/workspace?platform=shein&section_key=general_review`,
                  )
                }
                tone="ghost"
              >
                Review SHEIN data
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
