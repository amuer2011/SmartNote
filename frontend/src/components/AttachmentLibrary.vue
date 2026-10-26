<script setup lang="ts">
import {computed,onBeforeUnmount,ref,watch} from 'vue'
import {NButton,NModal} from 'naive-ui'
import {Download,FileText,Paperclip,Search,Trash2,Eraser} from 'lucide-vue-next'
import {api,type Attachment,type AttachmentUsage,type Note} from '../services/api'
const props=defineProps<{workspaceId?:string;attachments:Attachment[];notes:Note[];active:boolean;selected?:string;downloading?:boolean;beforeDelete:()=>Promise<void>}>()
const emit=defineEmits<{open:[attachment:Attachment];download:[attachment:Attachment];changed:[ids:string[]];message:[text:string]}>()
const query=ref(''),onlyUnused=ref(false),checked=ref<string[]>([]),usage=ref<AttachmentUsage[]>([]),loading=ref(false),deleting=ref(false),error=ref('')
const confirmation=ref<{ids:string[];cleanup:boolean}>(),allowReferenced=ref(false)
let generation=0,disposed=false
const usages=computed(()=>new Map(usage.value.map(u=>[u.id,u])))
const count=(a:Attachment)=>usages.value.get(a.id)||{id:a.id,notes:props.notes.filter(n=>n.content.includes('smartnote-asset://'+a.id)||(a.remoteUrl&&n.content.includes(a.remoteUrl))).length,trash:0}
const unused=computed(()=>props.attachments.filter(a=>count(a).notes===0&&count(a).trash===0))
const visible=computed(()=>props.attachments.filter(a=>a.name.toLowerCase().includes(query.value.toLowerCase())&&(!onlyUnused.value||unused.value.some(item=>item.id===a.id))))
const allChecked=computed(()=>visible.value.length>0&&visible.value.every(a=>checked.value.includes(a.id)))
const targets=computed(()=>props.attachments.filter(a=>confirmation.value?.ids.includes(a.id)))
const referenced=computed(()=>targets.value.filter(a=>count(a).notes>0||count(a).trash>0))
async function refresh(){
 const token=++generation,workspace=props.workspaceId;if(!workspace||!props.active)return
 loading.value=true;error.value=''
 try{const result=await api.AttachmentUsage(workspace);if(!disposed&&token===generation)usage.value=result||[]}
 catch(e){if(!disposed&&token===generation)error.value='附件引用检查失败：'+String(e)}finally{if(token===generation)loading.value=false}
}
watch(()=>JSON.stringify([props.workspaceId,props.active,props.attachments,props.notes.map(n=>[n.id,n.hash])]),()=>void refresh(),{immediate:true})
watch(()=>props.attachments,()=>{const ids=new Set(props.attachments.map(a=>a.id));checked.value=checked.value.filter(id=>ids.has(id))})
watch(()=>props.workspaceId,()=>{generation++;confirmation.value=undefined;checked.value=[];usage.value=[]})
function toggle(id:string,value:boolean){checked.value=value?[...new Set([...checked.value,id])]:checked.value.filter(item=>item!==id)}
function selectAll(value:boolean){for(const a of visible.value)toggle(a.id,value)}
async function prepare(ids:string[],cleanup=false){
 if(!ids.length||loading.value||deleting.value)return
 error.value=''
 try{await props.beforeDelete();await refresh();if(error.value)return;confirmation.value={ids,cleanup};allowReferenced.value=false}catch(e){error.value=String(e)}
}
async function remove(){
 const request=confirmation.value,workspace=props.workspaceId;if(!request||!workspace||deleting.value)return
 deleting.value=true;error.value=''
 try{
  await props.beforeDelete()
  const result=await api.DeleteAttachments({workspaceId:workspace,ids:request.ids,force:!request.cleanup&&allowReferenced.value})
  if(disposed||workspace!==props.workspaceId)return
  confirmation.value=undefined;checked.value=checked.value.filter(id=>!result.deleted.includes(id));emit('changed',result.deleted)
  const skipped=Object.keys(result.skipped||{}).length
  emit('message',`已删除 ${result.deleted.length} 个附件`+(skipped?`，${skipped} 个因新增引用或已不存在而跳过`:'')+(result.warnings?.length?'；'+result.warnings.join('；'):''))
  await refresh()
 }catch(e){if(workspace===props.workspaceId)error.value=String(e)}finally{deleting.value=false}
}
onBeforeUnmount(()=>{disposed=true;generation++})
</script>
<template>
 <div class="attachment-toolbar">
  <div class="list-search"><Search :size="15"/><input v-model="query" placeholder="按文件名查找附件…" aria-label="按文件名查找附件"/></div>
  <div class="library-summary"><span>{{visible.length}} 个文件 · {{(attachments.reduce((sum,a)=>sum+a.size,0)/1024/1024).toFixed(1)}} MB</span><label><input v-model="onlyUnused" type="checkbox"/>仅未引用</label></div>
  <div class="library-actions"><button class="secondary-button" :disabled="loading||deleting||!unused.length||!!error" aria-label="清理未引用附件" @click="prepare(unused.map(a=>a.id),true)"><Eraser :size="14"/>清理未引用（{{unused.length}}）</button><button class="secondary-button" :disabled="loading||deleting||!checked.length||!!error" @click="prepare(checked)"><Trash2 :size="14"/>删除选中（{{checked.length}}）</button></div>
  <label v-if="visible.length" class="library-selection"><input type="checkbox" :checked="allChecked" :indeterminate="!allChecked&&visible.some(a=>checked.includes(a.id))" aria-label="全选当前附件" @change="selectAll(($event.target as HTMLInputElement).checked)"/>全选当前结果<span v-if="loading">检查引用中…</span></label>
 </div>
 <p v-if="error&&!confirmation" class="library-error" role="alert">{{error}}<button class="text-button" @click="refresh">重试</button></p>
 <div class="attachment-list"><div v-for="a in visible" :key="a.id" class="attachment-row" :class="{selected:selected===a.id}">
  <input type="checkbox" :checked="checked.includes(a.id)" :aria-label="'选择附件 '+a.name" @change="toggle(a.id,($event.target as HTMLInputElement).checked)"/>
  <button class="attachment-open" @click="emit('open',a)"><FileText v-if="/^(text\/|application\/json)/.test(a.mime)" :size="18"/><Paperclip v-else :size="18"/><span><strong>{{a.name}}</strong><small>{{(a.size/1024).toFixed(1)}} KB · {{count(a).notes}} 篇笔记引用<span v-if="count(a).trash"> · 回收站 {{count(a).trash}} 篇</span></small></span></button>
  <button class="icon-button" :aria-label="'下载附件 '+a.name" title="下载附件" :disabled="downloading" @click="emit('download',a)"><Download :size="15"/></button>
  <button class="icon-button delete-attachment" :aria-label="'删除附件 '+a.name" title="删除附件" :disabled="loading||deleting||!!error" @click="prepare([a.id])"><Trash2 :size="15"/></button>
 </div></div>
 <div v-if="!visible.length" class="empty-state"><Paperclip :size="38"/><h3>{{attachments.length?'没有符合条件的附件':'重要的细节，也值得好好收藏。'}}</h3><p>在笔记编辑器中粘贴截图、拖入文件，或点击附件按钮。</p></div>
 <NModal :show="!!confirmation" preset="card" title="删除附件" class="resizable-modal attachment-delete-modal" style="width:520px;max-width:calc(100vw - 32px)" :mask-closable="!deleting" :closable="!deleting" @update:show="value=>{if(!value&&!deleting)confirmation=undefined}">
  <p>将删除 {{targets.length}} 个附件的本地记录及原件，此操作无法撤销。图床上的文件保留。</p>
  <ul class="deletion-files"><li v-for="a in targets.slice(0,8)" :key="a.id">{{a.name}}</li><li v-if="targets.length>8">另有 {{targets.length-8}} 个附件</li></ul>
  <p v-if="confirmation?.cleanup" class="library-help">删除前会再次检查笔记与回收站的引用，仍在使用的附件会自动跳过。</p>
  <label v-else-if="referenced.length" class="allow-referenced"><input v-model="allowReferenced" type="checkbox" aria-label="允许删除仍被引用的附件"/>{{referenced.length}} 个附件仍被引用；同时删除它们（相关链接会失效）</label>
  <p v-if="error" class="library-error" role="alert">{{error}}</p>
  <template #footer><div class="delete-actions"><NButton :disabled="deleting" @click="confirmation=undefined">取消</NButton><NButton type="error" :loading="deleting" @click="remove">确认删除</NButton></div></template>
 </NModal>
