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
  try {
    const payload = (await request.json()) as DraftPayload;
    const draft = await saveSheinStudioDraft(payload);
    return NextResponse.json({ draft });
  } catch (error) {
    return NextResponse.json(
      {
        error: "shein_studio_draft_save_failed",
        message: error instanceof Error ? error.message : "unknown error",
      },
      { status: 400 },
    );
  }
}
