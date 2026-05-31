import { readAppVersionInfo } from "@/lib/app-version";

export const dynamic = "force-dynamic";

export async function GET() {
  const version = await readAppVersionInfo();

  return Response.json(version, {
    headers: {
      "Cache-Control": "no-store, no-cache, must-revalidate, proxy-revalidate",
      Pragma: "no-cache",
      Expires: "0",
    },
  });
}
