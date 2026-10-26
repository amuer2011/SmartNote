<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,ref,watch} from 'vue'
import {Plus,Trash2,RefreshCw,Server,Check} from 'lucide-vue-next'
import {api,type LLMProvider,type LLMProviders} from '../services/api'
import ReasoningSelect from './ReasoningSelect.vue'
const emit=defineEmits<{saved:[]}>()
const state=ref<LLMProviders>(),selectedID=ref(''),secrets=ref<Record<string,string>>({}),clears=ref<Record<string,boolean>>({})
const baseline=ref(''),busy=ref(false),error=ref(''),message=ref(''),filter=ref(''),manual=ref('')
const reasoningPending=ref(false)
let alive=true,sequence=0
const fetched=new Set<string>()
const selected=computed(()=>state.value?.providers.find(p=>p.id===selectedID.value))
const dirty=computed(()=>!!state.value&&(JSON.stringify(state.value)!==baseline.value||Object.values(secrets.value).some(Boolean)||Object.values(clears.value).some(Boolean)))
const visibleModels=computed(()=>selected.value?.models.filter(m=>m.toLowerCase().includes(filter.value.toLowerCase()))||[])
function apply(value:LLMProviders){
 state.value={...value,providers:(value.providers||[]).map(p=>({...p,models:p.models||[],selectedModels:p.selectedModels||[],reasoningEffort:p.reasoningEffort||''}))}
 baseline.value=JSON.stringify(state.value)
 if(!state.value.providers.some(p=>p.id===selectedID.value))selectedID.value=state.value.defaultId||state.value.providers[0]?.id||''
 secrets.value={};clears.value={}
}
async function load(){const token=++sequence;busy.value=true;error.value='';message.value='';try{const value=await api.GetLLMProviders();if(alive&&token===sequence)apply(value)}catch(e){if(alive&&token===sequence)error.value=String(e)}finally{if(alive&&token===sequence)busy.value=false}}
function add(){if(!state.value)return;const id=crypto.randomUUID();state.value.providers.push({id,name:'新供应商',provider:'openai',endpoint:'https://api.openai.com/v1',model:'',reasoningEffort:'',enabled:state.value.enabled,hasSecret:false,revision:'',models:[],selectedModels:[]});selectedID.value=id;filter.value='';manual.value=''}
function remove(){if(!state.value||!selected.value)return;const id=selected.value.id;state.value.providers=state.value.providers.filter(p=>p.id!==id);delete secrets.value[id];delete clears.value[id];if(state.value.defaultId===id)state.value.defaultId=state.value.providers[0]?.id||'';selectedID.value=state.value.providers[0]?.id||'';if(!state.value.providers.length)state.value.enabled=false;message.value='已从草稿移除，保存后生效。'}
function changeType(){const p=selected.value;if(!p)return;p.reasoningEffort='';p.models=[];p.selectedModels=[];p.model='';p.endpoint=p.provider==='ollama'?'http://127.0.0.1:11434':'https://api.openai.com/v1'}
function validation(p:LLMProvider,requireModel=true){
 if(!p.name.trim())return '请填写供应商名称。'
 let url:URL;try{url=new URL(p.endpoint.trim())}catch{return '请输入有效的 API 地址。'}
 if(url.username||url.password||url.search||url.hash)return 'API 地址不能带账号、查询参数或片段。'
 const local=['localhost','127.0.0.1','[::1]','::1'].includes(url.hostname)
 if(!['https:','http:'].includes(url.protocol)||url.protocol==='http:'&&!local)return '远程服务必须使用 HTTPS，本机回环地址可用 HTTP。'
 if(p.provider==='ollama'&&!local)return 'Ollama 仅支持本机回环地址。'
 if(requireModel&&!p.model.trim())return '请拉取或手动添加模型，并选择默认模型。'
 return ''
}
async function fetchModels(){
 const p=selected.value;if(!p||busy.value)return
 const problem=validation(p,false);if(problem){error.value=problem;return}
 const token=sequence;busy.value=true;error.value='';message.value='正在获取供应商模型…'
 try{const models=await api.ListLLMModels({...p},secrets.value[p.id]||'',!!clears.value[p.id]);if(!alive||token!==sequence)return;p.models=[...new Set([...models,...p.selectedModels,...(p.model?[p.model]:[])])].sort();if(!p.model)p.model=models[0]||'';message.value=`已获取 ${models.length} 个模型；不勾选则全部可用，保存后生效。`}
 catch(e){if(alive&&token===sequence){error.value=String(e);message.value=''}}finally{if(alive&&token===sequence)busy.value=false}
}
function addModels(){const p=selected.value;if(!p)return;const names=manual.value.split(/[,，;；\n]/).map(s=>s.trim()).filter(Boolean);if(names.some(m=>m.length>200)){error.value='模型名称过长。';return}p.models=[...new Set([...p.models,...names])].sort();if(!p.model)p.model=names[0]||'';manual.value=''}
function toggleModel(model:string){const p=selected.value;if(!p)return;p.selectedModels=p.selectedModels.includes(model)?p.selectedModels.filter(m=>m!==model):[...p.selectedModels,model];if(p.selectedModels.length&&!p.selectedModels.includes(p.model))p.model=p.selectedModels[0]}
async function save(){
 if(!state.value||busy.value||reasoningPending.value)return
 for(const p of state.value.providers){const problem=validation(p);if(problem){selectedID.value=p.id;error.value=problem;return}}
 const token=sequence;busy.value=true;error.value='';message.value=''
 try{const out=await api.SaveLLMProviders(JSON.parse(JSON.stringify(state.value)),{...secrets.value},Object.keys(clears.value).filter(id=>clears.value[id]));if(!alive||token!==sequence)return;apply(out);message.value=out.enabled?'大模型配置已保存并启用。':'大模型配置已保存，当前未启用。';emit('saved')}
 catch(e){if(alive&&token===sequence)error.value=String(e)}finally{if(alive&&token===sequence)busy.value=false}
}
async function test(){
 const p=selected.value;if(!p||busy.value||reasoningPending.value)return
 const problem=validation(p);if(problem){error.value=problem;return}
 const token=sequence;busy.value=true;error.value='';message.value='正在发送测试消息…'
 try{const text=await api.TestLLMProvider({...p},secrets.value[p.id]||'',!!clears.value[p.id]);if(alive&&token===sequence)message.value=text}catch(e){if(alive&&token===sequence){error.value=String(e);message.value=''}}finally{if(alive&&token===sequence)busy.value=false}
}
async function autoFetch(){const p=selected.value;if(!p||busy.value||!p.revision||(!p.hasSecret&&p.provider!=='ollama'))return;const key=p.id+':'+p.endpoint+':'+p.revision;if(fetched.has(key))return;fetched.add(key);await fetchModels()}
watch(selectedID,()=>void autoFetch())
onMounted(async()=>{await load();await autoFetch()});onBeforeUnmount(()=>{alive=false;sequence++})
</script>
<template><section class="llm-settings" aria-label="大模型配置">
 <p class="llm-intro">配置多个供应商，在右侧 AI 助手中切换模型、对话、检索和修改笔记。大模型默认关闭，本地笔记、语义检索和任务不受影响。</p>
 <template v-if="state">
 <label class="setting-row"><span><strong>启用大模型</strong><p>仅主动发送时调用。测试连接和拉取模型可独立使用。</p></span><input v-model="state.enabled" type="checkbox" aria-label="启用大模型" :disabled="busy"/></label>
 <div class="provider-heading"><strong>供应商 <span>{{state.providers.length}} / 20</span></strong><button class="text-button" :disabled="busy||state.providers.length>=20" @click="add"><Plus :size="14"/>添加供应商</button></div>
 <div class="provider-tabs"><button v-for="p in state.providers" :key="p.id" :class="{active:p.id===selectedID}" :disabled="busy" @click="selectedID=p.id;filter='';manual=''"><Server :size="14"/><span>{{p.name||'未命名'}}</span><Check v-if="state.defaultId===p.id" :size="13"/></button></div>
 <template v-if="selected">
 <div class="provider-title"><label class="field-label" for="provider-name">供应商名称</label><button class="icon-button danger-hover" aria-label="移除当前供应商" title="移除供应商（保存后生效）" :disabled="busy" @click="remove"><Trash2 :size="15"/></button></div>
 <input id="provider-name" v-model="selected.name" class="date-input" maxlength="80" :disabled="busy"/>
 <label class="default-provider"><input type="radio" name="default-provider" :checked="state.defaultId===selected.id" :disabled="busy" @change="state.defaultId=selected.id"/>设为默认供应商</label>
 <label class="field-label" for="llm-provider">服务类型</label><select id="llm-provider" v-model="selected.provider" class="date-input" :disabled="busy" @change="changeType"><option value="openai">OpenAI 兼容接口</option><option value="ollama">Ollama（本机）</option></select>
 <label class="field-label" for="llm-endpoint">API 地址</label><input id="llm-endpoint" v-model="selected.endpoint" class="date-input" type="url" autocomplete="off" spellcheck="false" :disabled="busy"/>
 <p class="field-help">{{selected.provider==='openai'?'例如 https://api.openai.com/v1；仅填域名时自动补齐 /v1。支持完整的 /chat/completions 地址。':'例如 http://127.0.0.1:11434；仅允许本机回环地址。'}}</p>
 <label class="field-label" for="llm-secret">API 密钥</label><input id="llm-secret" v-model="secrets[selected.id]" class="date-input" type="password" autocomplete="new-password" :placeholder="selected.hasSecret?'已保存密钥；留空保持不变':'本机 Ollama 通常无需密钥'" :disabled="busy||clears[selected.id]"/>
 <p class="field-help">密钥保存在操作系统安全凭据存储中。更换地址或服务类型后请重新填写，旧密钥不会转发到新服务。</p>
 <label v-if="selected.hasSecret" class="default-provider"><input v-model="clears[selected.id]" type="checkbox" :disabled="busy"/>清除该供应商的已存密钥</label>
 <div class="model-heading"><strong>可用模型</strong><button class="secondary-button" :disabled="busy" @click="fetchModels"><RefreshCw :size="13"/>拉取模型</button></div>
 <p class="field-help">从供应商拉取模型后可多选；不勾选则全部可用。服务未提供列表时，可手动添加模型。</p>
 <input v-if="selected.models.length>8" v-model="filter" class="date-input" aria-label="筛选模型" placeholder="筛选模型…"/>
 <div v-if="selected.models.length" class="model-list" aria-label="可用模型列表"><label v-for="model in visibleModels" :key="model"><input type="checkbox" :checked="selected.selectedModels.includes(model)" :disabled="busy" @change="toggleModel(model)"/><span>{{model}}</span></label></div>
 <div class="manual-model"><input v-model="manual" class="date-input" aria-label="手动添加模型" placeholder="输入模型 ID，多个用逗号隔开" :disabled="busy" @keydown.enter.prevent="addModels"/><button class="secondary-button" :disabled="busy||!manual.trim()" @click="addModels">添加</button></div>
 <p class="field-help">{{selected.selectedModels.length?`已启用 ${selected.selectedModels.length} 个模型`:`全部 ${selected.models.length} 个模型可用`}}。</p>
 <label class="field-label" for="llm-model">默认模型</label><select id="llm-model" v-model="selected.model" class="date-input" :disabled="busy||!selected.models.length"><option v-if="!selected.model" value="">请先拉取或添加模型</option><option v-for="model in selected.selectedModels.length?selected.selectedModels:selected.models" :key="model" :value="model">{{model}}</option></select>
 <ReasoningSelect presentation="slider" v-model="selected.reasoningEffort" :provider="selected.provider" :model="selected.model" :connection="selected" :secret="secrets[selected.id]||''" :clear-secret="!!clears[selected.id]" :disabled="busy" help @pending="reasoningPending=$event"/>
 <p class="field-help">切换模型后，不支持的档位自动恢复为模型默认。强度越高通常耗时和用量越多；对话栏可单独调整。</p>
 </template>
 <div v-else class="empty-provider">还没有供应商，点击「添加供应商」开始配置。</div>
 <div class="actions"><button class="primary-button" :disabled="busy||reasoningPending" @click="save">保存大模型配置</button><button class="secondary-button" :disabled="busy||!selected||reasoningPending" @click="test">测试连接</button><button class="text-button" :disabled="busy" @click="load">重新加载</button></div>
 <p class="field-help">测试使用当前草稿，不保存、不启用，也不发送笔记。{{dirty?'有未保存的修改。':''}}</p>
 <div class="usage"><strong>使用 AI 助手</strong><p>打开笔记，点击右侧「AI 聊天」页签；可选中文字后提出要求，也可直接询问、检索或汇总工作区。修改建议须核对差异并确认才会保存。切换「大纲」页签即可返回笔记大纲。</p></div>
 </template>
 <p v-if="error" class="bookmark-error" role="alert">{{error}}</p><p v-if="message" class="test-result" role="status">{{message}}</p>
