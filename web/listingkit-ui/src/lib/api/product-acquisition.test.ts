import { afterEach, describe, expect, it, vi } from "vitest";
import { acquire1688, verify1688, readAcquisition } from "./product-acquisition";

const key="01991e24-61ab-4f5f-85d1-3157bb8b75c1";
const operation=Object.freeze({key,source:"981645030344",userId:"actor",organizationId:"B"});
const result={schemaVersion:1,operationId:key,outcome:"published",replayed:false,productKey:"crawler:1688:981645030344",publicationId:`source-run:acquisition:${key}`,catalogVersion:"1",warnings:[],missingFacts:[]};
afterEach(()=>vi.unstubAllGlobals());
describe("acquisition client",()=>{
  it("uses explicit original intent, safe same-origin transport and verify only",async()=>{
    const fetcher=vi.fn().mockImplementation(async()=>Response.json(result));vi.stubGlobal("fetch",fetcher);
    expect(await acquire1688(operation)).toEqual(result);expect(await verify1688(operation)).toEqual(result);expect(await readAcquisition(key,operation)).toEqual(result);
    expect(fetcher).toHaveBeenCalledTimes(3);
    expect(fetcher.mock.calls[1]![0]).toBe("/api/workbench/sourcing/1688/acquisitions/verify");
    for(const call of fetcher.mock.calls){const init=call[1] as RequestInit;const headers=new Headers(init.headers);expect(headers.get("X-Expected-User-ID")).toBe("actor");expect(headers.get("X-Expected-Organization-ID")).toBe("B");expect(headers.has("Authorization")).toBe(false);expect(init.credentials).toBe("same-origin");}
    expect(fetcher.mock.calls[0]![1].body).toBe('{"source":"981645030344"}');expect(new Headers(fetcher.mock.calls[1]![1].headers).get("Idempotency-Key")).toBe(key);
  });
  it("never auto-retries or rekeys a lost response",async()=>{
    const fetcher=vi.fn().mockRejectedValue(new TypeError("network lost"));vi.stubGlobal("fetch",fetcher);
    await expect(acquire1688(operation)).rejects.toMatchObject({code:"OUTCOME_UNKNOWN"});expect(fetcher).toHaveBeenCalledTimes(1);
  });
  it("rejects unsafe result binding and invalid context before dispatch",async()=>{
    const fetcher=vi.fn().mockResolvedValue(Response.json({...result,publicationId:"other"}));vi.stubGlobal("fetch",fetcher);
    await expect(acquire1688({...operation,organizationId:""})).rejects.toMatchObject({code:"INVALID_REQUEST"});expect(fetcher).not.toHaveBeenCalled();
    await expect(acquire1688(operation)).rejects.toMatchObject({code:"OUTCOME_UNKNOWN"});
  });
});
