import { afterEach, expect, it, vi } from "vitest";
import { MemberOperation } from "@/lib/api/members";
import { Pending, readPending, retainAdvancedReceipt, savePending } from "./member-pending";

const key='membership.pending:["actor","org"]';
const a:Pending={key:"ea0390e6-6fd0-4834-8e9c-277caf59c122",kind:"invite",input:{email:"original@example.test",firstName:"Original",lastName:"Member",role:"listingkit_viewer"}};
const b:Pending={...a,key:"fa0390e6-6fd0-4834-8e9c-277caf59c122",input:{...a.input,email:"other@example.test"}};
afterEach(()=>{sessionStorage.clear();vi.restoreAllMocks();});
it("preserves the current single command while adding and closing independent keys",()=>{
  sessionStorage.setItem(key,JSON.stringify(a));
  expect(readPending(sessionStorage.getItem(key))).toEqual([a]);
  savePending(sessionStorage,key,b);
  expect(readPending(sessionStorage.getItem(key))).toEqual([a,b]);
  savePending(sessionStorage,key,b.key);
  expect(readPending(sessionStorage.getItem(key))).toEqual([a]);
});
it("does not clear the original on failed storage or corrupt collection",()=>{
  const raw=JSON.stringify(a);sessionStorage.setItem(key,raw);
  vi.spyOn(Storage.prototype,"setItem").mockImplementation(()=>{throw new Error("quota");});
  expect(()=>savePending(sessionStorage,key,b)).toThrow();
  expect(sessionStorage.getItem(key)).toBe(raw);
  expect(()=>readPending("broken")).toThrow();
  expect(()=>readPending(JSON.stringify([a,a]))).toThrow();
});
it("keeps distinct scope records and rejects same-key intent replacement",()=>{
  savePending(sessionStorage,key,a);
  savePending(sessionStorage,'membership.pending:["actor","other"]',b);
  expect(()=>savePending(sessionStorage,key,{...b,key:a.key})).toThrow();
  expect(readPending(sessionStorage.getItem(key))).toEqual([a]);
});
it("does not let delayed ready/unknown snapshots downgrade known progress or a terminal receipt",()=>{
  const ready={id:a.key,userId:"actor",organizationId:"org",kind:"invite",step:"create_user",status:"pending",userEvidence:""} as MemberOperation;
  const unknown={...ready,status:"unknown"} as MemberOperation;
  const grant={...unknown,step:"create_authorization",userEvidence:"identity_verified"} as MemberOperation;
  const terminal={...grant,status:"acknowledged"} as MemberOperation;
  expect(retainAdvancedReceipt(unknown,ready)).toEqual(unknown);
  expect(retainAdvancedReceipt(grant,{...grant,status:"pending"})).toEqual(grant);
  expect(retainAdvancedReceipt(terminal,unknown)).toEqual(terminal);
  expect(retainAdvancedReceipt(unknown,terminal)).toEqual(terminal);
});
