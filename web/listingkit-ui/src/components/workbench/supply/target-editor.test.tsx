import { cleanup,render,screen,waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach,beforeEach,expect,it,vi } from "vitest";
import { SupplyTargetEditor } from "./target-editor";
import type { SupplySourceDetail,SupplyRules } from "@/lib/contracts/supply-chain";
import type { SupplyCommandState } from "./use-supply-command";
const state=vi.hoisted(()=>({rules:vi.fn(),inventory:vi.fn(),execute:vi.fn(),imagePanel:vi.fn()}));
vi.mock("../acquisition/product-image-set-panel",()=>({ProductImageSetPanel:(props:{onSaved:()=>void;effectiveVersion:string})=>{state.imagePanel(props);return <button onClick={props.onSaved}>保存图片测试回调</button>}}));
vi.mock("@/lib/api/supply-chain",async original=>({...await original<object>(),supplyRules:state.rules,supplyInventory:state.inventory}));
const id="11111111-1111-4111-8111-111111111111";
const source:SupplySourceDetail={source:{id,preparationId:id,collectionItemId:id,collectionRevision:1,source:{productKey:"product-a",publicationId:id,version:"1",kind:"own"}},product:{title:"原始商品",variants:[{sku:"source-sku",price:{amount:99},stock:42}]},images:[{id,url:"https://example.org/main.jpg",width:900,height:900}]};
const command={execute:state.execute} as unknown as SupplyCommandState;
function rules(mode:SupplyRules["merchant"]["application_type"]){return {merchant:{application_type:mode},rules:{application_type:mode,categories:[{category_id:123,product_type_id:456,category_name:"可发布类目",last_category:true,children:[]}],brands:[],warehouses:[],fill:{fill_in_standard_list:[],currency:"CNY",supplier_code_in_spu_dimension:false,picture_config_list:[]},attributes:{attribute_infos:[]}}} as unknown as SupplyRules}
beforeEach(()=>{state.rules.mockReset();state.inventory.mockReset().mockResolvedValue({assets:[{id:"approved-main",role:"main",url:"https://example.org/main.jpg",width:900,height:900}]});state.execute.mockReset();state.imagePanel.mockReset()});afterEach(cleanup);

it("passes the exact signed-64-bit source version into the image panel",async()=>{
 const version="9223372036854775807";
 state.rules.mockResolvedValue(rules("self_operated"));
 render(<SupplyTargetEditor scope={{userId:"actor",organizationId:"org"}} source={{...source,source:{...source.source,source:{...source.source.source,version}}}} storeId={id} command={command} disabled={false} saved={0}/>);
 await screen.findByLabelText("美国站售价 USD");
 expect(state.imagePanel.mock.lastCall?.[0].effectiveVersion).toBe(version);
});
for(const mode of ["self_operated","semi_managed","fully_managed"] as const)it(`renders the live ${mode} price and shelving fields without source inventory guesses`,async()=>{
 state.rules.mockResolvedValue(rules(mode));render(<SupplyTargetEditor scope={{userId:"actor",organizationId:"org"}} source={source} storeId={id} command={command} disabled={false} saved={0}/>);
 const price=await screen.findByLabelText(mode==="self_operated"?"美国站售价 USD":"供货价 CNY");expect(price).toHaveValue(mode==="self_operated"?null:"");
 expect(screen.queryByLabelText("库存数量")).not.toBeInTheDocument();
 if(mode!=="self_operated"){expect(screen.getByLabelText("上架方式")).toHaveValue("");await userEvent.selectOptions(screen.getByLabelText("上架方式"),"2");expect(screen.getByLabelText("期望上架时间（北京时间）")).toBeInTheDocument()}
 if(mode==="fully_managed")expect(screen.getByLabelText("上架要求")).toHaveValue("");
 await userEvent.click(screen.getByRole("button",{name:"保存并校验平台资料"}));
 await waitFor(()=>expect(state.execute).toHaveBeenCalled());const input=state.execute.mock.calls[0]![1];expect(input.sourceId).toBe(id);expect(input.draft.product.skc_list[0].sku_list[0].stock_info_list).toEqual([]);
 if(mode==="fully_managed")expect(input.draft.product.site_list).toBeUndefined();
});
it("approves only the explicit full source selection with original provenance",async()=>{
 state.rules.mockResolvedValue(rules("self_operated"));render(<SupplyTargetEditor scope={{userId:"actor",organizationId:"org"}} source={source} storeId={id} command={command} disabled={false} saved={0}/>);
 await screen.findByLabelText("美国站售价 USD");await userEvent.click(screen.getByLabelText("使用来源图 1"));await userEvent.selectOptions(screen.getByLabelText("来源图 1 用途"),"main");await userEvent.click(screen.getByRole("button",{name:"确认完整图片选择"}));
 expect(state.execute).toHaveBeenCalledWith("approve",{selection:{itemId:id,originalPublicationId:id,originalSnapshotVersion:1,effectiveCatalogVersion:1,targetPlatform:"shein"},images:[{id,role:"main"}],approved:[]});
});
it("requires an explicit stock quantity instead of supplying zero",async()=>{
 state.rules.mockResolvedValue(rules("self_operated"));render(<SupplyTargetEditor scope={{userId:"actor",organizationId:"org"}} source={source} storeId={id} command={command} disabled={false} saved={0}/>);
 await screen.findByLabelText("美国站售价 USD");await userEvent.click(screen.getByRole("button",{name:"添加库存记录"}));
 expect(screen.getByLabelText("库存数量")).toHaveValue(null);
 await userEvent.type(screen.getByLabelText("库存数量"),"0");expect(screen.getByLabelText("库存数量")).toHaveValue(0);
 await userEvent.clear(screen.getByLabelText("库存数量"));expect(screen.getByLabelText("库存数量")).toHaveValue(null);
});

