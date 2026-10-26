<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,onMounted,ref,watch} from 'vue'
import {Sparkles,X,Settings2,Plus,Send,Square,FileText,Check,Copy,Search,History,Brain,Maximize2,PanelRightClose,Paperclip} from 'lucide-vue-next'
import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import {admonitions} from '../services/admonitions'
import {vBlockCopy} from '../services/markdown-copy'
import '../markdown-copy.css'
import {api,type Note,type LLMProvider,type LLMProviders,type LLMSource,type LLMChatResult,type LLMEditProposal,type AssistantConversation,type AssistantAttachment} from '../services/api'
import ChatModelPicker from './ChatModelPicker.vue'
import FloatingWindow from './FloatingWindow.vue'
import AssistantLibrary from './AssistantLibrary.vue'
import ChatAttachments from './ChatAttachments.vue'
import {readChatAttachment,longTextThreshold,attachmentSize} from '../services/chat-attachments'
import {diffLines} from '../services/llm-diff'
const props=defineProps<{visible:boolean;panelHidden:boolean;workspaceId:string;note:Note|null;revision:number;selection:string;prepareContext:()=>Promise<{noteId:string;hash:string}>;applyEdit:(p:LLMEditProposal)=>Promise<void>}>()
const emit=defineEmits<{close:[];settings:[];source:[source:LLMSource];clearSelection:[]}>()
const floating=ref(false),floatingTarget=ref<HTMLElement>(),chatHost=ref<HTMLElement>(),inputHeight=ref(90)
let inputObserver:ResizeObserver|undefined,inputGesture:{y:number;height:number}|undefined
try{const saved=Number(localStorage.getItem('smartnote-ai-input-height'));if(Number.isFinite(saved)&&saved>=76)inputHeight.value=saved}catch{}
function fitInput(){const max=Math.max(76,(chatHost.value?.clientHeight||window.innerHeight)-270);inputHeight.value=Math.max(76,Math.min(max,inputHeight.value))}
function saveInput(){try{localStorage.setItem('smartnote-ai-input-height',String(inputHeight.value))}catch{}}
function resizeInput(event:PointerEvent){if(event.button!==0)return;event.preventDefault();inputGesture={y:event.clientY,height:inputHeight.value};(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)}
function moveInput(event:PointerEvent){if(!inputGesture)return;inputHeight.value=inputGesture.height+inputGesture.y-event.clientY;fitInput()}
function endInput(){if(!inputGesture)return;inputGesture=undefined;saveInput()}
function inputKeys(event:KeyboardEvent){if(!['ArrowUp','ArrowDown'].includes(event.key))return;event.preventDefault();event.stopPropagation();inputHeight.value+=event.key==='ArrowUp'?10:-10;fitInput();saveInput()}
watch([floating,()=>props.visible],async()=>{await nextTick();fitInput()})
onMounted(()=>{inputObserver=new ResizeObserver(fitInput);if(chatHost.value)inputObserver.observe(chatHost.value)})

