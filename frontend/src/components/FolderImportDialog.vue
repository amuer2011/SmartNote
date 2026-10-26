<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {NButton} from 'naive-ui'
import {FolderOpen,FolderTree,Upload,Sparkles,CheckCircle2,AlertCircle} from 'lucide-vue-next'
import ResizableModal from './ResizableModal.vue'
import {api,type FolderImportResult} from '../services/api'
import {useWorkspace} from '../stores/workspace'

const props=defineProps<{show:boolean}>(),emit=defineEmits<{'update:show':[value:boolean];zip:[]}>()
const store=useWorkspace(),source=ref(''),organize=ref(false)
const choosing=ref(false),copying=ref(false),organizing=ref(false),stopping=ref(false),stopped=ref(false)
const result=ref<FolderImportResult>(),error=ref(''),current=ref(''),done=ref(0),total=ref(0),moved=ref(0),kept=ref(0),failed=ref(0)
const details=ref<string[]>([])
let generation=0
const busy=computed(()=>choosing.value||copying.value||organizing.value)
const percent=computed(()=>total.value?Math.round(done.value/total.value*100):0)
function close(){if(!busy.value)emit('update:show',false)}
function report(text:string){if(details.value.length<100)details.value.push(text)}
watch(()=>props.show,value=>{
 if(!value){generation++;return}
 source.value='';organize.value=false;result.value=undefined;error.value=''
 choosing.value=false;copying.value=false;organizing.value=false;stopping.value=false;stopped.value=false
 current.value='';done.value=0;total.value=0;moved.value=0;kept.value=0;failed.value=0;details.value=[]
})
watch(()=>store.workspace?.id,()=>{
 if(!props.show||copying.value)return
 generation++;stopping.value=true;organizing.value=false;emit('update:show',false)
})
async function choose(){
 if(busy.value)return
 choosing.value=true;error.value=''
 const token=generation
 try{
  const path=await api.ChooseImportFolder()
  if(token!==generation)return
  source.value=path
 }catch(e){if(token===generation&&!String(e).includes('已取消'))error.value=String(e)}finally{if(token===generation)choosing.value=false}
}
async function start(){
 if(busy.value||!source.value||result.value)return
 const token=++generation
 copying.value=true;error.value=''
 try{
  const imported=await store.importFolder({source:source.value,organize:organize.value,templateWorkspaceId:store.workspace?.id||''})
  if(token!==generation)return
  if(!imported){error.value=store.error||'当前笔记尚未保存，请处理后重新导入。';return}
  result.value=imported;details.value=[...(imported.warnings||[])];copying.value=false
  if(organize.value)await organizeNotes(imported.workspace.id,token)
 }catch(e){if(token===generation)error.value=String(e)}finally{if(token===generation)copying.value=false}
}
async function organizeNotes(workspaceID:string,token:number){
 organizing.value=true;stopping.value=false
 const queue=store.data.notes.filter(note=>!note.taskArchived&&!note.automationIgnored).map(note=>note.id)
 total.value=queue.length
 try{
  for(const id of queue){
   if(stopping.value||token!==generation||store.workspace?.id!==workspaceID)break
   const note=store.data.notes.find(note=>note.id===id)
   if(!note){kept.value++;done.value++;continue}
   current.value=note.title
   try{
    const archived=await api.AutoArchive(workspaceID,note.id,note.hash)
    if(token!==generation||store.workspace?.id!==workspaceID)return
    if(archived?.operationId){moved.value++;report(note.title+' → '+archived.note.path);await store.refresh()}
    else kept.value++
   }catch(e){
    if(token!==generation||store.workspace?.id!==workspaceID)return
    failed.value++;report(note.title+'：'+String(e))
    // A model/configuration failure should not trigger thousands of identical calls.
    const message=String(e)
    if(!message.includes('目标')&&!message.includes('CONFLICT')&&!message.includes('已存在')){error.value='已停止整理：'+message;stopping.value=true}
   }
   done.value++
  }
 }finally{
  if(token===generation){stopped.value=done.value<total.value;organizing.value=false;current.value='';await store.refresh()}
 }
}
function zip(){if(busy.value)return;emit('update:show',false);emit('zip')}
</script>

