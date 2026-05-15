import { NextResponse } from "next/server";

import {
  deleteSheinStudioBatch,
  getSheinStudioBatch,
} from "@/lib/server/shein-studio-storage";

export const dynamic = "force-dynamic";
export const runtime = "nodejs";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ batchId: string }> },
) {
  const { batchId } = await params;
  const batch = await getSheinStudioBatch(batchId);
  if (!batch) {
    return NextResponse.json(
      { error: "shein_studio_batch_not_found", message: "batch not found" },
      { status: 404 },
    );
  }

  return NextResponse.json({ batch });
}

export async function DELETE(
  _request: Request,
  { params }: { params: Promise<{ batchId: string }> },
) {
  const { batchId } = await params;
  await deleteSheinStudioBatch(batchId);
  return NextResponse.json({ ok: true });
}
