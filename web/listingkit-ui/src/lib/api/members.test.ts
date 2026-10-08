import { afterEach, describe, expect, it, vi } from "vitest";
import { getMembers, parseMembers, invitationInput, getMemberOperations, parseMemberOperations, getMemberOperation, verifyMemberOperation, changeMemberRole, removeMember } from "./members";

const empty = { schemaVersion: "membership-v1", userId: "actor", organizationId: "org", items: [], total: 0, canManage: false, assignableRoles:[],roleDefinitions:[] };
afterEach(() => vi.unstubAllGlobals());
describe("membership read boundary", () => {
  it("sends encoded directory filters and resets only the requested page",async()=>{
    const fetch=vi.fn().mockResolvedValue(Response.json(empty));vi.stubGlobal("fetch",fetch);
    await getMembers({expectedUserId:"actor",expectedOrganizationId:"org"},20,{q:"  目标+&%  ",role:"sumi_role_e87cb45c05ad389dff6dea6e7bf581ee_02",state:"inactive"});
    const url=new URL(fetch.mock.calls[0][0],"http://localhost");
    expect(url.searchParams.get("q")).toBe("目标+&%");
    expect(url.searchParams.get("role")).toBe("sumi_role_e87cb45c05ad389dff6dea6e7bf581ee_02");
    expect(url.searchParams.get("state")).toBe("inactive");
    expect(url.searchParams.get("offset")).toBe("20");
  });
  it("rejects invalid directory query inputs before fetch",async()=>{
    const fetch=vi.fn();vi.stubGlobal("fetch",fetch);
    const scope={expectedUserId:"actor",expectedOrganizationId:"org"};
    for (const q of ["a".repeat(201),"目".repeat(67),"x\u0000", "\ud800"]) await expect(getMembers(scope,0,{q})).rejects.toMatchObject({code:"INVALID_REQUEST"});
    for (const offset of [-1,10001,0.5]) await expect(getMembers(scope,offset)).rejects.toMatchObject({code:"INVALID_REQUEST"});
    expect(fetch).not.toHaveBeenCalled();
    fetch.mockResolvedValue(Response.json(empty));await getMembers(scope,0,{q:"目".repeat(66)+"ab"});expect(fetch).toHaveBeenCalledTimes(1);
  });
  it("rejects invalid invitation identity before storing a command", () => {
    const input = {email:"test@example.com",firstName:"Test",lastName:"Member",role:"sumi_role_e87cb45c05ad389dff6dea6e7bf581ee_01"};
    for (const fields of [{email:"test@phone.invalid"},{firstName:"   "},{lastName:"Member\u0001"}]) expect(invitationInput.safeParse({...input,...fields}).success).toBe(false);
  });
  it("rejects a response for another organization", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...empty, organizationId: "foreign" }), { headers: { "Content-Type": "application/json" } })));
    await expect(getMembers({ expectedUserId: "actor", expectedOrganizationId: "org" })).rejects.toMatchObject({ code: "ORGANIZATION_CONTEXT_CHANGED" });
  });
  it("never converts provider errors to an empty directory", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: "DEPENDENCY_UNAVAILABLE", message: "Unavailable", requestId: "", fieldErrors: [] }), { status: 503, headers: { "Content-Type": "application/json" } })));
    await expect(getMembers({ expectedUserId: "actor", expectedOrganizationId: "org" })).rejects.toMatchObject({ code: "DEPENDENCY_UNAVAILABLE" });
  });
  it("rejects management options without backend manage permission", () => {
    expect(() => parseMembers({ ...empty, assignableRoles:["listingkit_admin"],roleDefinitions:[] })).toThrow();
    expect(parseMembers(empty).canManage).toBe(false);
  });
  it("does not send a canceled scope request", async () => {
    const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
    const controller = new AbortController(); controller.abort();
    await expect(getMembers({ expectedUserId: "actor", expectedOrganizationId: "org", signal: controller.signal })).rejects.toMatchObject({ code: "DEADLINE_EXCEEDED" });
    expect(fetch).not.toHaveBeenCalled();
  });
});

const pending={schemaVersion:"membership-operation-v1",userId:"actor",organizationId:"org",id:"ea0390e6-6fd0-4834-8e9c-277caf59c122",kind:"invite",step:"create_user",status:"unknown",targetUserId:"target",authorizationId:"",userEvidence:"",userAcknowledgment:null,acknowledgment:null,observation:"unavailable",observed:null};
const list={schemaVersion:"membership-operations-v1",userId:"actor",organizationId:"org",items:[pending],next:""};
it("binds every single receipt to its requested operation ID",async()=>{
  vi.stubGlobal("fetch",vi.fn().mockImplementation(()=>Promise.resolve(Response.json(pending))));
  const scope={expectedUserId:"actor",expectedOrganizationId:"org"}, key="4841d296-ef14-4c16-8d25-a7667e534feb";
  for(const read of [
    ()=>getMemberOperation(scope,key),()=>verifyMemberOperation(scope,key),
    ()=>changeMemberRole(scope,key,"grant",{role:"sumi_role_e87cb45c05ad389dff6dea6e7bf581ee_01",expectedVersion:"a".repeat(64)}),
    ()=>removeMember(scope,key,"grant",{expectedVersion:"a".repeat(64)}),
  ]) await expect(read()).rejects.toMatchObject({code:"INVALID_UPSTREAM_RESPONSE"});
});
it("rejects a pending item outside the envelope scope or repeated cursor",async()=>{
  for(const change of [{userId:"other"},{organizationId:"other"},{status:"rejected"}]) expect(()=>parseMemberOperations({...list,items:[{...pending,...change}]})).toThrow();
  expect(()=>parseMemberOperations({...list,items:[pending,pending]})).toThrow();
  vi.stubGlobal("fetch",vi.fn().mockResolvedValue(Response.json(list)));
  await expect(getMemberOperations({expectedUserId:"actor",expectedOrganizationId:"org"},pending.id)).rejects.toMatchObject({code:"INVALID_UPSTREAM_RESPONSE"});
});
it("requires the pending envelope to match the requesting actor and org",async()=>{
  vi.stubGlobal("fetch",vi.fn().mockResolvedValue(Response.json({...list,userId:"other",items:[{...pending,userId:"other"}]})));
  await expect(getMemberOperations({expectedUserId:"actor",expectedOrganizationId:"org"})).rejects.toMatchObject({code:"IDENTITY_CONTEXT_CHANGED"});
});
