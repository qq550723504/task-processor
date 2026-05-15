import { NextResponse } from "next/server";

import {
  getSheinStudioDraft,
  saveSheinStudioDraft,
} from "@/lib/server/shein-studio-storage";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioArtworkModel,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
} from "@/lib/types/shein-studio";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

function parseSelection(searchParams: URLSearchParams): SDSProductVariantSelection | undefined {
  const variantId = Number(searchParams.get("variantId") ?? 0);
  if (!Number.isFinite(variantId) || variantId <= 0) {
    return undefined;
  }

  return {
    productId: Number(searchParams.get("productId") ?? 0) || 0,
    parentProductId:
      Number(searchParams.get("parentProductId") ?? searchParams.get("productId") ?? 0) || 0,
    variantId,
    prototypeGroupId: Number(searchParams.get("prototypeGroupId") ?? 0) || 0,
    layerId: searchParams.get("layerId") ?? "",
    productName: searchParams.get("productName") ?? "Selected SDS product",
    variantLabel: searchParams.get("variantLabel") ?? "Current variant",
    printableWidth: Number(searchParams.get("printWidth") ?? 0) || undefined,
    printableHeight: Number(searchParams.get("printHeight") ?? 0) || undefined,
    templateImageUrl: searchParams.get("templateImageUrl") ?? undefined,
    mockupImageUrl: searchParams.get("mockupImageUrl") ?? undefined,
    mockupImageUrls: undefined,
    selectedVariantIds: parseOptionalNumberArray(searchParams.get("variantIds") ?? undefined),
  };
}

function parseOptionalNumberArray(value?: string) {
  if (!value) {
    return undefined;
  }
  const items = value
    .split(",")
    .map((item) => Number(item.trim()))
    .filter((item) => Number.isFinite(item) && item > 0);
  return items.length > 0 ? Array.from(new Set(items)) : undefined;
}

type DraftPayload = {
  prompt: string;
  styleCount: string;
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  artworkModel?: SheinStudioArtworkModel;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  renderSizeImagesWithSds?: boolean;
  transparentBackground?: boolean;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
};

export async function GET(request: Request) {
  const url = new URL(request.url);
  const draft = await getSheinStudioDraft(parseSelection(url.searchParams));
  return NextResponse.json({ draft });
}

export async function PUT(request: Request) {
  const startedAt = Date.now();
  let bodyBytes = 0;
  let parseDurationMs = 0;
  let payload: DraftPayload | undefined;
  try {
    const rawBody = await request.text();
    bodyBytes = Buffer.byteLength(rawBody, "utf8");
    const parseStartedAt = Date.now();
    payload = JSON.parse(rawBody) as DraftPayload;
    parseDurationMs = Date.now() - parseStartedAt;

    console.info("[shein-studio-draft] server save started", {
      bodyBytes,
      designCount: payload.designs.length,
      draftSaveStatus: "started",
      selectionVariantId: payload.selection?.variantId ?? null,
    });

    const writeStartedAt = Date.now();
    const draft = await saveSheinStudioDraft(payload);
    console.info("[shein-studio-draft] server save completed", {
      bodyBytes,
      designCount: payload.designs.length,
      draftSaveDurationMs: Date.now() - startedAt,
      draftSaveStatus: "succeeded",
      parseDurationMs,
      selectionVariantId: payload.selection?.variantId ?? null,
      writeDurationMs: Date.now() - writeStartedAt,
    });
    return NextResponse.json({ draft });
  } catch (error) {
    console.warn("[shein-studio-draft] server save failed", {
      bodyBytes,
      designCount: payload?.designs.length ?? 0,
      draftSaveDurationMs: Date.now() - startedAt,
      draftSaveStatus: "failed",
      error: error instanceof Error ? error.message : "unknown error",
      parseDurationMs,
      selectionVariantId: payload?.selection?.variantId ?? null,
    });
    return NextResponse.json(
      {
        error: "shein_studio_draft_save_failed",
        message: error instanceof Error ? error.message : "unknown error",
      },
      { status: 400 },
    );
  }
}
