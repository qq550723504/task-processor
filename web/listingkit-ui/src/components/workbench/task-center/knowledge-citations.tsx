import type { ProductKnowledge } from "@/lib/contracts/product-knowledge";
export function KnowledgeCitations({knowledge,humanEdited=false}:{knowledge?:ProductKnowledge;humanEdited?:boolean}) {
 if(!knowledge)return null;
 return <section aria-label="原 AI 建议的知识出处"><h3 className="font-semibold">知识出处</h3>
  <p>{humanEdited?"以下出处属于原 AI 建议，未重新证明人工编辑后的标题。":"企业知识仅补充表达依据；商品事实仍需当前 Product 证据支持。"}</p>
  {knowledge.status==="unavailable"?<p role="status">知识当前不可用。原建议的引用仍保留；名称与摘录已隐藏。</p>:knowledge.status==="uncited"?<p>本次采用企业知识上下文，模型未标注具体出处。</p>:knowledge.citations.map(citation=><details className="mt-3 rounded-lg border border-border bg-secondary p-3" key={citation.id}><summary>{citation.name} · {citation.state==="PARTIAL"?"部分解析可用":"可读"}</summary><p className="mt-2 break-all text-xs">版本标识：{citation.revisionId} · {citation.location}</p><p className="mt-2 whitespace-pre-wrap">{citation.excerpt}</p></details>)}
 </section>;
}
