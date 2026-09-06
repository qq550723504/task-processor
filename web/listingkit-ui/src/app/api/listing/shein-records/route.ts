import type { NextRequest } from "next/server";
import { handleSheinRecordsGET } from "@/lib/server/shein-records-route";
export const dynamic = "force-dynamic";
export function GET(request: NextRequest) { return handleSheinRecordsGET(request); }
export { POST, PUT, PATCH, DELETE, HEAD, OPTIONS } from "@/lib/server/shein-records-route";
