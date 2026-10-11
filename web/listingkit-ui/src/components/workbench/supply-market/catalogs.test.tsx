import {cleanup, render, screen, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {afterEach, beforeEach, expect, it, vi} from "vitest";
import {useMarketCommands} from "./shared";
import {SupplyCatalogsPage} from "./catalogs";

const context=vi.hoisted(()=>({user:{id:"member-a"},effectiveOrganization:{id:"org-a"},isLoading:false,isSwitching:false,error:null,blockingError:null}));
const commands=vi.hoisted(()=>({execute:vi.fn(),saved:null as {recordId:string}|null,notice:null,locked:false}));
vi.mock("@/components/providers/workbench-context-provider",()=>({useWorkbenchContext:()=>context}));
vi.mock("./shared",async importOriginal=>({...await importOriginal<typeof import("./shared")>(),useMarketCommands:vi.fn(()=>commands)}));
afterEach(cleanup);
beforeEach(()=>{vi.clearAllMocks();context.isLoading=false;context.effectiveOrganization.id="org-a";commands.saved=null;commands.locked=false});
function page(view?:"selection"|"apply",acquisitionAvailable=false,sdsAvailable=false){return render(<SupplyCatalogsPage view={view} acquisitionAvailable={acquisitionAvailable} sdsAvailable={sdsAvailable}/>)}
it("uses the catalog parent as a two-entry overview without mounting a write form",()=>{
  page();
  expect(screen.getByRole("heading",{name:"货盘集成",level:1})).toBeVisible();
  expect(screen.getByRole("link",{name:"进入货盘选品"})).toHaveAttribute("href","/workbench/supply/catalogs/selection");
  expect(screen.getByRole("link",{name:"申请对接货盘"})).toHaveAttribute("href","/workbench/supply/catalogs/apply");
  expect(screen.queryByRole("button",{name:"提交对接申请"})).not.toBeInTheDocument();
  expect(useMarketCommands).not.toHaveBeenCalled();
});
it.each([[false,false],[true,false],[false,true],[true,true]])("keeps selection links dependent on actual acquisition=%s / SDS=%s",(acquisitionAvailable,sdsAvailable)=>{
  page("selection",acquisitionAvailable,sdsAvailable);
  expect(screen.getByRole("heading",{name:"货盘选品",level:1})).toBeVisible();
  expect(within(screen.getByRole("navigation",{name:"面包屑"})).getByText("货盘选品")).toBeVisible();
  if(acquisitionAvailable)expect(screen.getByRole("link",{name:"进入 1688 选品与采集"})).toHaveAttribute("href","/workbench/supply/acquisition");
  else expect(screen.queryByRole("link",{name:"进入 1688 选品与采集"})).not.toBeInTheDocument();
  if(sdsAvailable)expect(screen.getByRole("link",{name:"查看模板与定制成品"})).toHaveAttribute("href","/workbench/supply/catalogs/sds");
  else expect(screen.getByText("当前实例未配置 SDS 定制能力")).toBeVisible();
  expect(screen.queryByRole("button",{name:"提交对接申请"})).not.toBeInTheDocument();
  expect(useMarketCommands).not.toHaveBeenCalled();
});
it("keeps the original member-scoped connection command and form fields on the apply leaf",async()=>{
  page("apply");
  expect(screen.getByRole("heading",{name:"申请对接",level:1})).toBeVisible();
  const breadcrumb=within(screen.getByRole("navigation",{name:"面包屑"}));
  expect(breadcrumb.getByRole("link",{name:"货盘集成"})).toHaveAttribute("href","/workbench/supply/catalogs");
  expect(breadcrumb.getByText("申请对接")).toBeVisible();
  expect(useMarketCommands).toHaveBeenCalledWith({userId:"member-a",organizationId:"org-a"});
  const user=userEvent.setup();
  await user.type(screen.getByLabelText("货盘名称"),"Fixture Catalog");
  await user.type(screen.getByLabelText("货盘网站（HTTPS）"),"https://catalog.example.test");
  await user.type(screen.getByLabelText("联系人"),"Fixture contact");
  await user.type(screen.getByLabelText("联系电话"),"12345");
  await user.type(screen.getByLabelText("供应品类"),"Home");
  await user.click(screen.getByRole("button",{name:"提交对接申请"}));
  expect(commands.execute).toHaveBeenCalledExactlyOnceWith({action:"submit_connection",connection:{name:"Fixture Catalog",website:"https://catalog.example.test",contact:"Fixture contact",telephone:"12345",categories:"Home"}});
});
it("keeps the existing pending lock and saved record link",()=>{
  commands.locked=true; commands.saved={recordId:"11111111-1111-4111-8111-111111111111"};
  page("apply");
  expect(screen.getByRole("button",{name:"提交对接申请"})).toBeDisabled();
  expect(screen.getByRole("link",{name:"查看对接进展"})).toHaveAttribute("href","/workbench/supply/applications/11111111-1111-4111-8111-111111111111");
});
it("waits for the existing identity boundary before mounting the application consumer",()=>{
  context.isLoading=true; page("apply");
  expect(screen.getByText("正在确认当前身份")).toBeVisible();
  expect(useMarketCommands).not.toHaveBeenCalled();
});
