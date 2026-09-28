import { handleInvitation } from "@/lib/server/invitations-route";
export { PUT, PATCH, DELETE, HEAD, OPTIONS } from "@/lib/server/members-route";
export const GET = handleInvitation;
export const POST = handleInvitation;
export const runtime = "nodejs";
export const dynamic = "force-dynamic";
