<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {Brain,History,MessageSquare,Search,Trash2} from 'lucide-vue-next'
import {api,type AssistantMemory,type AssistantConversation,type AssistantConversationInfo} from '../services/api'
import FloatingWindow from './FloatingWindow.vue'
const props=defineProps<{workspaceId:string;currentId:string;persist:()=>Promise<boolean>}>()
const emit=defineEmits<{restore:[conversation:AssistantConversation];deleted:[id:string]}>()
const historyOpen=ref(false),memoryOpen=ref(false),busy=ref(false),error=ref(''),message=ref(''),query=ref(''),pendingDelete=ref('')
const items=ref<AssistantConversationInfo[]>([]),memory=ref<AssistantMemory>({revision:'',text:''}),memoryDraft=ref('')
let generation=0
const filtered=computed(()=>items.value.filter(c=>c.title.toLowerCase().includes(query.value.trim().toLowerCase())))
const date=(value:string)=>new Date(value).toLocaleString(undefined,{year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit'})
async function openHistory(){
 if(busy.value)return
 historyOpen.value=true;memoryOpen.value=false;error.value='';query.value='';pendingDelete.value='';busy.value=true
 const token=++generation,workspace=props.workspaceId
 try{if(!await props.persist()){error.value='当前对话未能保存，请返回聊天查看提示后重试。';return};const data=await api.ListAssistantConversations(workspace);if(token===generation&&workspace===props.workspaceId)items.value=data}
 catch(e){if(token===generation)error.value=String(e)}finally{if(token===generation)busy.value=false}
}
async function restore(id:string){
 if(busy.value)return;const token=++generation,workspace=props.workspaceId;busy.value=true;error.value=''
 try{if(!await props.persist()){error.value='当前对话未能保存，请返回聊天查看提示后重试。';return};const data=await api.GetAssistantConversation(workspace,id);if(token!==generation||workspace!==props.workspaceId)return;emit('restore',data);historyOpen.value=false}
 catch(e){if(token===generation)error.value=String(e)}finally{if(token===generation)busy.value=false}
}
async function remove(id:string){
 if(busy.value)return;const token=++generation,workspace=props.workspaceId;busy.value=true;error.value=''
 try{if(!await props.persist()){error.value='当前对话未能保存，请返回聊天查看提示后重试。';return};const data=await api.GetAssistantConversation(workspace,id);if(token!==generation||workspace!==props.workspaceId)return;await api.DeleteAssistantConversation(workspace,id,data.revision);if(token!==generation||workspace!==props.workspaceId)return;emit('deleted',id);const remaining=await api.ListAssistantConversations(workspace);if(token!==generation||workspace!==props.workspaceId)return;items.value=remaining;pendingDelete.value=''}
 catch(e){if(token===generation)error.value=String(e)}finally{if(token===generation)busy.value=false}
}
async function openMemory(){
 if(busy.value)return;memoryOpen.value=true;historyOpen.value=false;error.value='';message.value='';busy.value=true
 const token=++generation,workspace=props.workspaceId
 try{const data=await api.GetAssistantMemory(workspace);if(token!==generation||workspace!==props.workspaceId)return;memory.value=data;memoryDraft.value=data.text}
 catch(e){if(token===generation)error.value=String(e)}finally{if(token===generation)busy.value=false}
}
async function saveMemory(){
 if(busy.value)return;busy.value=true;error.value='';message.value='';const token=++generation,workspace=props.workspaceId
 try{const data=await api.SaveAssistantMemory(workspace,{revision:memory.value.revision,text:memoryDraft.value});if(token!==generation||workspace!==props.workspaceId)return;memory.value=data;memoryDraft.value=data.text;message.value='长期背景已保存，下次提问起生效。'}
 catch(e){if(token===generation)error.value=String(e)}finally{if(token===generation)busy.value=false}
}
watch(()=>props.workspaceId,()=>{generation++;historyOpen.value=false;memoryOpen.value=false;busy.value=false;items.value=[];memory.value={revision:'',text:''};memoryDraft.value='';error.value='';message.value='';query.value='';pendingDelete.value=''})
defineExpose({openHistory,openMemory})
</script>
<template>
 <FloatingWindow :show="historyOpen" title="聊天历史" :initial-width="700" :initial-height="590" @close="historyOpen=false" @minimize="historyOpen=false">
  <section class="assistant-library" aria-label="聊天历史">
   <header class="library-intro"><History :size="20"/><div><strong>接着上次的话题聊</strong><p>当前工作区的对话保存在本机，点击即可继续。历史修改建议需要重新生成。</p></div></header>
   <label class="library-search"><Search :size="15"/><input v-model="query" aria-label="查找聊天历史" placeholder="查找对话…"/></label>
   <p v-if="busy" class="field-help" role="status">正在读取…</p>
   <p v-if="error" class="library-error" role="alert">{{error}}</p>
   <div class="conversation-list"><article v-for="item in filtered" :key="item.id" :class="{active:item.id===currentId}">
    <button class="conversation-open" :disabled="busy" :aria-label="'继续对话：'+item.title" @click="restore(item.id)"><MessageSquare :size="17"/><span><strong>{{item.title||'未命名对话'}}</strong><small>{{date(item.updated)}} · {{item.messages}} 条消息 <b v-if="item.id===currentId">· 当前</b></small></span></button>
    <div v-if="pendingDelete===item.id" class="delete-confirm"><span>删除此对话？</span><button class="text-button danger" :disabled="busy" @click="remove(item.id)">确认删除</button><button class="text-button" :disabled="busy" @click="pendingDelete=''">取消</button></div>
    <button v-else class="icon-button danger" :aria-label="'删除对话：'+item.title" :disabled="busy" @click="pendingDelete=item.id"><Trash2 :size="15"/></button>
   </article></div>
   <p v-if="!busy&&!filtered.length" class="library-empty">{{query?'没有匹配的对话。':'还没有对话，发送第一条消息后会自动保存。'}}</p>
  </section>
 </FloatingWindow>
 <FloatingWindow :show="memoryOpen" title="工作区长期背景" :initial-width="750" :initial-height="640" @close="memoryOpen=false" @minimize="memoryOpen=false">
  <section class="assistant-library memory-library" aria-label="工作区长期背景">
   <header class="library-intro"><Brain :size="21"/><div><strong>让交流有持续的背景</strong><p>写下项目背景、术语和交流偏好，保存后每次主动提问都会提供给所选模型。不同工作区独立保存。</p></div></header>
   <label class="field-label" for="assistant-memory">希望助手长期了解的内容</label>
   <textarea id="assistant-memory" v-model="memoryDraft" aria-label="长期背景内容" :disabled="busy" maxlength="8000" placeholder="例如：&#10;我在维护一个数据库迁移项目，灰度环境使用……&#10;这里的“归档”指已完成任务的笔记归档。&#10;回答先给结论，引用相关笔记；不确定时说明依据。" spellcheck="false"/>
   <div class="memory-meta"><span>仅保存在本机 · 可随时编辑、清空</span><span>{{Array.from(memoryDraft).length}} / 8000</span></div>
   <p class="field-help">笔记仍是知识来源。对话历史用于继续交流；长期背景用于提供稳定参考。清空文本并保存即可停止在后续提问中提供。</p>
   <p v-if="error" class="library-error" role="alert">{{error}}</p><p v-if="message" class="memory-success" role="status">{{message}}</p>
   <footer><button class="text-button" :disabled="busy" @click="memoryDraft=''">清空内容</button><div><button class="secondary-button" @click="memoryOpen=false">关闭</button><button class="primary-button" :disabled="busy" @click="saveMemory">保存长期背景</button></div></footer>
  </section>
 </FloatingWindow>
</template>
<style scoped>
.assistant-library{padding:22px;overflow:auto;min-height:0;width:100%}.library-intro{display:flex;gap:12px;align-items:flex-start;margin-bottom:18px}.library-intro>svg{flex-shrink:0;color:var(--accent);margin-top:2px}.library-intro strong{font-size:.94rem;font-weight:500}.library-intro p,.field-help{color:var(--muted);font-size:.78rem;line-height:1.9;margin:7px 0}.library-search{display:flex;align-items:center;gap:9px;border:1px solid var(--line);border-radius:8px;padding:9px 11px;background:var(--soft);color:var(--muted);margin-bottom:14px}.library-search input{border:0;outline:none;background:transparent;min-width:0;width:100%;color:var(--text);font:inherit;font-size:.83rem}.conversation-list article{display:flex;gap:8px;align-items:center;padding:5px 10px;border:1px solid var(--line);border-radius:8px;margin:9px 0}.conversation-list article.active{border-color:var(--accent);background:var(--accent-bg)}.conversation-open{display:flex;flex:1;min-width:0;gap:11px;align-items:center;text-align:left;padding:10px 0;color:var(--text)}.conversation-open>svg{flex-shrink:0;color:var(--muted)}.conversation-open>span{min-width:0}.conversation-open strong{display:block;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;font-size:.85rem;font-weight:500}.conversation-open small{display:block;font-size:.7rem;color:var(--muted);margin-top:6px}.conversation-open b{font-weight:400;color:var(--accent)}.delete-confirm{display:flex;gap:9px;align-items:center;font-size:.72rem;flex-wrap:wrap}.danger,.library-error{color:var(--danger,#cc5656)}.library-error{font-size:.8rem;line-height:1.8;overflow-wrap:anywhere}.library-empty{text-align:center;padding:40px 10px;color:var(--muted);font-size:.83rem}.memory-library{display:flex;flex-direction:column;min-height:0}.memory-library textarea{display:block;width:100%;flex:1;min-height:170px;resize:vertical;border:1px solid var(--line);border-radius:8px;background:var(--soft);color:var(--text);font:inherit;font-size:.87rem;line-height:1.9;padding:14px;outline:none}.memory-library textarea:focus{border-color:var(--accent)}.memory-meta{display:flex;justify-content:space-between;gap:10px;color:var(--muted);font-size:.69rem;margin:9px 0}.memory-library footer{display:flex;justify-content:space-between;gap:12px;margin-top:14px}.memory-library footer>div{display:flex;gap:9px}.memory-success{color:var(--accent);font-size:.8rem}button:disabled{opacity:.45;cursor:default}
</style>