<template>
 <ResizableModal class="folder-import-modal" :show="show" preset="card" title="导入笔记文件夹" style="width:660px;max-width:calc(100vw - 32px)" :closable="!busy" :mask-closable="!busy" :close-on-esc="!busy" @update:show="value=>{if(!value)close()}" @minimize="close">
  <section class="folder-import" aria-label="文件夹导入">
   <header class="folder-import-intro"><span><FolderOpen :size="25"/></span><div><strong>打开已准备好的笔记文件夹</strong><p>支持 Obsidian 和通用 .md 笔记。直接使用所选目录，后续编辑和整理在该目录中生效。</p></div></header>
   <template v-if="!result">
    <label class="field-label">笔记文件夹</label><div class="import-path"><span :class="{empty:!source}" :title="source">{{source||'选择你已拷贝好的笔记文件夹'}}</span><button class="secondary-button" :disabled="busy" aria-label="选择笔记文件夹" @click="choose"><FolderOpen :size="15"/>选择文件夹</button></div>
    <div class="import-strategies" role="radiogroup" aria-label="导入整理方式">
     <label :class="{selected:!organize}"><input v-model="organize" type="radio" :value="false" :disabled="busy"/><FolderTree :size="20"/><span><strong>保持原目录结构</strong><small>保留笔记及附件目录，并关闭自动整理。</small></span></label>
     <label :class="{selected:organize}"><input v-model="organize" type="radio" :value="true" :disabled="busy"/><Sparkles :size="20"/><span><strong>导入后自动整理</strong><small>按语义分类移动笔记并更新链接，可停止本次整理。</small></span></label>
    </div>
    <p class="field-help">自动整理使用现有分类与相似度阈值，未达阈值保留原目录，遇到同名文件不会覆盖。以后可在智能归档设置中调整。</p>
    <p class="field-help">图片和其他附件会加入附件库并适配本地引用。隐藏配置目录（如 .obsidian）保持原样，不加载插件。</p>
   </template>
   <template v-else>
    <div class="import-success"><CheckCircle2 :size="23"/><div><strong>{{result.workspace.name}} 已导入</strong><p>{{result.notes}} 篇笔记 · {{result.attachments}} 个附件 · 跳过 {{result.skipped}} 项</p><small>{{result.workspace.path}}</small></div></div>
    <div v-if="organize" class="import-progress" role="status" aria-live="polite"><strong>{{organizing?'正在自动整理':stopped?'本次整理已停止':'自动整理已完成'}}</strong><span>{{done}} / {{total}}</span><progress :value="done" :max="total||1" :aria-label="'整理进度 '+percent+'%'"/><small v-if="current">{{current}}</small><p>已移动 {{moved}} · 保持原位 {{kept}} · 未完成 {{failed}}</p><small v-if="stopped">未处理的笔记保留原目录，已完成的移动不撤销。</small></div>
    <p v-else class="field-help">已保留原目录结构，自动整理已关闭。可以直接浏览笔记；需要时再到智能归档设置开启。</p>
    <details v-if="details.length" class="import-details"><summary><AlertCircle :size="14"/>导入与整理明细（{{details.length}} 条{{result.warningCount>50?'，附件提示仅列前50条':''}}）</summary><ul><li v-for="(detail,index) in details" :key="index">{{detail}}</li></ul></details>
   </template>
   <p v-if="copying" class="field-help" role="status">正在加载笔记和附件，请稍候…</p>
   <p v-if="error" class="bookmark-error" role="alert">{{error}}</p>
  </section>
  <template #footer><div class="import-footer"><button class="text-button" :disabled="busy" @click="zip"><Upload :size="14"/>改用 ZIP 导入</button><div><NButton v-if="organizing" :disabled="stopping" @click="stopping=true">{{stopping?'正在停止…':'停止本次整理'}}</NButton><NButton v-else :disabled="busy" @click="close">{{result?'完成':'取消'}}</NButton><NButton v-if="!result" type="primary" :loading="copying" :disabled="busy||!source" @click="start">{{copying?'正在导入':'开始导入'}}</NButton></div></div></template>
 </ResizableModal>
</template>

<style scoped>
.folder-import-intro{display:flex;gap:14px;align-items:flex-start;margin:0 0 22px}.folder-import-intro>span{display:grid;place-items:center;background:var(--accent-bg);color:var(--accent);padding:12px;border-radius:12px;flex-shrink:0}.folder-import-intro strong{font-size:1.05rem}.folder-import-intro p{color:var(--secondary);line-height:1.8;margin:5px 0 0;font-size:.85rem}.import-path{display:flex;align-items:center;gap:10px;padding:9px 10px;border:1px solid var(--line);border-radius:8px;background:var(--soft);margin-bottom:16px;min-width:0}.import-path>span{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:.84rem}.import-path>span.empty{color:var(--muted)}.import-path>button{flex-shrink:0}.import-strategies{display:flex;flex-direction:column;gap:10px;margin:18px 0 12px}.import-strategies label{display:flex;align-items:center;gap:12px;padding:14px;border:1px solid var(--line);border-radius:8px;cursor:pointer}.import-strategies label.selected{background:var(--accent-bg);border-color:var(--accent)}.import-strategies input{accent-color:var(--accent);flex-shrink:0}.import-strategies svg{color:var(--accent);flex-shrink:0}.import-strategies strong,.import-strategies small{display:block}.import-strategies strong{font-size:.9rem}.import-strategies small{font-size:.77rem;line-height:1.8;color:var(--secondary);margin-top:3px}.import-success{display:flex;align-items:flex-start;gap:12px;background:var(--accent-bg);border-radius:9px;padding:16px;margin-bottom:18px}.import-success>svg{color:var(--accent);flex-shrink:0}.import-success p{margin:6px 0;color:var(--secondary);font-size:.84rem}.import-success small{color:var(--muted);overflow-wrap:anywhere}.import-progress{display:flex;flex-wrap:wrap;gap:8px;align-items:center;border:1px solid var(--line);border-radius:8px;padding:15px}.import-progress>span{margin-left:auto;color:var(--muted)}.import-progress progress{width:100%;height:8px;accent-color:var(--accent)}.import-progress p,.import-progress small{width:100%;margin:4px 0;color:var(--secondary);font-size:.8rem;overflow-wrap:anywhere}.import-details{margin-top:16px;font-size:.8rem}.import-details summary{display:flex;align-items:center;gap:7px;cursor:pointer;color:var(--secondary)}.import-details ul{max-height:180px;overflow:auto;padding-left:22px;line-height:1.8;color:var(--secondary)}.import-details li{overflow-wrap:anywhere;margin:5px 0}.import-footer{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap}.import-footer>div{display:flex;gap:8px}.import-footer>.text-button{display:flex;align-items:center;gap:6px;font-size:.8rem}.folder-import .field-help{line-height:1.8}
</style>
