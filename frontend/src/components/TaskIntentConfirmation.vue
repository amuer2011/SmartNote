<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {NButton} from 'naive-ui'
import {CalendarDays,FileText,ListTodo} from 'lucide-vue-next'
import StyledDatePicker from './StyledDatePicker.vue'
import ResizableModal from './ResizableModal.vue'
import {api,type Task,type Note} from '../services/api'
import {recurrenceLabel} from '../services/recurrence'
import {useWorkspace} from '../stores/workspace'
const props=defineProps<{workspaceId:string}>()
const emit=defineEmits<{open:[note:Note,line:number]}>()
const store=useWorkspace(),busy=ref(false),error=ref(''),deferred=ref(new Set<string>())
const key=(t:Task)=>t.id+':'+t.sourceRevision
const allPending=computed(()=>store.data.settings.activeId===props.workspaceId?store.data.tasks.filter(t=>t.needsConfirmation):[])
const pending=computed(()=>allPending.value.filter(t=>!deferred.value.has(key(t))))
const task=computed(()=>pending.value[0])
const until=ref('')
watch(()=>task.value?key(task.value):'',()=>{until.value=task.value?.recurrence?.until||'';error.value=''}, {immediate:true})
const note=computed(()=>store.data.notes.find(n=>n.id===task.value?.noteId))
function later(){if(busy.value)return;deferred.value=new Set([...deferred.value,...pending.value.map(key)]);error.value=''}
async function resolve(accept:boolean,ignoreNote=false){
 if(busy.value||!task.value)return
 const item=task.value,id=props.workspaceId
 busy.value=true;error.value=''
 try{
  await store.save();if(store.dirty)throw Error('请先处理笔记的未保存修改')
  if(store.workspace?.id!==id)return
  await api.ResolveTaskIntent({workspaceId:id,taskId:item.id,sourceRevision:item.sourceRevision||'',accept,ignoreNote,until:accept&&item.recurrence?until.value:''})
  await store.refresh()
 }catch(e){error.value=String(e)}finally{busy.value=false}
}
function openSource(){if(!note.value||!task.value)return;const n=note.value,line=task.value.line;later();emit('open',n,line)}
async function ignoreAll(){
 if(busy.value||!task.value)return
 const id=props.workspaceId,items=allPending.value.map(item=>({taskId:item.id,sourceRevision:item.sourceRevision||''}))
 busy.value=true;error.value=''
 try{
  await store.save();if(store.dirty)throw Error('请先处理笔记的未保存修改')
  if(store.workspace?.id!==id)return
  await api.IgnoreAllTaskIntents({workspaceId:id,tasks:items})
  await store.refresh()
 }catch(e){if(store.workspace?.id===id){await store.refresh();error.value=String(e)}}finally{busy.value=false}
}
</script>
<template>
 <ResizableModal :show="!!task" title="确认识别到的待办" preset="card" style="width:640px;max-width:calc(100vw - 40px)" :mask-closable="!busy" :close-on-esc="!busy" :closable="!busy" @update:show="value=>{if(!value)later()}" @minimize="later">
  <div v-if="task" class="intent-confirmation">
   <div class="intent-heading"><span class="intent-symbol"><ListTodo :size="23"/></span><div><strong>是否将这篇笔记加入待办？</strong><p>识别到任务意图，确认后才会加入待办和日历。</p></div></div>
   <section class="intent-details"><h3>{{task.title}}</h3><p><FileText :size="15"/><span>{{note?.path||task.noteTitle}}</span></p><p><CalendarDays :size="15"/><span>{{task.dueAt?'日期：'+task.dueAt:'未识别到具体日期'}}<template v-if="task.recurrence"> · {{recurrenceLabel(task.recurrence)}}</template></span></p></section>
   <div v-if="task.recurrence" class="intent-until"><StyledDatePicker v-model="until" label="循环截止日期（可选）"/><p class="intent-help">留空表示无截止日期；设置后，截止日期当天仍可生成任务。</p></div>
   <p class="intent-help">忽略本次：编辑后重新识别。忽略此笔记：以后不再自动识别待办或自动分类。忽略所有：对当前工作区全部 {{allPending.length}} 篇待确认笔记执行相同操作，已加入的待办不受影响。可在笔记右键菜单恢复。</p>
   <button class="text-button" :disabled="busy||!note" @click="openSource">查看来源笔记</button>
   <p v-if="error" class="intent-error" role="alert">{{error}}</p>
   <p v-if="pending.length>1" class="intent-help">还有 {{pending.length-1}} 项识别结果等待确认</p>
  </div>
  <template #footer><div class="intent-actions"><NButton :disabled="busy" @click="later">稍后确认</NButton><NButton :disabled="busy" @click="resolve(false)">忽略本次识别</NButton><NButton :disabled="busy" @click="resolve(false,true)">忽略此笔记</NButton><NButton :disabled="busy" :title="'忽略当前工作区全部 '+allPending.length+' 篇待确认笔记'" @click="ignoreAll">忽略所有</NButton><NButton type="primary" :loading="busy" @click="resolve(true)">加入待办</NButton></div></template>
 </ResizableModal>
</template>
<style scoped>
.intent-confirmation{color:var(--text)}.intent-heading{display:flex;gap:12px;align-items:center}.intent-symbol{display:flex;padding:11px;border-radius:12px;color:var(--accent);background:var(--accent-bg)}.intent-heading strong{font-size:1.05rem}.intent-heading p,.intent-help{color:var(--muted);font-size:.86rem;line-height:1.7;margin:6px 0}.intent-details{margin:18px 0;padding:15px;background:var(--soft);border:1px solid var(--line);border-radius:10px}.intent-details h3{margin:0 0 13px;font-size:1rem;overflow-wrap:anywhere}.intent-details p{display:flex;gap:8px;color:var(--muted);margin:8px 0;font-size:.86rem;overflow-wrap:anywhere}.intent-details svg{flex-shrink:0;margin-top:2px}.intent-actions{display:flex;gap:8px;flex-wrap:wrap;justify-content:flex-end}.intent-error{color:var(--danger,#c54d61);overflow-wrap:anywhere}
</style>
