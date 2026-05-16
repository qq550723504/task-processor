"use client";

import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
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
        SHEIN 资料任务已创建
      </div>
      <p className="mt-1 text-sm leading-6 text-emerald-800">
        打开工作区确认 SHEIN 资料、价格和提交状态；如果需要处理生成/审核队列，可进入队列页。
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
                variant="secondary"
              >
                状态
              </Button>
              <Button
                onClick={() => router.push(`/listing-kits/${task.id}/queue?platform=shein`)}
                variant="secondary"
              >
                队列
              </Button>
              <Button
                onClick={() =>
                  router.push(
                    `/listing-kits/${task.id}/workspace?platform=shein&section_key=general_review`,
                  )
                }
                variant="ghost"
              >
                审核 SHEIN 资料
              </Button>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
