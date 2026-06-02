import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

export function ImagePreviewDialog({
  activeImage,
  activeImageCanRegenerate,
  canRegenerate,
  isRegenerating,
  onClose,
  onRegenerate,
  regenerationError,
  regenerationPrompt,
  setRegenerationPrompt,
}: {
  activeImage?: SheinPreviewImage | null;
  activeImageCanRegenerate: boolean;
  canRegenerate: boolean;
  isRegenerating?: boolean;
  onClose: () => void;
  onRegenerate?: (image: SheinPreviewImage, prompt: string) => Promise<void> | void;
  regenerationError?: string | null;
  regenerationPrompt: string;
  setRegenerationPrompt: (value: string) => void;
}) {
  if (!activeImage) {
    return null;
  }

  return (
    <div
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/78 px-4 py-6 backdrop-blur-sm"
      role="dialog"
    >
      <div className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[1.75rem] bg-white shadow-2xl">
        <div className="flex flex-col gap-3 border-b border-zinc-200 px-5 py-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              SHEIN 图片预览
            </p>
            <h3 className="mt-1 truncate text-lg font-semibold text-zinc-950">
              {activeImage.label}
            </h3>
            <p className="mt-1 break-all text-xs text-zinc-500">
              {activeImage.url}
            </p>
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
            <Button asChild className="w-full sm:w-auto" variant="outline">
              <a href={activeImage.url} rel="noreferrer" target="_blank">
                打开原图
              </a>
            </Button>
            <Button
              className="w-full sm:w-auto"
              onClick={onClose}
              type="button"
            >
              关闭
            </Button>
          </div>
        </div>
        <div className="grid min-h-0 flex-1 gap-4 overflow-auto bg-zinc-100 p-4 xl:grid-cols-[minmax(0,1fr)_360px]">
          <div className="min-h-0">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              alt={activeImage.label}
              className="mx-auto max-h-[76vh] max-w-full rounded-2xl bg-white object-contain shadow-sm"
              src={toImageProxyUrl(activeImage.url)}
            />
          </div>
          {onRegenerate && activeImageCanRegenerate ? (
            <form
              className="space-y-3 rounded-2xl border border-zinc-200 bg-white p-4 shadow-sm"
              onSubmit={async (event) => {
                event.preventDefault();
                if (!canRegenerate) return;
                try {
                  await onRegenerate(activeImage, regenerationPrompt.trim());
                  setRegenerationPrompt("");
                  onClose();
                } catch {
                  // The parent surfaces the API message in this panel.
                }
              }}
            >
              <div>
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  重新生成这张图
                </p>
                <p className="mt-1 text-xs leading-5 text-zinc-600">
                  描述这张图哪里不合适。系统会把当前图片作为问题参考一起发给模型。
                </p>
              </div>
              <Textarea
                className="min-h-32 resize-y rounded-2xl px-3 py-3"
                onChange={(event) => setRegenerationPrompt(event.target.value)}
                placeholder="例如：保持图案不变，去掉多余文字，商品放大一点，修复边缘裁切。"
                value={regenerationPrompt}
              />
              {regenerationError ? (
                <p className="rounded-xl bg-red-50 px-3 py-2 text-xs leading-5 text-red-700">
                  {regenerationError}
                </p>
              ) : null}
              <Button
                className="w-full"
                disabled={!canRegenerate}
                type="submit"
              >
                {isRegenerating ? "重新生成中..." : "重新生成并替换"}
              </Button>
            </form>
          ) : null}
        </div>
      </div>
    </div>
  );
}
