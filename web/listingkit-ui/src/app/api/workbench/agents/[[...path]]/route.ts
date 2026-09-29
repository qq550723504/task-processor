export const dynamic = "force-dynamic";
export {
  handleAgentConfiguration as GET,
  handleAgentConfiguration as POST,
  handleAgentConfiguration as PUT,
  rejectAgentConfiguration as PATCH,
  rejectAgentConfiguration as DELETE,
  rejectAgentConfiguration as HEAD,
  rejectAgentConfiguration as OPTIONS,
} from "@/lib/server/agent-configuration-route";
