// @vitest-environment node
import { afterEach, describe, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
import { readRetailPrices } from "./retail-prices";
import { parseRetailPriceCatalog } from "@/lib/api/retail-prices";

const catalog = {
  schema_version: "retail-price-catalog-v1", store_period_days: 30,
  items: [{ offer_id: "store-service-30d-v1", product_kind: "STORE_RENEWAL_PERIOD", resource_type: "store_renewal_period", currency: "CNY", unit_price_minor: "16800", min_quantity: "1", max_quantity: "120", pricing_version: "store-168-v1" }],
};
afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals(); vi.useRealTimers(); });

describe("retail price read", () => {
  it("reads the fixed public path without credentials or a cache", async () => {
    vi.stubEnv("COMMERCIAL_API_ORIGIN", "http://localhost:8085/");
    const fetch = vi.fn().mockResolvedValue(Response.json(catalog));
    vi.stubGlobal("fetch", fetch);
    expect(await readRetailPrices()).toEqual(catalog);
    expect(fetch).toHaveBeenCalledWith("http://localhost:8085/api/v1/commercial/resource-offers", expect.objectContaining({ method:"GET", headers:{Accept:"application/json"}, redirect:"manual", cache:"no-store" }));
  });
  it.each([undefined,"http://user:password@localhost:8085","http://localhost:8085/private","file:///tmp/secret"]) ("does not dispatch for an invalid origin %s", async origin => {
    vi.stubEnv("COMMERCIAL_API_ORIGIN",origin);
    const fetch=vi.fn(); vi.stubGlobal("fetch",fetch);
    expect(await readRetailPrices()).toBeNull(); expect(fetch).not.toHaveBeenCalled();
  });
  it.each([
    () => Response.json({ ...catalog, organization_id:"private-org" }),
    () => Response.json({ ...catalog, items:[{...catalog.items[0],unit_price_minor:"1.68"}] }),
    () => Response.json({ ...catalog, items:[{...catalog.items[0],unit_price_minor:"9223372036854775808"}] }),
    () => Response.json({ ...catalog, items:[catalog.items[0],catalog.items[0]] }),
    () => new Response('{"schema_version":"wrong","schema_version":"retail-price-catalog-v1","store_period_days":30,"items":[]}',{headers:{"Content-Type":"application/json"}}),
    () => new Response(" ".repeat(32*1024+1),{headers:{"Content-Type":"application/json"}}),
    () => new Response("<html>unavailable</html>",{headers:{"Content-Type":"text/html"}}),
    () => new Response(null,{status:302,headers:{Location:"https://untrusted.example"}}),
    () => new Response(null,{status:503}),
  ])("fails closed for malformed or unavailable responses",async make => {
    vi.stubEnv("COMMERCIAL_API_ORIGIN","http://localhost:8085");
    vi.stubGlobal("fetch",vi.fn().mockResolvedValue(make()));
    expect(await readRetailPrices()).toBeNull();
  });
  it("aborts an incomplete response at the bounded deadline", async () => {
    vi.useFakeTimers(); vi.stubEnv("COMMERCIAL_API_ORIGIN","http://localhost:8085");
    vi.stubGlobal("fetch",vi.fn().mockResolvedValue(new Response(new ReadableStream({start(controller){controller.enqueue(new TextEncoder().encode('{"schema_version":'));}}),{headers:{"Content-Type":"application/json"}})));
    const pending=readRetailPrices();
    await vi.advanceTimersByTimeAsync(5001);
    expect(await pending).toBeNull();
  });
  it("keeps an empty catalog distinct from invalid data",()=>{
    expect(parseRetailPriceCatalog({...catalog,items:[]})).toEqual({...catalog,items:[]});
    expect(parseRetailPriceCatalog({...catalog,store_period_days:31})).toBeNull();
  });
});