</section></template>
<style scoped>
.llm-settings{max-width:760px;margin-bottom:28px}.llm-intro{font-size:.86rem;color:var(--muted);line-height:1.9}.date-input{width:100%;min-width:0;background:var(--bg);color:var(--text);border-color:var(--line)}input[type=checkbox],input[type=radio]{accent-color:var(--accent)}.provider-heading,.model-heading,.provider-title{display:flex;align-items:center;justify-content:space-between;gap:8px;margin-top:24px;font-size:.86rem}.provider-heading strong span{font-size:.73rem;color:var(--muted);font-weight:400;margin-left:6px}.provider-tabs{display:flex;gap:7px;flex-wrap:wrap;margin-top:14px}.provider-tabs button{display:flex;gap:6px;align-items:center;max-width:100%;padding:8px 10px;border:1px solid var(--line);border-radius:7px;color:var(--muted);font-size:.8rem}.provider-tabs button span{overflow:hidden;text-overflow:ellipsis}.provider-tabs button.active{border-color:var(--accent);background:var(--accent-bg);color:var(--accent)}.provider-title .field-label{margin:0 0 8px}.default-provider{display:flex;gap:7px;align-items:center;margin-top:12px;color:var(--muted);font-size:.78rem}.model-list{display:flex;flex-direction:column;max-height:220px;overflow:auto;border:1px solid var(--line);border-radius:7px;margin:10px 0}.model-list label{display:flex;align-items:center;gap:9px;font-size:.8rem;padding:9px 12px;overflow-wrap:anywhere}.model-list label:hover{background:var(--hover)}.model-list input{flex-shrink:0}.manual-model{display:flex;gap:8px;margin-top:12px}.manual-model input{flex:1}.actions{display:flex;align-items:center;flex-wrap:wrap;gap:10px;margin-top:24px}.test-result{color:var(--muted);font-size:.82rem;overflow-wrap:anywhere;line-height:1.9}.usage{border:1px solid var(--line);border-radius:8px;padding:16px;margin-top:20px;font-size:.83rem}.usage p{font-size:.78rem;color:var(--muted);line-height:1.9;margin:10px 0 0}.empty-provider{padding:24px;color:var(--muted);font-size:.85rem}button:disabled{opacity:.45;cursor:default}
</style>
