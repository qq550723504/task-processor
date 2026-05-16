import { SheinCreatedTasksList } from "@/components/listingkit/shein-studio/shein-created-tasks-list";
import { Badge } from "@/components/ui/badge";
import type { SheinStudioCreatedTask } from "@/lib/types/shein-studio";

export function SheinStudioTasksStep({
  createdTasks,
}: {
  createdTasks: SheinStudioCreatedTask[];
}) {
  return (
    <div
      id="shein-created-tasks"
      className="scroll-mt-6 rounded-[1.75rem] border border-zinc-200/80 bg-white p-5 shadow-sm"
    >
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
            第 4 步 · SHEIN 任务
          </p>
          <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
            审核已生成的工作区
          </h2>
          <p className="mt-1 max-w-2xl text-sm leading-6 text-zinc-600">
            打开每个任务的工作区，完成最终图片、价格、属性和提交确认。
          </p>
        </div>
        <Badge className="rounded-full px-3 py-1 text-xs" variant="neutral">
          {createdTasks.length} 个任务
        </Badge>
      </div>
      {createdTasks.length ? (
        <SheinCreatedTasksList tasks={createdTasks} />
      ) : (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm leading-6 text-amber-900">
          还没有创建 SHEIN 任务。先回到“审核款式”步骤批准款式，再在“生成图片”
          步骤点击“生成 SHEIN 资料”。
        </div>
      )}
    </div>
  );
}
