import { NextResponse } from "next/server";

import { buildSheinStyleGallery } from "@/lib/server/shein-style-gallery";

export const dynamic = "force-dynamic";

export async function GET() {
  return NextResponse.json(await buildSheinStyleGallery());
}
