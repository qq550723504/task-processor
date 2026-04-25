"use client";

import { MouseEvent, useEffect, useMemo, useState } from "react";
import Image from "next/image";

import { Button } from "@/components/shared/button";
import { resolveGeneratedDesignSrc } from "@/lib/shein-studio/design-image";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

function resolveMockupSurfaces(selection?: SDSProductVariantSelection) {
  const items = [
    ...(selection?.mockupImageUrls ?? []),
    selection?.mockupImageUrl,
    selection?.templateImageUrl,
  ].filter((item): item is string => Boolean(item));

  return [...new Set(items)];
}

export function SheinDesignLightbox({
  activeView,
  design,
  onClose,
  onViewChange,
  selection,
}: {
  activeView: "design" | "mockup";
  design?: SheinStudioGeneratedDesign | null;
  onClose: () => void;
  onViewChange: (view: "design" | "mockup") => void;
  selection?: SDSProductVariantSelection;
}) {
  const surfaces = useMemo(() => resolveMockupSurfaces(selection), [selection]);
  const [activeSurfaceIndex, setActiveSurfaceIndex] = useState(0);
  const [zoomMode, setZoomMode] = useState<"fit" | "detail">("fit");

  const printableWidth = selection?.printableWidth ?? 1;
  const printableHeight = selection?.printableHeight ?? 1;
  const printableRatio = printableWidth / printableHeight;
  const activeSurfaceUrl = surfaces[activeSurfaceIndex];
  const hasMultipleSurfaces = surfaces.length > 1;

  useEffect(() => {
    function handleKeyDown(event: globalThis.KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
        return;
      }

      if (activeView !== "mockup" || !hasMultipleSurfaces) {
        return;
      }

      if (event.key === "ArrowLeft") {
        setActiveSurfaceIndex((current) =>
          current === 0 ? surfaces.length - 1 : current - 1,
        );
      }

      if (event.key === "ArrowRight") {
        setActiveSurfaceIndex((current) =>
          current === surfaces.length - 1 ? 0 : current + 1,
        );
      }

      if (event.key.toLowerCase() === "z") {
        setZoomMode((current) => (current === "fit" ? "detail" : "fit"));
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [activeView, hasMultipleSurfaces, onClose, surfaces.length]);

  function handleBackdropClick(event: MouseEvent<HTMLDivElement>) {
    if (event.target === event.currentTarget) {
      onClose();
    }
  }

  if (!design) {
    return null;
  }

  const designSrc = resolveGeneratedDesignSrc(design);

  function showPreviousSurface() {
    setActiveSurfaceIndex((current) =>
      current === 0 ? surfaces.length - 1 : current - 1,
    );
  }

  function showNextSurface() {
    setActiveSurfaceIndex((current) =>
      current === surfaces.length - 1 ? 0 : current + 1,
    );
  }

  const imageFitClass = zoomMode === "fit" ? "object-contain" : "object-cover";
  const previewInsetClass =
    zoomMode === "fit" ? "absolute inset-[14%_24%]" : "absolute inset-[8%_16%]";
  const overlayStyle =
    zoomMode === "fit"
      ? {
          width:
            printableRatio >= 1 ? "100%" : `${Math.max(56, printableRatio * 100)}%`,
          height:
            printableRatio >= 1
              ? `${Math.max(56, (1 / printableRatio) * 100)}%`
              : "100%",
        }
      : undefined;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/78 px-4 py-6 backdrop-blur-sm"
      onClick={handleBackdropClick}
      role="presentation"
    >
      <div className="relative w-full max-w-6xl rounded-[2rem] border border-white/10 bg-zinc-950 p-4 text-white shadow-[0_30px_80px_rgba(0,0,0,0.35)]">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-400">
              Image preview
            </p>
            <h3 className="mt-1 font-serif text-2xl tracking-[-0.03em]">
              Inspect generated art before approval.
            </h3>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              onClick={() => onViewChange("design")}
              tone={activeView === "design" ? "primary" : "secondary"}
            >
              Design
            </Button>
            <Button
              onClick={() => onViewChange("mockup")}
              tone={activeView === "mockup" ? "primary" : "secondary"}
            >
              Mockups
            </Button>
            {activeView === "mockup" ? (
              <Button
                onClick={() =>
                  setZoomMode((current) => (current === "fit" ? "detail" : "fit"))
                }
                tone="secondary"
              >
                {zoomMode === "fit" ? "Detail zoom" : "Fit view"}
              </Button>
            ) : null}
            <Button onClick={onClose} tone="ghost">
              Close
            </Button>
          </div>
        </div>

        <div className="overflow-hidden rounded-[1.5rem] border border-white/10 bg-zinc-900">
          {activeView === "design" ? (
            <div className="relative aspect-square min-h-[60vh]">
              <Image
                alt="Generated design preview"
                className="object-contain"
                fill
                src={designSrc}
                unoptimized
              />
            </div>
          ) : (
            <div className="space-y-4 p-4">
              <div className="relative aspect-[4/3] min-h-[56vh] rounded-[1.25rem] bg-[radial-gradient(circle_at_top,_rgba(161,161,170,0.14),_transparent_58%),linear-gradient(180deg,_#18181b_0%,_#0f0f13_100%)]">
                {activeSurfaceUrl ? (
                  <Image
                    alt="Template mockup"
                    className={imageFitClass}
                    fill
                    src={activeSurfaceUrl}
                    unoptimized
                  />
                ) : null}
                <div className={`${previewInsetClass} flex items-center justify-center`}>
                  <div
                    className="relative overflow-hidden rounded-[1rem] border border-white/10 bg-white/95 shadow-[0_18px_50px_rgba(15,23,42,0.4)]"
                    style={overlayStyle}
                  >
                    <Image
                      alt="Mockup design preview"
                      className={imageFitClass}
                      fill
                      src={designSrc}
                      unoptimized
                    />
                  </div>
                </div>
                {hasMultipleSurfaces ? (
                  <>
                    <button
                      aria-label="Previous mockup view"
                      className="absolute left-4 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full border border-white/12 bg-zinc-950/70 text-lg font-semibold text-white shadow-[0_10px_25px_rgba(0,0,0,0.25)] transition hover:bg-zinc-900"
                      onClick={showPreviousSurface}
                      type="button"
                    >
                      ‹
                    </button>
                    <button
                      aria-label="Next mockup view"
                      className="absolute right-4 top-1/2 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full border border-white/12 bg-zinc-950/70 text-lg font-semibold text-white shadow-[0_10px_25px_rgba(0,0,0,0.25)] transition hover:bg-zinc-900"
                      onClick={showNextSurface}
                      type="button"
                    >
                      ›
                    </button>
                  </>
                ) : null}
                {hasMultipleSurfaces ? (
                  <div className="absolute bottom-4 right-4 rounded-full border border-white/12 bg-zinc-950/75 px-3 py-1 text-xs font-medium text-zinc-200">
                    {activeSurfaceIndex + 1} / {surfaces.length}
                  </div>
                ) : null}
              </div>

              {hasMultipleSurfaces ? (
                <div className="space-y-2">
                  <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-400">
                    Mockup views
                  </div>
                  <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-4">
                    {surfaces.map((imageUrl, index) => (
                      <button
                        className={`relative overflow-hidden rounded-[1rem] border transition ${
                          index === activeSurfaceIndex
                            ? "border-emerald-400 shadow-[0_10px_25px_rgba(16,185,129,0.18)]"
                            : "border-white/10 hover:border-white/30"
                        }`}
                        key={`${imageUrl}:${index}`}
                        onClick={() => setActiveSurfaceIndex(index)}
                        type="button"
                      >
                        <div className="relative aspect-square bg-zinc-900">
                          <Image
                            alt={`Mockup view ${index + 1}`}
                            className="object-cover"
                            fill
                            src={imageUrl}
                            unoptimized
                          />
                        </div>
                        <div className="border-t border-white/10 bg-zinc-950/90 px-3 py-2 text-left text-xs font-medium text-zinc-200">
                          View {index + 1}
                        </div>
                      </button>
                    ))}
                  </div>
                </div>
              ) : null}
              {activeView === "mockup" ? (
                <div className="rounded-[1rem] border border-white/10 bg-zinc-950/80 px-4 py-3 text-xs leading-6 text-zinc-300">
                  Press <span className="font-semibold text-white">Z</span> to switch
                  between fit view and detail zoom.
                </div>
              ) : null}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