it("collects one stock proof only when the current official field is shown",async()=>{
 const current=rules("semi_managed");current.rules.fill.fill_in_standard_list=[{field_key:"proof_of_stock",module:"skc",required:true,show:true}];state.rules.mockResolvedValue(current);
 render(<SupplyTargetEditor scope={{userId:"actor",organizationId:"org"}} source={source} storeId={id} command={command} disabled={false} saved={0}/>);
 await userEvent.click(await screen.findByRole("button",{name:"添加库存证明"}));
 await userEvent.type(screen.getByLabelText("库存证明文件名 1"),"stock.pdf");await userEvent.selectOptions(screen.getByLabelText("库存证明类型 1"),"2");await userEvent.type(screen.getByLabelText("库存证明链接 1"),"https://files.example.org/stock.pdf");
 expect(screen.queryByRole("button",{name:"添加库存证明"})).not.toBeInTheDocument();await userEvent.click(screen.getByRole("button",{name:"保存并校验平台资料"}));
 expect(state.execute.mock.calls[0]![1].draft.product.skc_list[0].proof_of_stock_list).toEqual([{file_name:"stock.pdf",type:"2",url:"https://files.example.org/stock.pdf"}]);
});

it("refreshes the existing official inventory after the image set is saved",async()=>{
 state.rules.mockResolvedValue(rules("self_operated"));
 state.inventory.mockResolvedValueOnce({assets:[{id:"old",role:"main",url:"https://example.org/main.jpg"}]}).mockResolvedValue({assets:[{id:"new",role:"main",url:"https://example.org/new.jpg"}]});
 render(<SupplyTargetEditor scope={{userId:"actor",organizationId:"org"}} source={source} storeId={id} command={command} disabled={false} saved={0}/>);
 await screen.findByLabelText("美国站售价 USD");await waitFor(()=>expect(state.inventory).toHaveBeenCalledOnce());
 await userEvent.click(screen.getByRole("button",{name:"添加图片位置"}));expect(screen.getByLabelText("图片 1").querySelector('option[value="old"]')).toBeInTheDocument();
 await userEvent.click(screen.getByRole("button",{name:"保存图片测试回调"}));
 await waitFor(()=>expect(state.inventory).toHaveBeenCalledTimes(2));
 expect(screen.getByLabelText("图片 1").querySelector('option[value="new"]')).toBeInTheDocument();
 expect(screen.getByLabelText("图片 1").querySelector('option[value="old"]')).toBeNull();
});
