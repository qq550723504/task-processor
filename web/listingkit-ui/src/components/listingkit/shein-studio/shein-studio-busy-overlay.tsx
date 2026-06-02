export function SheinStudioBusyOverlay({ message }: { message: string }) {
  return (
    <div className="sticky top-4 z-20 rounded-[1.75rem] border border-zinc-200/80 bg-white/95 px-5 py-4 shadow-lg backdrop-blur">
      <div className="flex flex-col items-start gap-4 sm:flex-row sm:flex-wrap">
        <div className="mt-0.5 h-10 w-10 animate-spin rounded-full border-4 border-zinc-200 border-t-zinc-950" />
        <div className="min-w-0 flex-1 space-y-2">
          <h3 className="text-base font-semibold text-zinc-950">{message}</h3>
          <p className="text-sm leading-6 text-zinc-600">
            图片生成通常需要 1-3 分钟，请勿重复点击生成。
          </p>
        </div>
        <div className="w-full rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-900 sm:max-w-xs">
          当前仅锁定本次生图相关字段和提交按钮，避免重复扣费或让结果和表单状态错位。
        </div>
      </div>
    </div>
  );
}
