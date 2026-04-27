import { NextResponse } from "next/server";

import {
  listSheinStudioBatches,
  saveSheinStudioBatch,
} from "@/lib/server/shein-studio-storage";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
} from "@/lib/types/shein-studio";

export const dynamic = "force-dynamic";

type BatchPayload = {
  id?: string;
  prompt: string;
  styleCount: string;
  productImageCount?: string;
  productImagePrompt?: string;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  transparentBackground?: boolean;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
};

export async function GET() {
  const batches = await listSheinStudioBatches();
  return NextResponse.json({ batches });
}

export async function POST(request: Request) {
  try {
    const payload = (await request.json()) as BatchPayload;
    const batch = await saveSheinStudioBatch(payload);
    return NextResponse.json({ batch });
  } catch (error) {
    return NextResponse.json(
      {
        error: "shein_studio_batch_save_failed",
        message: error instanceof Error ? error.message : "unknown error",
      },
      { status: 400 },
    );
  }
}
