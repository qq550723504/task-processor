import { NextResponse } from "next/server";

import { buildStyleGallery } from "@/lib/server/style-gallery";

export const dynamic = "force-dynamic";

export async function GET() {
  return NextResponse.json(await buildStyleGallery());
}
