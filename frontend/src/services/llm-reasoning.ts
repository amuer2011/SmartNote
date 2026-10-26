import catalog from '../../../internal/core/llm_reasoning_models.json'
import type {LLMReasoningCapability} from './api'

export const effortLabels:Record<string,string>={'':'模型默认',none:'不思考（none）',minimal:'极低（minimal）',low:'低（low）',medium:'中（medium）',high:'高（high）',xhigh:'极高（xhigh）',max:'最高（max）',off:'关闭思考（false）',on:'开启思考（true）'}
export function reasoningCapability(provider:'openai'|'ollama',model:string):LLMReasoningCapability {
 const name=model.trim().toLowerCase().replace(provider==='openai'?/^openai\//:/^ollama\//,'')
 const rule=catalog.find(rule=>{const candidate=rule.namespaced?name.slice(name.lastIndexOf('/')+1):name;return rule.provider===provider&&rule.models.some(id=>candidate===id||rule.dated&&candidate.startsWith(id+'-')&&/^\d{4}-\d{2}-\d{2}$/.test(candidate.slice(id.length+1))||rule.tags&&candidate.startsWith(id+':'))})
 if(rule)return {known:true,efforts:rule.efforts,message:rule.wire==='deepseek'?'使用 DeepSeek 参数格式；不思考发送 thinking.type=disabled。代理别名按 DeepSeek 接口适配，支持范围以网关为准。':rule.efforts.length?'按模型能力展示；模型默认不传思考参数。':'此模型不支持可调思考强度，使用模型默认。'}
 return {known:false,efforts:provider==='ollama'?['off','on','low','medium','high']:['none','minimal','low','medium','high','xhigh','max'],message:'未能确认此模型的支持范围。以下为兼容档位，请以供应商文档为准；不确定时选择模型默认。'}
}
export function normalizeReasoning(value:string,capability:LLMReasoningCapability){return capability.efforts.includes(value)?value:''}
