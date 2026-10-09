import { NextRequest } from "next/server";
import { dispatchData } from "@/lib/server/data-services-dispatch";
export const dynamic = "force-dynamic";
export const GET = (request: NextRequest, context: {
    params: Promise<{
        path?: string[];
    }>;
}) => dispatchData(request, context, true);
export const POST = GET;
