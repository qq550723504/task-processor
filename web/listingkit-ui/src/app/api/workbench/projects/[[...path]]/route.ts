import { handleProjectCenter, rejectProjectCenter } from "@/lib/server/project-center-route";
export const runtime="nodejs";
export const dynamic="force-dynamic";
export const GET=handleProjectCenter;
export const POST=handleProjectCenter;
export const PATCH=handleProjectCenter;
export const PUT=rejectProjectCenter;
export const DELETE=rejectProjectCenter;
