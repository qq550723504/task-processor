import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { proxyCockpit } from "./operations-cockpit-proxy";
beforeEach(() => vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "https://app.test"));
afterEach(() => {vi.unstubAllGlobals();vi.unstubAllEnvs();});
const id = "123e4567-e89b-42d3-a456-426614174000";
const headers = { "X-Expected-User-ID": "actor-a", "X-Expected-Organization-ID": "org-a", Cookie: "shuomi_effective_organization=org-a", Origin: "https://app.test", "Content-Type": "application/json", "Idempotency-Key": id };
it("rejects stale organization and incomplete cost facts before dispatch", async () => {
 const fetcher = vi.fn();vi.stubGlobal("fetch",fetcher);
 const stale = new Request("https://app.test/api/operations-cockpit/capabilities",{headers:{...headers,"X-Expected-Organization-ID":"org-b"}});
 expect((await proxyCockpit(stale,"private-fixture-token","actor-a")).status).toBe(409);
 const body = {id,storeId:id,expectedRevision:"0",fact:{period:{startDate:"2026-10-01",endDate:"2026-10-01"},amounts:{revenue:100},note:""}};
 const partial = new Request("https://app.test/api/operations-cockpit/facts",{method:"POST",headers,body:JSON.stringify(body)});
 expect((await proxyCockpit(partial,"private-fixture-token","actor-a")).status).toBe(400);
 expect(fetcher).not.toHaveBeenCalled();
});
it("keeps an ambiguous mutation unknown when its receipt is mismatched",async()=>{
 vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://localhost:8085/api/v1");
 vi.stubGlobal("fetch",vi.fn(async()=>Response.json({commandId:id,operation:"fact_create",id:"123e4567-e89b-42d3-a456-426614174001",revision:"1",committedAt:"2026-10-10T01:00:00Z"})));
 const body={id,storeId:id,expectedRevision:"0",fact:{period:{startDate:"2026-10-01",endDate:"2026-10-01"},amounts:{revenue:100,refunds:0,procurement:0,logistics:0,platform:0,advertising:0,other:0},note:""}};
 const response=await proxyCockpit(new Request("https://app.test/api/operations-cockpit/facts",{method:"POST",headers,body:JSON.stringify(body)}),"private-fixture-token","actor-a");
 expect(await response.json()).toEqual({code:"OUTCOME_UNKNOWN"});
});
it("rejects a different fact returned for a detail URL",async()=>{
 vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://localhost:8085/api/v1");
 const wrong="123e4567-e89b-42d3-a456-426614174001";
 vi.stubGlobal("fetch",vi.fn(async()=>Response.json({id:wrong,storeId:id,revision:"1",period:{startDate:"2026-10-01",endDate:"2026-10-01"},amounts:{revenue:100,refunds:0,procurement:0,logistics:0,platform:0,advertising:0,other:0},note:"",updatedBy:"actor-a",updatedAt:"2026-10-10T01:00:00Z"})));
 const response=await proxyCockpit(new Request(`https://app.test/api/operations-cockpit/facts/${id}`,{headers}),"fixture-token","actor-a");expect(response.status).toBe(502);
});