type Entry={role:'user'|'assistant';content:string;attachments?:AssistantAttachment[];result?:LLMChatResult;service?:string;notice?:boolean}
const attachments=ref<AssistantAttachment[]>([]),attachmentInput=ref<HTMLInputElement>(),attachmentReading=ref(false),attachmentNotice=ref('')
let attachmentGeneration=0
async function addAttachments(files:File[]){
 if(busy.value||attachmentReading.value||!files.length)return
 const generation=attachmentGeneration
 attachmentReading.value=true;error.value='';attachmentNotice.value=''
 try{
  if(attachments.value.length+files.length>8)throw Error('每条消息最多添加8个附件。')
  const loaded=await Promise.all(files.map(readChatAttachment))
  if(generation!==attachmentGeneration)return
  if([...attachments.value,...loaded].reduce((n,a)=>n+attachmentSize(a),0)>16*1024*1024)throw Error('单条消息的附件总大小不能超过16MB。')
  attachments.value.push(...loaded)
 }catch(e){if(generation===attachmentGeneration)error.value=String(e)}finally{if(generation===attachmentGeneration)attachmentReading.value=false}
}
async function pasteMessage(event:ClipboardEvent){
 const files=Array.from(event.clipboardData?.files||[])
 if(files.length){event.preventDefault();await addAttachments(files);return}
 const text=event.clipboardData?.getData('text/plain')||''
 if(Array.from(text).length>=longTextThreshold){event.preventDefault();const count=attachments.value.length;await addAttachments([new File([text],'粘贴文本-'+new Date().toISOString().replace(/[:.]/g,'-')+'.txt',{type:'text/plain'})]);if(attachments.value.length>count)attachmentNotice.value='长文本已转为文本附件，发送时会提供给模型。'}
}
function dropAttachments(event:DragEvent){const files=Array.from(event.dataTransfer?.files||[]);if(files.length){event.preventDefault();void addAttachments(files)}}
const config=ref<LLMProviders>(),providerID=ref(''),model=ref(''),reasoning=ref('')
const reasoningPending=ref(false)
const entries=ref<Entry[]>([]),draft=ref(''),error=ref(''),busy=ref(false),loading=ref(false),transcript=ref<HTMLElement>(),input=ref<HTMLTextAreaElement>()
const review=ref<LLMEditProposal>(),applying=ref(false),reviewError=ref(''),applied=ref<Record<string,boolean>>({}),copied=ref(-1)
const reviewText=ref(''),feedback=ref(''),reviewReply=ref(''),refining=ref(false),reviewEdits=ref<Record<string,string>>({})
let sequence=0,loadSequence=0,request='',disposed=false
const library=ref<InstanceType<typeof AssistantLibrary>>(),conversationID=ref(crypto.randomUUID().replaceAll('-','')),conversationTitle=ref('当前对话'),historyError=ref(''),historySaving=ref(false)
let historyWorkspace='',restoring=false,saveTimer:ReturnType<typeof setTimeout>|undefined,saveQueue=Promise.resolve(true)
const conversationRevisions=new Map<string,string>()
function rememberConversation(){try{localStorage.setItem('smartnote-assistant-last:'+props.workspaceId,conversationID.value)}catch{}}
function clearConversation(){
 attachmentGeneration++;attachmentReading.value=false;attachments.value=[];attachmentNotice.value='';restoring=true;entries.value=[];draft.value='';error.value='';review.value=undefined;reviewEdits.value={};feedback.value='';applied.value={};copied.value=-1;conversationID.value=crypto.randomUUID().replaceAll('-','');conversationTitle.value='当前对话';historyError.value='';historySaving.value=false;restoring=false
}
function storedEntries(){return entries.value.map(entry=>({role:entry.role,content:entry.content+(entry.result?.proposals.length?'\n[历史修改建议状态] '+entry.result.proposals.map(p=>`${p.title}：${applied.value[p.id]?'已确认保存':'尚未保存'}；历史建议须重新生成才能应用`).join('；'):''),attachments:entry.attachments,sources:entry.result?.sources||[],service:entry.service,notice:entry.notice}))}
function persistConversation(workspace=props.workspaceId):Promise<boolean>{
 clearTimeout(saveTimer)
 if(!entries.value.length||!workspace)return Promise.resolve(true)
 const id=conversationID.value,key=workspace+':'+id
 const payload=JSON.parse(JSON.stringify(storedEntries()))
 saveQueue=saveQueue.catch(()=>false).then(async()=>{
  if(props.workspaceId===workspace&&conversationID.value===id)historySaving.value=true
  try{const saved=await api.SaveAssistantConversation(workspace,{id,revision:conversationRevisions.get(key)||'',title:'',updated:'',entries:payload});conversationRevisions.set(key,saved.revision);if(!disposed&&props.workspaceId===workspace&&conversationID.value===id){conversationTitle.value=saved.title;historyError.value='';rememberConversation()};return true}
  catch(e){if(!disposed&&props.workspaceId===workspace&&conversationID.value===id)historyError.value='聊天历史保存失败：'+String(e);return false}
  finally{if(props.workspaceId===workspace&&conversationID.value===id)historySaving.value=false}
 })
 return saveQueue
}
function restoreConversation(value:AssistantConversation){
 cancel();clearTimeout(saveTimer);attachmentGeneration++;attachmentReading.value=false;attachments.value=[];attachmentNotice.value='';restoring=true
 conversationID.value=value.id;conversationTitle.value=value.title;conversationRevisions.set(props.workspaceId+':'+value.id,value.revision)
 entries.value=value.entries.map(entry=>({...entry,result:entry.role==='assistant'&&!entry.notice?{text:entry.content,sources:entry.sources||[],proposals:[],steps:[]}:undefined}));draft.value='';error.value='';historyError.value='';review.value=undefined;reviewEdits.value={};applied.value={};restoring=false;rememberConversation();scroll()
}
async function initializeHistory(){
 const workspace=props.workspaceId,token=loadSequence;if(!workspace||historyWorkspace===workspace)return
 historyWorkspace=workspace
 try{const list=await api.ListAssistantConversations(workspace);if(disposed||token!==loadSequence||props.workspaceId!==workspace)return;if(list.length&&!entries.value.length){let last='';try{last=localStorage.getItem('smartnote-assistant-last:'+workspace)||''}catch{};const selected=list.find(item=>item.id===last)||list[0]!;const saved=await api.GetAssistantConversation(workspace,selected.id);if(!disposed&&token===loadSequence&&props.workspaceId===workspace&&!entries.value.length)restoreConversation(saved)}}
 catch(e){if(token===loadSequence&&props.workspaceId===workspace)historyError.value='聊天历史读取失败：'+String(e)}
}
watch(entries,()=>{if(restoring)return;clearTimeout(saveTimer);saveTimer=setTimeout(()=>void persistConversation(),350)},{deep:true,flush:'sync'})

