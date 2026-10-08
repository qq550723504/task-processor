"use client";
import {useState} from "react";
import {useQuery} from "@tanstack/react-query";
import {QRCodeSVG} from "qrcode.react";
import {Button} from "@/components/ui/button";
import {ResourceDialog} from "@/components/workbench/resources/resource-dialog";
import {ecoRequest,ecoMerchantSchema,ecoDocumentTypes,EcoservicesError,type EcoScope,type EcoApplication,type EcoIdentity,type EcoMerchantInput} from "@/lib/api/ecoservices";
import {ReadFailure,useEcoCommands,yuan,date} from "./shared";
import {FileUpload} from "./files";

const names=["大陆身份证","境外护照","香港来往内地通行证","澳门来往内地通行证","台湾来往大陆通行证","外国人居留证","港澳居民居住证","台湾居民居住证"];
const states:Record<string,string>={PREPARING:"资料准备中",CHECKING:"资料校验中",ACCOUNT_NEED_VERIFY:"需完成账户验证",AUDITING:"微信审核中",REJECTED:"微信已驳回",NEED_SIGN:"待商户管理员签约",FINISH:"微信已开通",FROZEN:"微信申请已冻结",CANCELED:"微信申请已取消"};
function emptyIdentity():EcoIdentity{return {type:ecoDocumentTypes[0],name:"",number:"",address:"",frontFileId:"",backFileId:"",validFrom:"",validUntil:"长期"}}
export function MerchantOnboarding({scope,application}:{scope:EcoScope;application:EcoApplication}){
 const [form,setForm]=useState(false),commands=useEcoCommands(scope);
 const query=useQuery({queryKey:["ecoservices",scope.userId,scope.organizationId,"merchant",application.id],queryFn:async({signal})=>{try{return await ecoRequest(scope,"applications/"+application.id+"/merchant",ecoMerchantSchema,{signal})}catch(e){if(e instanceof EcoservicesError&&e.code==="ECOSERVICES_NOT_FOUND")return null;throw e}},refetchInterval:30000});
 const status=query.data;
 return <section className="eco-detail">{commands.notice}<h3>微信商户准入</h3><p>当前按企业主体、对公账户进件，法人作为管理员完成微信验证与签约。业务资料以平台批准的原申请为准。</p>
 {query.isPending?<p>正在读取原申请…</p>:query.error?<ReadFailure error={query.error} retry={()=>void query.refetch()}/>:status?<>
  <p>{states[status.state]} · {date(status.updatedAt)}</p>{status.verificationPending?<p>当前渠道结果尚未核实，本次结果未确认时不能提交不同资料或签约。</p>:null}{status.state==="PREPARING"&&(status.revisionVersion==="1"||!status.verificationPending)?<Button disabled={commands.locked} onClick={()=>void commands.execute({path:"applications/"+application.id+"/merchant/resume",key:crypto.randomUUID(),body:"{}",version:application.version,output:"merchant"}).catch(()=>undefined)}>继续原商户申请</Button>:null}{status.reason?<p role="status">渠道意见：{status.reason}</p>:null}
  {status.legalValidationUrl?<div><h4>法人账户验证</h4><QRCodeSVG value={status.legalValidationUrl} size={180}/><p>请商户法人用微信扫描，按渠道指引验证。</p></div>:null}
  {status.signUrl?<div><h4>原申请签约</h4><QRCodeSVG value={status.signUrl} size={180}/><p>请本申请的商户管理员用本人实名微信扫描并确认协议。签约后点击刷新。</p></div>:null}
  {status.bank?<div><h4>账户验证汇款指引</h4><p>付款户名：{status.bank.accountName}{status.bank.accountNumber?" · 账号："+status.bank.accountNumber:""}</p><p>验证金额：{yuan(status.bank.payAmountMinor)}</p><p>收款户名：{status.bank.destinationName} · 账号：{status.bank.destinationNumber}</p><p>开户行：{status.bank.destinationBank} · {status.bank.city}</p><p>汇款备注：{status.bank.remark}</p><p>截止时间：{status.bank.deadline}</p><p>请按渠道要求操作，该验证款项不属于服务交易付款。</p></div>:null}
  {status.state==="REJECTED"||status.state==="CANCELED"?<p>当前申请尚未取得渠道资格。更正沿原申请继续；更换营业执照须重新由平台审核并确认协议。</p>:null}
  {status.canCorrect&&!status.verificationPending&&application.state==="APPROVED"&&application.agreementAccepted?<Button disabled={commands.locked} onClick={()=>setForm(true)}>更正微信商户资料</Button>:null}
  <Button variant="outline" onClick={()=>void query.refetch()} disabled={query.isFetching}>刷新原渠道状态</Button>
 </>:<Button disabled={!application.agreementAccepted||application.state!=="APPROVED"} onClick={()=>setForm(true)}>提交本企业商户资料</Button>}
 {form?<MerchantForm scope={scope} application={application} revisionVersion={status?.revisionVersion??"0"} onClose={()=>{setForm(false);void query.refetch()}}/>:null}</section>
}
function MerchantForm({scope,application,revisionVersion,onClose}:{scope:EcoScope;application:EcoApplication;revisionVersion:string;onClose:()=>void}){
 const commands=useEcoCommands(scope),[files,setFiles]=useState(application.fileIds),[details,setDetails]=useState<EcoMerchantInput>({expectedRevisionVersion:revisionVersion,licenseFileId:"",legal:emptyIdentity(),soleLegalBeneficiary:true,beneficiaries:[],contactMobile:"",accountBank:"",accountNumber:"",bankBranchName:"",merchantShortName:"",storeName:"",storeUrl:""}),[confirm,setConfirm]=useState(false);
 const update=<K extends keyof EcoMerchantInput>(key:K,value:EcoMerchantInput[K])=>setDetails(v=>({...v,[key]:value}));
 return <ResourceDialog title={revisionVersion==="0"?"提交企业微信商户资料":"更正原微信商户资料"} onClose={onClose} locked={commands.locked}><div className="eco-detail"><form autoComplete="off" onSubmit={e=>{e.preventDefault();void commands.execute({path:"applications/"+application.id+"/merchant",key:crypto.randomUUID(),body:JSON.stringify(details),version:application.version,output:"merchant"}).then(onClose).catch(()=>undefined)}}><fieldset disabled={commands.locked}>
 <p>批准的企业：{application.companyName} · {application.registrationNumber}。证件照片须完整、清晰、每张不超过2 MiB，仅支持 JPG、PNG。</p>
 <label>{revisionVersion==="0"?"已审核的营业执照":"营业执照（更换后须重新审核）"}<ProofSelect ids={revisionVersion==="0"?application.fileIds:files} value={details.licenseFileId} onChange={v=>update("licenseFileId",v)}/></label>
 <h3>法人资料</h3><DocumentFields value={details.legal} onChange={v=>update("legal",v)} files={files}/>
 <label><input type="checkbox" checked={details.soleLegalBeneficiary} onChange={e=>setDetails(v=>({...v,soleLegalBeneficiary:e.target.checked,beneficiaries:e.target.checked?[]:[emptyIdentity()]}))}/>法人是企业唯一最终受益所有人</label>
 {!details.soleLegalBeneficiary?<><p>请填写完整的最终受益所有人名单，包含法人时也需填写，最多4人。</p>{details.beneficiaries.map((b,i)=><div key={i}><h4>受益所有人 {i+1}</h4><DocumentFields value={b} files={files} beneficiary onChange={value=>update("beneficiaries",details.beneficiaries.map((v,n)=>n===i?value:v))}/><Button type="button" variant="outline" disabled={details.beneficiaries.length===1} onClick={()=>update("beneficiaries",details.beneficiaries.filter((_,n)=>n!==i))}>移除此人</Button></div>)}<Button type="button" variant="outline" disabled={details.beneficiaries.length===4} onClick={()=>update("beneficiaries",[...details.beneficiaries,emptyIdentity()])}>添加受益所有人</Button></>:null}
 <FileUpload commands={commands} path={"applications/"+application.id+"/files"} ids={files} onFiles={setFiles} maxBytes={2*1024*1024} maxFiles={21} accept=".png,.jpg,.jpeg"/>
 <h3>联系人和对公账户</h3><p>本申请由上述法人担任商户管理员。</p><label>法人手机号<input required type="tel" maxLength={32} value={details.contactMobile} onChange={e=>update("contactMobile",e.target.value)}/></label><label>开户银行名称<input required maxLength={128} value={details.accountBank} onChange={e=>update("accountBank",e.target.value)}/></label><label>对公银行账号<input required inputMode="numeric" maxLength={64} value={details.accountNumber} onChange={e=>update("accountNumber",e.target.value)}/></label><label>开户行全称（渠道要求支行时填写）<input maxLength={128} value={details.bankBranchName} onChange={e=>update("bankBranchName",e.target.value)}/></label>
 <h3>经营场景</h3><label>商户简称<input required maxLength={64} value={details.merchantShortName} onChange={e=>update("merchantShortName",e.target.value)}/></label><label>店铺或服务网站名称<input required maxLength={128} value={details.storeName} onChange={e=>update("storeName",e.target.value)}/></label><label>可直接访问的主页链接<input required type="url" maxLength={256} value={details.storeUrl} onChange={e=>update("storeUrl",e.target.value)}/></label>
 <label><input required type="checkbox" checked={confirm} onChange={e=>setConfirm(e.target.checked)}/>确认资料属于当前企业，同意提交给微信用于商户审核；后续协议由商户管理员本人签署</label><Button type="submit" disabled={!confirm}>提交原商户申请</Button>
 </fieldset></form>{commands.notice}</div></ResourceDialog>
}
function ProofSelect({ids,value,onChange,required=true}:{ids:string[];value:string;onChange:(v:string)=>void;required?:boolean}){return <select required={required} value={value} onChange={e=>onChange(e.target.value)}><option value="">请选择已保存材料</option>{ids.map((id,i)=><option key={id} value={id}>材料 {i+1} · {id.slice(0,8)}</option>)}</select>}
function DocumentFields({value,onChange,files,beneficiary=false}:{value:EcoIdentity;onChange:(v:EcoIdentity)=>void;files:string[];beneficiary?:boolean}){
 const update=<K extends keyof EcoIdentity>(key:K,v:EcoIdentity[K])=>onChange({...value,[key]:v});return <div className="eco-form"><label>证件类型<select value={value.type} onChange={e=>update("type",e.target.value as EcoIdentity["type"])}>{ecoDocumentTypes.map((type,i)=><option key={type} value={type}>{names[i]}</option>)}</select></label><label>姓名<input required maxLength={128} value={value.name} onChange={e=>update("name",e.target.value)}/></label><label>证件号码<input required maxLength={64} value={value.number} onChange={e=>update("number",e.target.value)}/></label>{beneficiary?<label>证件居住地址<input required maxLength={512} value={value.address} onChange={e=>update("address",e.target.value)}/></label>:null}<label>证件正面<ProofSelect ids={files} value={value.frontFileId} onChange={v=>update("frontFileId",v)}/></label><label>证件反面（护照可留空）<ProofSelect ids={files} value={value.backFileId} required={value.type!=="IDENTIFICATION_TYPE_OVERSEA_PASSPORT"} onChange={v=>update("backFileId",v)}/></label><label>有效期开始<input required type="date" value={value.validFrom} onChange={e=>update("validFrom",e.target.value)}/></label><label>有效期结束（YYYY-MM-DD或长期）<input required placeholder="长期" maxLength={10} value={value.validUntil} onChange={e=>update("validUntil",e.target.value)}/></label></div>
}
