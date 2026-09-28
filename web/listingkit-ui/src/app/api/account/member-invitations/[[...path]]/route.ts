import { handleMembers } from "@/lib/server/members-route";
export { PUT, PATCH, DELETE, HEAD, OPTIONS } from "@/lib/server/members-route";
export const GET = handleMembers;
export const POST = handleMembers;
export const runtime = "nodejs";
export const dynamic = "force-dynamic";
