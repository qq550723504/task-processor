import {expect,it,vi,afterEach} from "vitest";
import {buildWorkbenchUpstreamRequest,WORKBENCH_COOKIE_NAME} from "./workbench-proxy";
afterEach(()=>vi.unstubAllEnvs());
it("passes admitted market and SDS source filters through the existing supply BFF",async()=>{
 vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://localhost:8080/api/v1");
 const id="11111111-1111-4111-8111-111111111111";
 for(const sourceKind of ["market","sds_template","sds_finished"]){
  const path=["supply-preparations",id,"stages"];
  const request=new Request(`https://console.example.test/api/workbench/${path.join("/")}?storeId=${id}&stage=all&sourceKind=${sourceKind}`,{headers:{cookie:`${WORKBENCH_COOKIE_NAME}=org-a`,"X-Expected-Organization-ID":"org-a","X-Expected-User-ID":"actor-a"}});
  const result=await buildWorkbenchUpstreamRequest(request,path,"server-token","actor-a");expect(result).not.toBeInstanceOf(Response);
 }
});