</template>
<style scoped>
.library-summary,.library-actions,.library-selection{display:flex;align-items:center;gap:10px;flex-wrap:wrap;font-size:.78rem;color:var(--muted)}.library-summary{justify-content:space-between}.library-summary label,.library-selection{display:flex;align-items:center;gap:6px}.library-actions .secondary-button{display:inline-flex;align-items:center;gap:5px;font-size:.75rem;padding:6px 8px}.library-selection span{margin-left:auto}.attachment-toolbar{display:flex;flex-direction:column;gap:12px}.attachment-row{gap:4px}.attachment-row>input{flex-shrink:0;margin-right:3px;accent-color:var(--accent)}.attachment-open{gap:7px}.attachment-row .icon-button{flex-shrink:0;width:23px}.delete-attachment:hover{color:var(--danger,#c54d61)}.library-error{color:var(--danger,#c54d61);font-size:.84rem;line-height:1.8;overflow-wrap:anywhere}.library-error .text-button{margin-left:8px}.deletion-files{max-height:180px;overflow:auto;background:var(--soft);border:1px solid var(--line);padding:12px 12px 12px 32px;margin:16px 0;border-radius:8px}.deletion-files li{overflow-wrap:anywhere;line-height:1.8}.library-help{color:var(--muted);line-height:1.8;font-size:.85rem}.allow-referenced{display:flex;align-items:flex-start;gap:8px;line-height:1.8;color:var(--danger,#c54d61)}.allow-referenced input{margin-top:5px;accent-color:var(--accent)}.delete-actions{display:flex;gap:10px;justify-content:flex-end}
</style>