const provider=computed(()=>config.value?.providers.find(p=>p.id===providerID.value))
const availableModels=(p:LLMProvider)=>p.selectedModels.length?p.selectedModels:[...new Set([...p.models,...(p.model?[p.model]:[])])]
const models=computed(()=>provider.value?availableModels(provider.value):[])
function selectModel(providerId:string,modelId:string){
 if(providerID.value!==providerId){providerID.value=providerId;reasoning.value=provider.value?.reasoningEffort||''}
 model.value=modelId
}
const changes=computed(()=>review.value?diffLines(review.value.oldText,reviewText.value):[])
const md=new MarkdownIt({html:false,linkify:false,breaks:true}).use(admonitions)
md.renderer.rules.image=()=>''
const markdown=(text:string)=>DOMPurify.sanitize(md.render(text),{FORBID_TAGS:['img','video','audio','iframe'],FORBID_ATTR:['style']})
function chooseProvider(){const p=provider.value;model.value=p&&models.value.includes(p.model)?p.model:models.value[0]||'';reasoning.value=p?.reasoningEffort||''}
async function load(){const token=++loadSequence;loading.value=true;try{const value=await api.GetLLMProviders();if(disposed||token!==loadSequence)return;config.value={...value,providers:value.providers.map(p=>({...p,models:p.models||[],selectedModels:p.selectedModels||[]}))};if(!config.value.providers.some(p=>p.id===providerID.value))providerID.value=value.defaultId;const allowed=models.value;if(!allowed.includes(model.value))chooseProvider();if(!value.enabled)cancel();await initializeHistory()}catch(e){if(token===loadSequence)error.value=String(e)}finally{if(token===loadSequence)loading.value=false}}
function scroll(){void nextTick(()=>{if(transcript.value)transcript.value.scrollTop=transcript.value.scrollHeight})}
function cancel(){sequence++;if(request){void api.CancelLLM(request);request='';if(busy.value)entries.value.push({role:'assistant',content:'本次请求已停止。',notice:true})}busy.value=false;scroll()}
function close(){cancel();emit('close')}
async function reset(){cancel();const workspace=props.workspaceId;if(!await persistConversation()||workspace!==props.workspaceId)return;clearConversation();emit('clearSelection');void nextTick(()=>input.value?.focus())}
async function send(text=draft.value,reviewContext?:LLMEditProposal){
 text=text.trim();if((!text&&!attachments.value.length)||busy.value||attachmentReading.value||reasoningPending.value||!config.value?.enabled||!provider.value)return
 if(Array.from(text).length>=longTextThreshold&&!reviewContext){await addAttachments([new File([text],'长文本消息.txt',{type:'text/plain'})]);if(error.value)return;if(draft.value.trim()===text)draft.value='';text=''}
 if(Array.from(text).length>8000){error.value='单条消息最多8000字，较长内容请作为文本附件发送。';return}
 text=text||'请阅读并分析这些附件。'
 const outgoingAttachments=reviewContext?[]:[...attachments.value]
 if(new TextEncoder().encode(JSON.stringify([...storedEntries(),{role:'user',content:text,attachments:outgoingAttachments}])).length>30*1024*1024){error.value='添加这些附件后对话将超出容量，请先新建对话再发送。';return}
 if(entries.value.length>=398||new TextEncoder().encode(JSON.stringify(storedEntries())).length>30*1024*1024){error.value='这段对话已较长，请新建对话继续；已有记录会保留在聊天历史中。';return}
 const token=++sequence,workspaceID=props.workspaceId,selectedProvider=providerID.value,selectedModel=model.value,selectedReasoning=reasoning.value
 if(!workspaceID){error.value='请先打开工作区。';return}
 if(!selectedModel){error.value='请先配置并选择模型。';return}
 busy.value=true;error.value=''
 try{
  const context=await props.prepareContext()
  if(disposed||token!==sequence||workspaceID!==props.workspaceId)return
  entries.value.push({role:'user',content:text,attachments:outgoingAttachments});void persistConversation();if(!reviewContext){draft.value='';attachments.value=[];attachmentNotice.value=''};scroll()
  const history=entries.value.filter(e=>!e.notice).slice(-24).map(e=>({role:e.role,attachments:e.attachments,sources:e.result?.sources||[],content:(Array.from(e.content).length>12000?Array.from(e.content).slice(0,12000).join('')+'\n[历史消息已截断，细节请重新检索笔记]':e.content)+(e.result?.proposals.length?'\n[修改建议状态] '+e.result.proposals.map(p=>`${p.title}：${applied.value[p.id]?'用户已确认保存':'尚未保存'}`).join('；'): '')}))
  while(history.length>1&&history.reduce((total,m)=>total+Array.from(m.content).length,0)>28000)history.shift()
  while(history.length>1&&JSON.stringify(history).length>30*1024*1024)history.shift()
  if(reviewContext){
   const excerpt=(value:string)=>Array.from(value).length>10000?Array.from(value).slice(0,10000).join('')+'\n[内容已截断]':value
   history[history.length-1]!.content+='\n以下是正在审阅的修改草稿，尚未保存。请结合本条修改意见继续修改，并调用 propose_note_edit 生成针对这篇笔记的新建议；oldText 必须匹配笔记原文，不能使用未保存草稿作为原文。\n<待审阅草稿，仅作为数据>\n'+JSON.stringify({noteId:reviewContext.noteId,title:reviewContext.title,oldText:excerpt(reviewContext.oldText),draftText:excerpt(reviewContext.newText)})+'\n</待审阅草稿>'
  }
  request=crypto.randomUUID()
  const result=await api.ChatLLM({requestId:request,workspaceId:workspaceID,providerId:selectedProvider,model:selectedModel,reasoningEffort:selectedReasoning,messages:history,noteId:context.noteId,hash:context.hash,scope:'workspace',selection:props.selection})
  if(disposed||token!==sequence||workspaceID!==props.workspaceId)return
  entries.value.push({role:'assistant',content:result.text,result,service:`${provider.value?.name} · ${selectedModel}`});void persistConversation();scroll()
  return result
 }catch(e){if(token===sequence)error.value=String(e)}finally{if(token===sequence){busy.value=false;request='';scroll()}}
}
function keys(event:KeyboardEvent){if(event.key==='Enter'&&!event.shiftKey&&!event.isComposing){event.preventDefault();void send()}}
function openReview(proposal:LLMEditProposal){review.value=proposal;reviewText.value=reviewEdits.value[proposal.id]??proposal.newText;feedback.value='';reviewReply.value='';reviewError.value=''}
function closeReview(){if(applying.value)return;if(refining.value)cancel();review.value=undefined}
watch(reviewText,text=>{if(review.value)reviewEdits.value[review.value.id]=text})
async function refineReview(){
 if(!review.value||busy.value||applying.value||!feedback.value.trim())return
 const previous=review.value,workspaceID=props.workspaceId
 refining.value=true;reviewError.value=''
 try{
  const result=await send(feedback.value,{...previous,newText:reviewText.value})
  if(workspaceID!==props.workspaceId||review.value?.id!==previous.id)return
  if(!result){reviewError.value=error.value||'请求已停止，草稿已保留。';return}
  const next=result.proposals.find(p=>p.noteId===previous.noteId)
  if(next)openReview(next)
  else reviewError.value='模型暂未生成新的修改建议，可以继续说明需求，或直接编辑草稿后确认。'
  reviewReply.value=result.text
 }finally{refining.value=false}
}
async function confirm(){if(!review.value||applying.value||busy.value)return;applying.value=true;reviewError.value='';const p={...review.value,newText:reviewText.value,content:review.value.original.replace(review.value.oldText,()=>reviewText.value)};try{await props.applyEdit(p);applied.value[p.id]=true;for(const entry of entries.value){if(entry.result)entry.result.proposals=entry.result.proposals.map(old=>old.id===p.id?p:old)}review.value=undefined;scroll()}catch(e){reviewError.value=String(e)}finally{applying.value=false}}
async function copy(entry:Entry,index:number){try{await navigator.clipboard.writeText(entry.content);copied.value=index}catch(e){error.value=String(e)}}
watch(()=>[props.visible,props.revision] as const,([visible])=>{if(visible){void load();scroll()}},{immediate:true})
watch(()=>props.panelHidden,hidden=>{if(hidden)cancel()})
watch(()=>props.workspaceId,(_workspace,previous)=>{
 cancel();clearTimeout(saveTimer)
 if(previous)void persistConversation(previous)
 loadSequence++;loading.value=false;clearConversation();historyWorkspace='';config.value=undefined
 if(props.visible)void load()
})
onBeforeUnmount(()=>{inputObserver?.disconnect();saveInput();clearTimeout(saveTimer);void persistConversation();disposed=true;loadSequence++;cancel()})
</script>
<template><div class="ai-docked-host">
 <div v-if="floating" class="ai-floating-placeholder"><Maximize2 :size="22"/><p>AI 聊天已在浮动窗口打开</p><button class="secondary-button" @click="floating=false">停靠回侧栏</button></div>
 <FloatingWindow :show="floating&&visible" title="AI 聊天" :initial-width="680" :initial-height="740" storage-key="ai-chat" @close="floating=false" @minimize="floating=false"><div ref="floatingTarget" class="ai-floating-body"/></FloatingWindow>
 <Teleport :to="floatingTarget||'body'" :disabled="!floating"><section ref="chatHost" class="ai-assistant" aria-label="AI 助手">
 <header class="ai-heading"><span :title="conversationTitle">{{conversationTitle}}</span><div><button class="icon-button" :aria-label="floating?'停靠 AI 聊天':'浮动 AI 聊天'" :title="floating?'停靠回右侧栏':'浮动窗口，可拖动边缘调整大小'" @click="floating=!floating"><component :is="floating?PanelRightClose:Maximize2" :size="15"/></button><button class="icon-button" aria-label="打开聊天历史" title="聊天历史" :disabled="busy||loading||applying" @click="library?.openHistory()"><History :size="15"/></button><button class="icon-button" aria-label="编辑工作区长期背景" title="长期背景与交流偏好" :disabled="busy||loading||applying" @click="library?.openMemory()"><Brain :size="15"/></button><button class="icon-button" aria-label="新建 AI 对话" title="新建对话" :disabled="applying||historySaving||loading" @click="reset"><Plus :size="15"/></button><button class="icon-button" aria-label="打开大模型配置" title="大模型配置" @click="emit('settings')"><Settings2 :size="15"/></button><button class="icon-button" aria-label="关闭 AI 助手" title="返回笔记大纲，保留对话" @click="close"><X :size="15"/></button></div></header>
 <div v-if="!config?.enabled&&!loading" class="ai-config-notice">请先启用大模型。<button class="text-button" @click="emit('settings')">前往配置</button></div>
 <div ref="transcript" class="ai-transcript" role="log" aria-label="AI 对话记录" aria-live="polite">
 <div v-if="!entries.length" class="ai-welcome"><span><Sparkles :size="26" :stroke-width="1.4"/></span><h3>让积累的知识，接着生长。</h3><p>围绕笔记持续交流，也可以问我待办、书签、知识关联和软件用法。</p><button :disabled="busy||!config?.enabled||!note" @click="send('请概括当前笔记的重点，不要修改原文。')"><Sparkles :size="14"/>概括笔记重点</button><button :disabled="busy||!config?.enabled" @click="draft='请检索工作区中关于';input?.focus()"><Search :size="14"/>检索并汇总相关笔记</button></div>
 <article v-for="(entry,index) in entries" :key="index" class="ai-message" :class="[entry.role,{notice:entry.notice}]"><div class="message-label">{{entry.role==='user'?'你':entry.service||'AI 助手'}}<button v-if="entry.role==='assistant'&&!entry.notice" class="icon-button" :aria-label="'复制回复 '+(index+1)" @click="copy(entry,index)"><Check v-if="copied===index" :size="12"/><Copy v-else :size="12"/></button></div><div v-if="entry.role==='assistant'&&!entry.notice" v-block-copy class="ai-markdown" v-html="markdown(entry.content)"/><p v-else class="plain-message">{{entry.content}}</p>
 <ChatAttachments :items="entry.attachments||[]"/><details v-if="entry.result?.steps.length" class="ai-steps"><summary>查看操作过程 · {{entry.result.steps.length}} 步</summary><p v-for="(step,i) in entry.result.steps" :key="i">{{step}}</p></details>
 <div v-if="entry.result?.sources.length" class="ai-sources"><span>来源笔记</span><button v-for="(source,i) in entry.result.sources" :key="source.noteId" @click="emit('source',source)">[{{i+1}}] {{source.title}}</button></div>
 <div v-for="proposal in entry.result?.proposals||[]" :key="proposal.id" class="ai-proposal"><strong><Check v-if="applied[proposal.id]" :size="14"/><FileText v-else :size="14"/>{{proposal.title}}</strong><p>{{proposal.reason}}</p><button class="secondary-button" :disabled="applied[proposal.id]||busy||applying" @click="openReview(proposal)">{{applied[proposal.id]?'已确认保存':'查看差异并确认'}}</button></div></article>
 <div v-if="busy" class="ai-pending" role="status"><span class="spinner"/>正在思考并按需查阅笔记…</div>
 </div>
 <footer class="ai-composer"><div class="ai-input-resize" role="separator" aria-label="调整 AI 输入框高度" aria-orientation="horizontal" tabindex="0" @pointerdown="resizeInput" @pointermove="moveInput" @pointerup="endInput" @pointercancel="endInput" @lostpointercapture="endInput" @keydown="inputKeys"/><p v-if="historyError" class="bookmark-error" role="alert">{{historyError}}</p><p v-if="error" class="bookmark-error" role="alert">{{error}}</p><div class="ai-input-card" @dragover.prevent @drop="dropAttachments"><ChatAttachments :items="attachments" removable :disabled="busy||attachmentReading" @remove="id=>attachments=attachments.filter(a=>a.id!==id)"/><p v-if="attachmentNotice" class="attachment-notice" role="status">{{attachmentNotice}}</p><input ref="attachmentInput" type="file" multiple hidden aria-label="选择聊天附件" @change="event=>{addAttachments(Array.from((event.target as HTMLInputElement).files||[]));(event.target as HTMLInputElement).value=''}"/><div v-if="selection" class="ai-context"><FileText :size="12"/><span>已选择 {{Array.from(selection).length}} 字</span><button class="icon-button" aria-label="清除 AI 选中文本" :disabled="busy" @click="emit('clearSelection')"><X :size="12"/></button></div><div v-else-if="note" class="ai-context" :title="note.title"><FileText :size="12"/><span>{{note.title}}</span><small>当前笔记</small></div><textarea ref="input" :style="{height:inputHeight+'px'}" v-model="draft" aria-label="发送给 AI 的消息" placeholder="向笔记提问，或描述想做的修改…" rows="3" :disabled="busy||loading||!config?.enabled" @keydown="keys" @paste="pasteMessage"/><div class="ai-options"><div class="ai-model-row"><button class="icon-button" aria-label="添加聊天附件" title="添加图片、文本或其他附件，也可粘贴或拖入" :disabled="busy||loading||attachmentReading||!config?.enabled" @click="attachmentInput?.click()"><Paperclip :size="16"/></button><ChatModelPicker :providers="config?.providers||[]" :provider-id="providerID" :model="model" v-model:reasoning="reasoning" :disabled="busy||loading" :visible="visible&&!panelHidden" @select="selectModel" @pending="reasoningPending=$event"/><button v-if="busy" class="ai-send-button" aria-label="停止" title="停止生成" @click="cancel"><Square :size="14"/></button><button v-else class="ai-send-button" aria-label="发送" title="发送消息（Enter）" :disabled="(!draft.trim()&&!attachments.length)||!config?.enabled||!model||loading||reasoningPending||attachmentReading" @click="send()"><Send :size="15"/></button></div></div></div><div class="ai-compose-hint">Enter 发送 · Shift+Enter 换行 · 支持粘贴附件 <span v-if="historySaving">· 保存对话中…</span><span v-else-if="entries.length&&!historyError">· 对话保存在本机</span></div><p class="ai-privacy">附件随当前对话保存，删除对话一并删除 · 图片及文件识别取决于所选模型</p></footer>
 <AssistantLibrary ref="library" :workspace-id="workspaceId" :current-id="conversationID" :persist="()=>persistConversation()" @restore="restoreConversation" @deleted="id=>{if(id===conversationID)clearConversation()}"/>
 <FloatingWindow :show="!!review" title="审阅笔记修改" :initial-width="900" :initial-height="740" @close="closeReview" @minimize="closeReview"><section v-if="review" class="ai-review" aria-label="AI 修改审阅"><div class="ai-review-body"><h3>{{review.title}}</h3><p class="field-help">{{review.reason}}</p><label class="field-label" for="ai-review-draft">修改后的内容</label><textarea id="ai-review-draft" v-model="reviewText" class="ai-review-draft" aria-label="编辑修改建议" :disabled="applying||refining" spellcheck="false"/><details class="ai-review-differences" open><summary>与原文的差异</summary><pre class="ai-diff"><div v-for="(line,index) in changes" :key="index" :class="line.kind"><span>{{line.kind==='added'?'+ ':line.kind==='removed'?'- ':'  '}}</span>{{line.text||' '}}</div></pre></details><div v-if="reviewReply" class="ai-review-response" aria-live="polite"><strong>AI 回复</strong><div v-block-copy class="ai-markdown" v-html="markdown(reviewReply)"/></div><div class="ai-review-feedback"><label class="field-label" for="ai-review-feedback">继续提出修改意见</label><textarea id="ai-review-feedback" v-model="feedback" aria-label="修改意见" placeholder="例如：语气再自然一点，保留第二段的数据…" rows="2" :disabled="busy||applying"/><div class="ai-review-feedback-actions"><span class="field-help">{{provider?.name}} / {{model}}</span><button v-if="refining" class="secondary-button" @click="cancel">停止生成</button><button v-else class="secondary-button" :disabled="busy||applying||!feedback.trim()||!config?.enabled" @click="refineReview"><Sparkles :size="14"/>继续修改</button></div></div><p class="field-help">可直接编辑草稿，也可继续对话调整。确认后才保存，笔记发生变化时会阻止覆盖。</p><p v-if="reviewError" class="bookmark-error" role="alert">{{reviewError}}</p></div><div class="ai-review-actions"><button class="secondary-button" :disabled="applying" @click="closeReview">返回聊天</button><button class="primary-button" :disabled="applying||busy||reviewText===review.oldText" @click="confirm">{{applying?'正在保存…':'确认修改笔记'}}</button></div></section></FloatingWindow></section></Teleport></div></template>
