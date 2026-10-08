import {cleanup,render,screen,waitFor} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {afterEach,beforeEach,expect,it,vi} from "vitest";
import {BulkSupplyReviewPanel} from "./bulk-review-panel";
import type {SupplyCommandState} from "./use-supply-command";
import type {SupplyOperationItem} from "@/lib/contracts/supply-chain";
const mocks=vi.hoisted(()=>({read:vi.fn(),proposal:vi.fn(),execute:vi.fn(),guard:vi.fn(()=>()=>{})}));
vi.mock("@/lib/api/supply-chain",()=>({readSupplyRecord:mocks.read}));
vi.mock("@/lib/api/product-title-review-client",()=>({fetchProductTitleProposal:mocks.proposal}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>({registerOrganizationSwitchGuard:mocks.guard})}));
const items=[1,2,3].map(i=>({sourceId:`source-${i}`,recordId:`record-${i}`,recordRevision:1,status:"review",resultReference:`11111111-1111-4111-8111-11111111111${i}`} as SupplyOperationItem));
function proposal(i:number){return {schema_version:1,coverage:"product-title-proposals-only",proposal_id:items[i-1].resultReference,owner:"actor",input:{product_key:`product-${i}`,base_version:"1"},before:`原始标题 ${i}`,after:`建议标题 ${i}`,original_title:`建议标题 ${i}`,policy:"title-review-v1",state:"pending",revision:"1",evidence:[],quality:{overall:1,evidence_coverage:1,required_field_coverage:1},unresolved:[],decisions:[]};}
function command(){return {busy:false,pending:null,ready:true,execute:mocks.execute} as unknown as SupplyCommandState;}
beforeEach(()=>{mocks.read.mockReset().mockImplementation((_s,id)=>{const i=Number(id.split("-")[1]);return Promise.resolve({id,revision:1,effectiveVersion:"1",source:{id:items[i-1].sourceId,source:{productKey:`product-${i}`}},merchant:{store_id:"store"}})});mocks.proposal.mockReset().mockImplementation(({proposalId})=>Promise.resolve(proposal(Number(proposalId.slice(-1)))));mocks.execute.mockReset().mockImplementation((_r,c)=>Promise.resolve({...proposal(Number(c.proposalId.slice(-1))),state:"accepted",revision:"2"}));});afterEach(cleanup);
it("previews each actual proposal and only accepts selected revisions after explicit confirmation",async()=>{
 render(<BulkSupplyReviewPanel scope={{userId:"actor",organizationId:"org"}} storeId="store" items={items} command={command()} permissions={["listingkit.admin.write"]}/>);
 await screen.findByText("建议标题 1");expect(mocks.execute).not.toHaveBeenCalled();expect(screen.getByRole("button",{name:"确认批量审核通过"})).toBeDisabled();
 await userEvent.click(screen.getByLabelText("我已核对以上全部建议"));await userEvent.click(screen.getByRole("button",{name:"确认批量审核通过"}));
 await screen.findByText("已批准 3 件建议；请分别确认应用并补全目标资料。");expect(mocks.execute.mock.calls).toHaveLength(3);
 expect(mocks.execute.mock.calls[0]).toEqual(["review-decision",{proposalId:items[0].resultReference,sourceId:"source-1",recordId:"record-1",productKey:"product-1",baseVersion:"1",input:{action:"accept",expected_revision:"1"}}]);
});
it("stops at an uncertain item and never submits later items or Apply",async()=>{
 mocks.execute.mockImplementationOnce(()=>Promise.resolve({...proposal(1),state:"accepted",revision:"2"})).mockImplementationOnce(()=>Promise.resolve(undefined));
 render(<BulkSupplyReviewPanel scope={{userId:"actor",organizationId:"org"}} storeId="store" items={items} command={command()} permissions={["listingkit.admin.write"]}/>);await screen.findByText("建议标题 3");await userEvent.click(screen.getByLabelText("我已核对以上全部建议"));await userEvent.click(screen.getByRole("button",{name:"确认批量审核通过"}));
 await screen.findByText(/已批准 1 件，处理已停止/);expect(mocks.execute).toHaveBeenCalledTimes(2);expect(mocks.execute.mock.calls.every(c=>c[0]==="review-decision")).toBe(true);
});
it("rejects a mismatched record before any batch decision",async()=>{
 mocks.read.mockResolvedValue({id:"other",revision:1,source:{id:"other",source:{productKey:"other"}},merchant:{store_id:"other"},effectiveVersion:"1"});
 render(<BulkSupplyReviewPanel scope={{userId:"actor",organizationId:"org"}} storeId="store" items={items} command={command()} permissions={["listingkit.admin.write"]}/>);await screen.findByRole("alert");expect(mocks.execute).not.toHaveBeenCalled();expect(screen.queryByRole("button",{name:"确认批量审核通过"})).not.toBeInTheDocument();
});
it("keeps final approval unavailable without current administrator permission",async()=>{
 render(<BulkSupplyReviewPanel scope={{userId:"actor",organizationId:"org"}} storeId="store" items={items} command={command()} permissions={[]}/>);await waitFor(()=>expect(screen.getByRole("button",{name:"确认批量审核通过"})).toBeDisabled());expect(mocks.execute).not.toHaveBeenCalled();
});
