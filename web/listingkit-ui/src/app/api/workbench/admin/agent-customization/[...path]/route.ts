import { handleAgentCustomization, rejectAgentCustomization } from "@/lib/server/agent-customization-route";
export const GET = handleAgentCustomization;
export const POST = handleAgentCustomization;
export const PUT = rejectAgentCustomization;
export const PATCH = rejectAgentCustomization;
export const DELETE = rejectAgentCustomization;
export const HEAD = rejectAgentCustomization;
export const OPTIONS = rejectAgentCustomization;
