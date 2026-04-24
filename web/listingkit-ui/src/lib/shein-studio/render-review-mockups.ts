"use client";

import { resolveGeneratedDesignSrc } from "@/lib/shein-studio/design-image";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

const DEFAULT_CANVAS_WIDTH = 1600;
const DEFAULT_CANVAS_HEIGHT = 1200;
const DEFAULT_SURFACE_LIMIT = 3;

function uniqueSurfaceUrls(selection?: SDSProductVariantSelection) {
  return [
    ...(selection?.mockupImageUrls ?? []),
    selection?.mockupImageUrl,
    selection?.templateImageUrl,
  ].filter((item, index, source): item is string => Boolean(item) && source.indexOf(item) === index);
}

async function loadBitmap(url: string) {
  const response = await fetch(url, { cache: "no-store" });
  if (!response.ok) {
    throw new Error(`load image failed: ${response.status}`);
  }
  const blob = await response.blob();
  return createImageBitmap(blob);
}

function containRect(input: {
  targetWidth: number;
  targetHeight: number;
  sourceWidth: number;
  sourceHeight: number;
}) {
  const sourceRatio = input.sourceWidth / input.sourceHeight;
  const targetRatio = input.targetWidth / input.targetHeight;

  if (sourceRatio > targetRatio) {
    const width = input.targetWidth;
    const height = width / sourceRatio;
    return {
      width,
      height,
      x: 0,
      y: (input.targetHeight - height) / 2,
    };
  }

  const height = input.targetHeight;
  const width = height * sourceRatio;
  return {
    width,
    height,
    x: (input.targetWidth - width) / 2,
    y: 0,
  };
}

function overlayFrame(selection?: SDSProductVariantSelection) {
  const printableWidth = selection?.printableWidth ?? 1;
  const printableHeight = selection?.printableHeight ?? 1;
  const printableRatio = printableWidth / printableHeight;

  const frameWidth = DEFAULT_CANVAS_WIDTH * 0.52;
  const frameHeight = DEFAULT_CANVAS_HEIGHT * 0.72;
  const fit = containRect({
    targetWidth: frameWidth,
    targetHeight: frameHeight,
    sourceWidth: printableRatio >= 1 ? printableRatio : 1,
    sourceHeight: printableRatio >= 1 ? 1 : 1 / printableRatio,
  });

  return {
    x: DEFAULT_CANVAS_WIDTH * 0.24 + fit.x,
    y: DEFAULT_CANVAS_HEIGHT * 0.14 + fit.y,
    width: fit.width,
    height: fit.height,
  };
}

async function canvasToFile(canvas: HTMLCanvasElement, fileName: string) {
  const blob = await new Promise<Blob>((resolve, reject) => {
    canvas.toBlob((result) => {
      if (!result) {
        reject(new Error("render mockup blob is empty"));
        return;
      }
      resolve(result);
    }, "image/png");
  });

  return new File([blob], fileName, { type: "image/png" });
}

async function renderOneMockup(input: {
  designUrl: string;
  surfaceUrl: string;
  selection?: SDSProductVariantSelection;
  fileName: string;
}) {
  const [surfaceBitmap, designBitmap] = await Promise.all([
    loadBitmap(input.surfaceUrl),
    loadBitmap(input.designUrl),
  ]);

  const canvas = document.createElement("canvas");
  canvas.width = DEFAULT_CANVAS_WIDTH;
  canvas.height = DEFAULT_CANVAS_HEIGHT;
  const context = canvas.getContext("2d");
  if (!context) {
    throw new Error("mockup canvas context is unavailable");
  }

  context.fillStyle = "#111111";
  context.fillRect(0, 0, canvas.width, canvas.height);

  const surfaceRect = containRect({
    targetWidth: canvas.width,
    targetHeight: canvas.height,
    sourceWidth: surfaceBitmap.width,
    sourceHeight: surfaceBitmap.height,
  });
  context.drawImage(
    surfaceBitmap,
    surfaceRect.x,
    surfaceRect.y,
    surfaceRect.width,
    surfaceRect.height,
  );

  const frame = overlayFrame(input.selection);
  context.save();
  context.shadowColor = "rgba(15, 23, 42, 0.35)";
  context.shadowBlur = 28;
  context.fillStyle = "rgba(255,255,255,0.96)";
  context.beginPath();
  context.roundRect(frame.x, frame.y, frame.width, frame.height, 22);
  context.fill();
  context.restore();

  const designRect = containRect({
    targetWidth: frame.width,
    targetHeight: frame.height,
    sourceWidth: designBitmap.width,
    sourceHeight: designBitmap.height,
  });

  context.drawImage(
    designBitmap,
    frame.x + designRect.x,
    frame.y + designRect.y,
    designRect.width,
    designRect.height,
  );

  return canvasToFile(canvas, input.fileName);
}

export async function buildReviewMockupFiles(input: {
  design: SheinStudioGeneratedDesign;
  selection?: SDSProductVariantSelection;
  maxCount?: number;
}) {
  const designUrl = resolveGeneratedDesignSrc(input.design);
  if (!designUrl) {
    throw new Error("Generated design image is missing.");
  }

  const surfaces = uniqueSurfaceUrls(input.selection).slice(0, input.maxCount ?? DEFAULT_SURFACE_LIMIT);
  if (surfaces.length === 0) {
    const response = await fetch(designUrl);
    const blob = await response.blob();
    return [new File([blob], `${input.design.id}-design.png`, { type: blob.type || "image/png" })];
  }

  return Promise.all(
    surfaces.map((surfaceUrl, index) =>
      renderOneMockup({
        designUrl,
        surfaceUrl,
        selection: input.selection,
        fileName: `${input.design.id}-mockup-${index + 1}.png`,
      }),
    ),
  );
}
