import { afterEach, describe, expect, it, vi } from "vitest";
import { getCommercialOverview, parseCommercialOverview } from "./commercial";

export function commercialFixture() {
  const metrics = ["listingkit_generations_succeeded", "product_image_jobs_succeeded", "shein_drafts_succeeded", "shein_publishes_succeeded", "storage_bytes_current"];
  return { organization_id: "org-B", observed_at: "2026-09-07T00:00:00Z", plans: [{code:"base_payg",name:"基础方案 · 按需使用",source:"approved_product_description",availability:"not_for_sale",price:null,currency:null}], subscription: null, entitlements: [], usage: metrics.map((metric,index) => ({module_code:index===4?"oss_storage":"listingkit",metric,source:"subscription_usage_ledger",unit:index===4?"byte":"operation",period_key:index===4?"__current__":"2026-09",window_start:index===4?null:"2026-09-01T00:00:00Z",window_end:index===4?null:"2026-10-01T00:00:00Z",state:"unknown",committed:null,reserved:null,updated_at:null})), resource_balance:{state:"unsupported",value:null},cash_balance:{state:"unsupported",value:null} };
}
afterEach(() => vi.unstubAllGlobals());
describe("commercial contract", () => {
  it.each(["custom plan", "定制方案"])("preserves existing owner plan code %j", planCode => {
    const fixture = commercialFixture();
    const subscription = { plan_code: planCode, plan_name: "Current paid contract", status: "active", effective_status: "active", starts_at: null, expires_at: null, updated_at: fixture.observed_at };
    expect(parseCommercialOverview({ ...fixture, subscription })?.subscription?.plan_code).toBe(planCode);
    expect(parseCommercialOverview({ ...fixture, subscription: { ...subscription, plan_code: "界".repeat(43) } })).toBeNull();
  });
  it.each(["not-number", "1.5", "1e3", "", "+1", "01", "9223372036854775808", "-9223372036854775809"])("returns null without throwing for malformed integer %j in every quantity position", value => {
    const fixture = commercialFixture();
    for (const index of [0, 4]) {
      for (const field of ["committed", "reserved"]) {
        const input = { ...fixture, usage: fixture.usage.map((row, i) => i === index ? { ...row, state: "known", committed: "1", reserved: "0", updated_at: fixture.observed_at, [field]: value } : row) };
        expect(parseCommercialOverview(input)).toBeNull();
      }
    }
    for (const field of ["raw_value", "value"]) {
      const limit = { metric: "store_count", source_key: "store_count", unit: "store", kind: "finite", raw_value: "1", value: "1", [field]: value };
      const entitlement = { module_code: "store_management", status: "active", effective_status: "active", starts_at: null, expires_at: null, updated_at: fixture.observed_at, limits_scope: "explicit_grant_only", uninterpreted_limit_count: 0, limits: [limit] };
      expect(parseCommercialOverview({ ...fixture, entitlements: [entitlement] })).toBeNull();
    }
  });
  it("preserves missing and signed storage values without Number conversion", () => {
    const fixture = commercialFixture();
    const input = {...fixture, usage:fixture.usage.map((u,i)=>i===4?{...u,state:"known",committed:"9007199254740993",reserved:"-1",updated_at:fixture.observed_at}:u)};
    expect(parseCommercialOverview(input)?.usage[4].committed).toBe("9007199254740993");
    expect(parseCommercialOverview(input)?.usage[0].committed).toBeNull();
    expect(parseCommercialOverview({...input,cash_balance:{state:"unsupported",value:"0"}})).toBeNull();
    expect(parseCommercialOverview({...input,usage:input.usage.map((u,i)=>i===0?{...u,state:"known",committed:1,reserved:"0",updated_at:fixture.observed_at}:u)})).toBeNull();
  });
  it("binds the client to expected organization and refuses a different result", async () => {
    const fetchMock=vi.fn().mockResolvedValue(Response.json(commercialFixture())); vi.stubGlobal("fetch",fetchMock);
    await expect(getCommercialOverview("org-B")).resolves.toMatchObject({organization_id:"org-B"});
    expect(fetchMock.mock.calls[0][1]).toMatchObject({method:"GET",cache:"no-store",redirect:"manual",headers:{"X-Expected-Organization-ID":"org-B"}});
    await expect(getCommercialOverview("org-C")).rejects.toMatchObject({code:"INVALID_UPSTREAM_RESPONSE"});
  });
  it("refuses malformed and oversized actual bytes", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response('{"organization_id":"org-B","organization_id":"org-C"}',{headers:{"content-type":"application/json"}})));
    await expect(getCommercialOverview("org-B")).rejects.toMatchObject({code:"INVALID_UPSTREAM_RESPONSE"});
    vi.stubGlobal("fetch",vi.fn().mockResolvedValue(new Response(" ".repeat(65537),{headers:{"content-type":"application/json","content-length":"1"}})));
    await expect(getCommercialOverview("org-B")).rejects.toMatchObject({code:"INVALID_UPSTREAM_RESPONSE"});
  });
  it("rejects mismatched metric units and alias sources",()=>{
    const fixture=commercialFixture();
    const grant={module_code:"listingkit",status:"active",effective_status:"active",starts_at:null,expires_at:null,updated_at:fixture.observed_at,limits_scope:"explicit_grant_only",uninterpreted_limit_count:0,limits:[{metric:"product_image_jobs_succeeded",source_key:"product_image_jobs",unit:"operation",kind:"finite",raw_value:"7",value:"7"}]};
    expect(parseCommercialOverview({...fixture,entitlements:[grant]})).not.toBeNull();
    expect(parseCommercialOverview({...fixture,entitlements:[{...grant,limits:[{...grant.limits[0],unit:"byte"}]}]})).toBeNull();
    expect(parseCommercialOverview({...fixture,entitlements:[{...grant,limits:[{...grant.limits[0],source_key:"internal_secret"}]}]})).toBeNull();
  });
});
