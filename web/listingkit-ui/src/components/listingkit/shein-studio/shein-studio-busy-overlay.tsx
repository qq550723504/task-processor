export function SheinStudioBusyOverlay({ message }: { message: string }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-[2rem] border border-white/20 bg-white px-6 py-6 text-center shadow-2xl">
        <div className="mx-auto h-12 w-12 animate-spin rounded-full border-4 border-zinc-200 border-t-zinc-950" />
        <h3 className="mt-5 text-lg font-semibold text-zinc-950">{message}</h3>
        <p className="mt-2 text-sm leading-6 text-zinc-600">
          图片生成耗时较长，通常需要 1-3 分钟。请不要刷新页面或重复点击，完成后界面会自动更新。
        </p>
        <div className="mt-4 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-900">
          当前已锁定操作，避免重复提交导致多次扣费或生成重复任务。
        </div>
      </div>
    </div>
  );
}
