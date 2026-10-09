import { cleanup,fireEvent,render,screen } from "@testing-library/react";
import { afterEach,beforeEach,expect,it,vi } from "vitest";
import { ImageConfigurationDialog } from "./image-configuration-dialog";
import { type ImageSetTemplate } from "@/lib/contracts/image-set-configuration";

beforeEach(()=>{
  Object.defineProperty(HTMLDialogElement.prototype,"showModal",{configurable:true,value:function(this:HTMLDialogElement){this.open=true}});
  Object.defineProperty(HTMLDialogElement.prototype,"close",{configurable:true,value:function(this:HTMLDialogElement){this.open=false}});
});
afterEach(cleanup);
const initial:ImageSetTemplate={schema:"image-config-v1",mode:"standard",shareOriginals:false,background:"白色背景",language:"en",carousel:[{id:"identity",purpose:"product_identity"}],detail:[]};
it("configures both task groups while sharing only originals",()=>{
  const saved=vi.fn();render(<ImageConfigurationDialog initial={initial} onSave={saved} onClose={vi.fn()}/>);
  fireEvent.click(screen.getByRole("button",{name:/共用原始素材/}));
  fireEvent.click(screen.getByRole("button",{name:/详情图 配置详情图生成任务/}));
  fireEvent.click(screen.getByRole("checkbox",{name:/商品全貌/}));
  fireEvent.click(screen.getByRole("checkbox",{name:/使用步骤/}));
  fireEvent.click(screen.getByRole("button",{name:"确认选择"}));
  expect(saved).toHaveBeenCalledWith(expect.objectContaining({shareOriginals:true,carousel:initial.carousel,detail:[{id:"detail-product_overview",purpose:"product_overview"},{id:"detail-usage_steps",purpose:"usage_steps"}]}));
  expect(initial.detail).toHaveLength(0);
});
it("keeps custom instructions separate and blocks empty or oversized plans",()=>{
  const saved=vi.fn();render(<ImageConfigurationDialog initial={initial} onSave={saved} onClose={vi.fn()}/>);
  fireEvent.click(screen.getByRole("button",{name:/自定义指令/}));
  fireEvent.click(screen.getByRole("button",{name:"新增主图任务"}));
  fireEvent.click(screen.getByRole("button",{name:"确认选择"}));
  expect(saved).not.toHaveBeenCalled();
  fireEvent.change(screen.getByLabelText("主图任务 1 指令"),{target:{value:"仅展示原商品的真实细节"}});
  fireEvent.click(screen.getByRole("button",{name:"确认选择"}));
  expect(saved).toHaveBeenCalledWith(expect.objectContaining({mode:"custom",carousel:[expect.objectContaining({purpose:"custom",brief:"仅展示原商品的真实细节"})],detail:[]}));
});
