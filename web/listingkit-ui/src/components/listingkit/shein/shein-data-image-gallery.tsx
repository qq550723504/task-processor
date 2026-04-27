import { useState } from "react";

import { Card } from "@/components/shared/card";
import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";
import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";

export function SheinDataImageGallery({
  images,
  selectedUrl,
  onSelect,
  onRegenerate,
  isRegenerating = false,
  regenerationError,
}: {
  images: SheinPreviewImage[];
  selectedUrl?: string;
  onSelect: (image: SheinPreviewImage) => void;
  onRegenerate?: (image: SheinPreviewImage, prompt: string) => Promise<void> | void;
  isRegenerating?: boolean;
  regenerationError?: string | null;
}) {
  const [activeImage, setActiveImage] = useState<SheinPreviewImage | null>(null);
  const [regenerationPrompt, setRegenerationPrompt] = useState("");

  const canRegenerate =
    Boolean(onRegenerate && activeImage) &&
    regenerationPrompt.trim().length > 0 &&
    !isRegenerating;

  if (images.length === 0) {
    return null;
  }

  return (
    <Card className="border-zinc-200 bg-white p-5">
      <div className="space-y-4">
        <div className="flex flex-col gap-1 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
              SHEIN data images
            </p>
            <p className="mt-1 text-sm leading-6 text-zinc-600">
              SDS-rendered and SHEIN-ready images are shown here. Source images are
              shown only when no rendered payload is available.
            </p>
          </div>
          <span className="text-xs font-medium text-zinc-500">
            {images.length} image{images.length === 1 ? "" : "s"}
          </span>
        </div>

        <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
          {images.map((image) => {
            const active = image.url === selectedUrl;
            return (
              <button
                className={[
                  "group min-w-0 rounded-2xl border bg-white p-2 text-left transition",
                  active
                    ? "border-zinc-950 ring-2 ring-zinc-950/10"
                    : "border-zinc-200 hover:border-zinc-400",
                ].join(" ")}
                key={image.id}
                onClick={() => {
                  onSelect(image);
                  setActiveImage(image);
                  setRegenerationPrompt("");
                }}
                type="button"
              >
                <div className="aspect-square overflow-hidden rounded-xl bg-zinc-50">
                  {/* The URLs come from SHEIN/SDS payloads and may not be known to Next image config. */}
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    alt={image.label}
                    className="h-full w-full object-contain transition group-hover:scale-[1.02]"
                    src={toImageProxyUrl(image.url)}
                  />
                </div>
                <div className="mt-2 min-w-0">
                  <p className="truncate text-xs font-semibold text-zinc-900">
                    {image.label}
                  </p>
                  <p className="mt-1 truncate text-[11px] text-zinc-500">
                    {image.url}
                  </p>
                </div>
              </button>
            );
          })}
        </div>
      </div>

      {activeImage ? (
        <div
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/78 px-4 py-6 backdrop-blur-sm"
          role="dialog"
        >
          <div className="flex max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-[1.75rem] bg-white shadow-2xl">
            <div className="flex flex-wrap items-start justify-between gap-3 border-b border-zinc-200 px-5 py-4">
              <div className="min-w-0">
                <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  SHEIN image preview
                </p>
                <h3 className="mt-1 truncate text-lg font-semibold text-zinc-950">
                  {activeImage.label}
                </h3>
                <p className="mt-1 break-all text-xs text-zinc-500">
                  {activeImage.url}
                </p>
              </div>
              <div className="flex flex-wrap gap-2">
                <a
                  className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
                  href={activeImage.url}
                  rel="noreferrer"
                  target="_blank"
                >
                  Open original
                </a>
                <button
                  className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
                  onClick={() => setActiveImage(null)}
                  type="button"
                >
                  Close
                </button>
              </div>
            </div>
            <div className="grid min-h-0 flex-1 gap-4 overflow-auto bg-zinc-100 p-4 lg:grid-cols-[minmax(0,1fr)_360px]">
              <div className="min-h-0">
                {/* eslint-disable-next-line @next/next/no-img-element */}
                <img
                  alt={activeImage.label}
                  className="mx-auto max-h-[76vh] max-w-full rounded-2xl bg-white object-contain shadow-sm"
                  src={toImageProxyUrl(activeImage.url)}
                />
              </div>
              {onRegenerate ? (
                <form
                  className="space-y-3 rounded-2xl border border-zinc-200 bg-white p-4 shadow-sm"
                  onSubmit={async (event) => {
                    event.preventDefault();
                    if (!activeImage || !canRegenerate) return;
                    try {
                      await onRegenerate(activeImage, regenerationPrompt.trim());
                      setRegenerationPrompt("");
                      setActiveImage(null);
                    } catch {
                      // The parent surfaces the API message in this panel.
                    }
                  }}
                >
                  <div>
                    <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                      Regenerate this image
                    </p>
                    <p className="mt-1 text-xs leading-5 text-zinc-600">
                      Describe what is wrong. The current image will be sent back
                      as the problem reference.
                    </p>
                  </div>
                  <textarea
                    className="min-h-32 w-full resize-y rounded-2xl border border-zinc-200 bg-white px-3 py-3 text-sm text-zinc-900 outline-none transition placeholder:text-zinc-400 focus:border-zinc-950 focus:ring-2 focus:ring-zinc-950/10"
                    onChange={(event) => setRegenerationPrompt(event.target.value)}
                    placeholder="Example: keep the same artwork, but remove the extra text, make the product larger, and fix the cropped edge."
                    value={regenerationPrompt}
                  />
                  {regenerationError ? (
                    <p className="rounded-xl bg-red-50 px-3 py-2 text-xs leading-5 text-red-700">
                      {regenerationError}
                    </p>
                  ) : null}
                  <button
                    className="inline-flex h-10 w-full items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800 disabled:cursor-not-allowed disabled:bg-zinc-300"
                    disabled={!canRegenerate}
                    type="submit"
                  >
                    {isRegenerating ? "Regenerating..." : "Regenerate and replace"}
                  </button>
                </form>
              ) : null}
            </div>
          </div>
        </div>
      ) : null}
    </Card>
  );
}