<style scoped>
.ai-assistant{display:flex;flex-direction:column;height:100%;min-height:0;min-width:0;background:var(--bg)}.ai-heading{display:flex;justify-content:space-between;align-items:center;gap:8px;padding:14px 15px;border-bottom:1px solid var(--line);flex-shrink:0}.ai-heading strong{display:flex;gap:7px;align-items:center;font-size:.9rem;font-weight:500}.ai-heading strong svg{color:var(--accent)}.ai-heading>div{display:flex;gap:2px}.ai-options{padding:10px 14px;display:grid;grid-template-columns:1fr 1fr;gap:10px;border-bottom:1px solid var(--line);flex-shrink:0}.ai-options label{display:flex;flex-direction:column;gap:5px;font-size:.73rem;color:var(--muted);min-width:0}.ai-options select{width:100%;min-width:0;background:var(--bg);color:var(--text);border:1px solid var(--line);border-radius:6px;padding:7px;font-size:.76rem;height:34px}.ai-options :deep(.field-label){margin:0 0 5px;font-size:.73rem;color:var(--muted)}.ai-options :deep(.date-input){padding:7px;height:34px;font-size:.76rem;border-radius:6px}.ai-config-notice{font-size:.78rem;color:var(--muted);padding:10px 14px;line-height:1.8;border-bottom:1px solid var(--line)}.ai-config-notice button{display:inline}.ai-transcript{flex:1;min-height:100px;overflow:auto;padding:16px 14px;overscroll-behavior:contain}.ai-welcome{padding:24px 1px;color:var(--muted)}.ai-welcome>span{color:var(--accent)}.ai-welcome h3{font-size:1rem;color:var(--text);line-height:1.6;font-weight:500;margin:14px 0 8px}.ai-welcome p{font-size:.8rem;line-height:1.9;margin-bottom:22px}.ai-welcome>button{display:flex;align-items:center;gap:8px;width:100%;border:1px solid var(--line);border-radius:7px;padding:11px 12px;margin:8px 0;text-align:left;font-size:.79rem;background:var(--soft);color:var(--secondary)}.ai-welcome button:hover{border-color:var(--accent)}.ai-message{margin-bottom:22px;min-width:0}.message-label{display:flex;align-items:center;justify-content:space-between;gap:6px;font-size:.69rem;color:var(--muted);margin-bottom:7px;overflow-wrap:anywhere}.plain-message{white-space:pre-wrap;overflow-wrap:anywhere;font-size:.85rem;line-height:1.8}.user .plain-message{padding:11px 13px;background:var(--accent-bg);border-radius:9px;color:var(--text)}.notice{color:var(--muted)}.notice .plain-message{font-size:.78rem}.ai-markdown{font-size:.85rem;line-height:1.9;overflow-wrap:anywhere}.ai-markdown :deep(p){margin:8px 0}.ai-markdown :deep(h1),.ai-markdown :deep(h2),.ai-markdown :deep(h3){font-size:1em;margin:15px 0 8px}.ai-markdown :deep(ul),.ai-markdown :deep(ol){padding-left:20px}.ai-markdown :deep(pre){overflow:auto;font-size:.78rem;padding:10px;background:var(--soft);border:1px solid var(--line);border-radius:6px}.ai-markdown :deep(table){display:block;overflow:auto;border-collapse:collapse}.ai-markdown :deep(th),.ai-markdown :deep(td){border:1px solid var(--line);padding:5px}.ai-markdown :deep(a){color:var(--accent)}.ai-steps{font-size:.73rem;color:var(--muted);margin-top:12px;line-height:1.8}.ai-steps summary{cursor:pointer}.ai-steps p{margin:5px 0}.ai-sources{display:flex;flex-direction:column;align-items:flex-start;gap:5px;margin-top:12px;font-size:.75rem}.ai-sources>span{color:var(--muted);font-size:.7rem}.ai-sources button{color:var(--accent);text-align:left;overflow-wrap:anywhere}.ai-proposal{padding:12px;margin-top:12px;border:1px solid var(--line);border-radius:7px;background:var(--soft)}.ai-proposal strong{display:flex;align-items:center;gap:6px;font-size:.8rem;font-weight:500}.ai-proposal p{font-size:.76rem;line-height:1.8;color:var(--muted);margin:7px 0 10px}.ai-proposal button{font-size:.76rem}.ai-pending{display:flex;align-items:center;gap:8px;font-size:.77rem;color:var(--muted)}.ai-composer{flex-shrink:0;padding:10px 14px 12px;border-top:1px solid var(--line)}.ai-context{display:flex;align-items:center;gap:6px;margin:0 0 8px;font-size:.72rem;color:var(--muted)}.ai-context>span{overflow:hidden;white-space:nowrap;text-overflow:ellipsis}.ai-context .icon-button{margin-left:auto}.ai-composer textarea{resize:vertical;width:100%;min-height:70px;max-height:180px;display:block;border:1px solid var(--line);background:var(--bg);color:var(--text);border-radius:8px;padding:11px;outline:none;font:inherit;font-size:.82rem;line-height:1.7}.ai-composer textarea:focus{border-color:var(--accent)}.ai-send-row{display:flex;justify-content:space-between;align-items:center;gap:6px;margin-top:8px}.ai-send-row>span{font-size:.64rem;color:var(--muted)}.ai-send-row button{font-size:.76rem;padding:7px 12px}.ai-privacy{font-size:.64rem;line-height:1.7;color:var(--muted);margin:8px 0 0}.ai-review{padding:20px;width:100%;min-width:0;overflow:auto}.ai-review h3{font-size:1rem;font-weight:500}.ai-diff{font-size:.87rem;line-height:1.9;white-space:pre-wrap;overflow-wrap:anywhere;border:1px solid var(--line);border-radius:7px;padding:12px;max-height:350px;overflow:auto}.ai-diff .added{background:#4aab5930}.ai-diff .removed{background:#d44b5030;text-decoration:line-through}.ai-review-actions{display:flex;gap:9px;justify-content:flex-end;margin-top:20px}button:disabled{opacity:.45;cursor:default}.bookmark-error{font-size:.76rem;line-height:1.7;overflow-wrap:anywhere}
.ai-heading{padding:6px 12px;min-height:36px;border-bottom:0}.ai-heading>span{font-size:.71rem;color:var(--muted)}.ai-transcript{padding:8px 14px 14px;min-height:60px}.ai-welcome{padding-top:30px}.ai-composer{padding:8px 10px 10px;border-top:0}.ai-input-card{border:1px solid var(--line);border-radius:10px;padding:9px;background:var(--soft);transition:border-color .15s,box-shadow .15s}.ai-input-card:focus-within{border-color:var(--accent);box-shadow:0 0 0 2px var(--accent-bg)}.ai-context{display:flex;width:fit-content;max-width:100%;border:1px solid var(--line);border-radius:5px;background:var(--bg);padding:3px 6px;margin:0 0 3px;min-height:25px}.ai-context svg{flex-shrink:0}.ai-context small{white-space:nowrap;font-size:.62rem;color:var(--muted);margin-left:5px}.ai-context .icon-button{width:18px;height:18px;padding:2px}.ai-composer textarea{border:0;background:transparent;border-radius:0;padding:8px 2px;min-height:76px;max-height:150px;font-size:.82rem;box-shadow:none}.ai-composer textarea:focus{border:0;box-shadow:none;outline:none}.ai-options{display:flex;flex-direction:column;gap:5px;padding:5px 0 0;border:0}.ai-config-row,.ai-model-row{display:flex;align-items:center;gap:5px;min-width:0}.ai-options select{border:0;background:transparent;color:var(--secondary);height:28px;padding:3px 2px;font-size:.72rem;text-overflow:ellipsis;width:auto;max-width:100%;outline-offset:0}.ai-options select:focus-visible{outline:1px solid var(--accent)}.ai-config-row>select:first-child{flex:1;min-width:0}.ai-config-row>select:last-child{max-width:110px}.ai-model-row>select{flex:1;min-width:0}.ai-options .reasoning-select{flex:0 1 105px;min-width:60px}.ai-options :deep(.date-input){background:transparent;color:var(--muted);border:0;height:28px;padding:3px 0;font-size:.7rem;box-shadow:none}.ai-send-button{width:29px;height:29px;border-radius:50%;background:var(--accent);color:var(--bg);display:flex;align-items:center;justify-content:center;flex-shrink:0}.ai-send-button:hover{filter:brightness(1.12)}.ai-send-button:focus-visible{outline:2px solid var(--accent);outline-offset:2px}.ai-compose-hint{font-size:.63rem;color:var(--muted);margin:7px 2px 0}.ai-privacy{font-size:.62rem;margin:4px 2px 0;line-height:1.6}.ai-composer .bookmark-error{margin:0 0 8px}.user .plain-message{background:var(--soft);border:1px solid var(--line)}


.attachment-notice{font-size:.7rem;color:var(--muted);line-height:1.7;margin:5px 0}.ai-model-picker{width:100%;min-width:0}.ai-options .reasoning-select{flex:1.25;min-width:0}.ai-options :deep(.n-base-selection){font-size:.75rem}.ai-model-row{gap:7px}.ai-options{gap:7px}
.ai-review-draft,.ai-review-feedback textarea{display:block;width:100%;resize:vertical;border:1px solid var(--line);border-radius:7px;background:var(--bg);color:var(--text);padding:12px;font:inherit;line-height:1.8;outline:none}.ai-review-draft{min-height:150px;font-family:var(--note-font-family);font-size:var(--note-font-size);tab-size:4}.ai-review textarea:focus{border-color:var(--accent);box-shadow:0 0 0 2px var(--accent-bg)}.ai-review-differences{margin-top:16px}.ai-review-differences summary{font-size:.8rem;color:var(--muted);cursor:pointer}.ai-review .ai-diff{max-height:220px;margin-top:9px}.ai-review-response{margin-top:14px;padding:12px;border:1px solid var(--line);border-radius:7px;background:var(--soft);max-height:180px;overflow:auto}.ai-review-response>strong{font-size:.76rem;color:var(--accent);font-weight:500}.ai-review-feedback{border-top:1px solid var(--line);margin-top:16px}.ai-review-feedback textarea{min-height:65px;font-size:.85rem}.ai-review-feedback-actions{display:flex;gap:12px;align-items:center;justify-content:space-between;margin-top:8px}.ai-review-feedback-actions .field-help{margin:0;overflow-wrap:anywhere}.ai-review-feedback-actions button{flex-shrink:0}
.ai-review{display:flex;flex-direction:column;min-height:0;padding:0;overflow:hidden}.ai-review-body{flex:1;min-height:0;overflow:auto;padding:20px}.ai-review-actions{flex-shrink:0;padding:12px 20px;margin:0;border-top:1px solid var(--line);background:var(--bg)}
.ai-heading>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;min-width:0}.ai-heading>div{flex-shrink:0}
.ai-docked-host,.ai-floating-body{height:100%;width:100%;min-height:0;min-width:0}.ai-floating-placeholder{padding:32px 15px;text-align:center;color:var(--muted);font-size:.8rem;line-height:2}.ai-floating-placeholder>svg{margin:auto}.ai-floating-placeholder button{margin:16px auto}.ai-heading>span{min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.ai-heading>div{flex-shrink:0}.ai-assistant{overflow:auto}.ai-composer{position:relative}.ai-input-resize{height:9px;margin:-8px 0 3px;cursor:ns-resize;touch-action:none;display:flex;align-items:center;justify-content:center}.ai-input-resize:after{content:'';width:36px;height:3px;border-radius:3px;background:var(--line)}.ai-input-resize:hover:after,.ai-input-resize:focus-visible:after{background:var(--accent)}.ai-composer textarea{resize:none;max-height:none}
.ai-model-row .ai-model-picker{width:auto;flex:1;min-width:0}.ai-model-row{gap:3px}.ai-model-row>.icon-button{width:23px;flex-shrink:0}.ai-options{padding-top:5px}
</style>
