import { readFile, stat } from "node:fs/promises";
import path from "node:path";

import { NextResponse } from "next/server";

import { resolveGalleryImagePath } from "@/lib/server/shein-style-gallery";

export const dynamic = "force-dynamic";

const CONTENT_TYPES: Record<string, string> = {
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".png": "image/png",
  ".webp": "image/webp",
};

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ source: string; path: string[] }> },
) {
  const { source, path: segments } = await params;
  const filePath = resolveGalleryImagePath(source, segments.map(decodeURIComponent));
  if (!filePath) {
    return NextResponse.json({ error: "invalid_image_path" }, { status: 400 });
  }

  try {
    const info = await stat(filePath);
    if (!info.isFile()) {
      return NextResponse.json({ error: "image_not_found" }, { status: 404 });
    }
    const body = await readFile(filePath);
    return new NextResponse(body, {
      headers: {
        "Cache-Control": "public, max-age=3600",
        "Content-Type": CONTENT_TYPES[path.extname(filePath).toLowerCase()] ?? "application/octet-stream",
      },
    });
  } catch {
    return NextResponse.json({ error: "image_not_found" }, { status: 404 });
  }
}
