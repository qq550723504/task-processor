import type {CommercialOverview} from "@/lib/api/commercial";
import {commercialResourcesFixture} from "./commercial-resources";
// Synthetic component data, never an application fallback.
export function commercialOverviewFixture(organizationId="org-B"):CommercialOverview{
 return {schema_version:"unified-base-prepaid-v1",organization_id:organizationId,observed_at:"2026-09-28T00:00:00Z",base_plan:{code:"base_payg",name:"基础方案",subscription_required:false,store_period_days:30,ai_limit_period:"utc_calendar_month",resource_expiry:"none"},resources:{state:"available",value:commercialResourcesFixture(organizationId)},store_services:{state:"available",value:{records:3,active:2,expired:1,expiring_soon:1}}};
}
