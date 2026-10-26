<script setup lang="ts">
import {computed,onBeforeUnmount,ref,watch} from 'vue'
import {NButton} from 'naive-ui'
import {Trash2,Folder,FileText,ArrowLeft,RefreshCw} from 'lucide-vue-next'
import ResizableModal from './ResizableModal.vue'
import {api,type TrashEntry} from '../services/api'
import {trashRows,trashTargets,trashParent,type TrashRow,type TrashPreview} from '../services/trash-tree'
const props=defineProps<{show:boolean;workspaceId:string}>()
const emit=defineEmits<{preview:[value:TrashPreview|undefined]}>()
const entries=ref<TrashEntry[]>([]),source=ref('notes'),directory=ref(''),selected=ref<TrashRow>(),error=ref(''),busy=ref(false),loading=ref(false)
const menu=ref<{x:number;y:number;row:TrashRow}>(),confirmation=ref<{row?:TrashRow;targets?:TrashEntry[]}>()
let epoch=0,readEpoch=0
onBeforeUnmount(()=>{epoch++;readEpoch++})
const rows=computed(()=>trashRows(entries.value,source.value,directory.value))
const sourceLabel=computed(()=>source.value==='notes'?'笔记回收':'文件夹回收')
function resetSelection(){selected.value=undefined;readEpoch++;menu.value=undefined;emit('preview',undefined)}
async function load(token=epoch){
 loading.value=true;error.value='';resetSelection()
 try{const result=await api.ListTrash(props.workspaceId);if(token===epoch&&props.show)entries.value=result}
 catch(e){if(token===epoch)error.value=String(e)}finally{if(token===epoch)loading.value=false}
}
watch(()=>[props.show,props.workspaceId] as const,()=>{epoch++;entries.value=[];directory.value='';source.value='notes';confirmation.value=undefined;busy.value=false;loading.value=false;error.value='';resetSelection();if(props.show)void load(epoch)},{immediate:true})
function switchSource(value:string){if(busy.value||loading.value)return;source.value=value;directory.value='';error.value='';resetSelection()}
function goUp(){if(busy.value)return;directory.value=trashParent(directory.value);error.value='';resetSelection()}
async function open(row:TrashRow){
 if(busy.value)return
 resetSelection();error.value=''
 if(row.directory){directory.value=row.path;return}
 if(!row.entry)return
 selected.value=row;const token=epoch,read=++readEpoch,id=props.workspaceId
 const preview:TrashPreview={entry:row.entry,path:row.path,content:'',loading:true,error:''};emit('preview',preview)
 try{const content=await api.ReadTrashFile(id,row.entry.source,row.entry.path);if(token===epoch&&read===readEpoch)emit('preview',{...preview,content,loading:false})}
 catch(e){if(token===epoch&&read===readEpoch){error.value=String(e);emit('preview',{...preview,loading:false,error:String(e)})}}
}
function askDelete(row?:TrashRow){if(busy.value||loading.value||!row)return;menu.value=undefined;confirmation.value={row,targets:trashTargets(entries.value,source.value,row)}}
function context(event:MouseEvent,row:TrashRow){if(busy.value)return;selected.value=row;menu.value={x:Math.max(8,Math.min(event.clientX,window.innerWidth-190)),y:Math.max(8,Math.min(event.clientY,window.innerHeight-65)),row};(event.currentTarget as HTMLElement).focus()}
async function permanentlyDelete(){
 if(!confirmation.value||busy.value)return
 const action=confirmation.value,token=epoch,id=props.workspaceId;busy.value=true;error.value=''
 try{
 if(action.row){for(const entry of action.targets||[])await api.DeleteTrashEntry(id,entry.source,entry.path)}else await api.EmptyTrash(id)
 if(token!==epoch)return
 confirmation.value=undefined;directory.value='';await load(token)
 }catch(e){if(token===epoch){const message=String(e);confirmation.value=undefined;directory.value='';await load(token);error.value=message}}finally{if(token===epoch)busy.value=false}
}
</script>
<template>
 <div v-show="show" class="trash-browser" tabindex="0" @click="menu=undefined" @keydown.delete.prevent.stop="askDelete(selected)" @dragover.prevent.stop @drop.prevent.stop>
  <div class="trash-toolbar"><div class="trash-sources"><NButton :type="source==='notes'?'primary':'default'" :disabled="busy||loading" @click="switchSource('notes')">笔记回收</NButton><NButton :type="source==='folders'?'primary':'default'" :disabled="busy||loading" @click="switchSource('folders')">文件夹回收</NButton></div><div class="trash-actions"><NButton aria-label="刷新回收站" :disabled="busy||loading" @click="load()"><RefreshCw :size="15"/></NButton><NButton type="error" secondary :disabled="busy||loading||!entries.length" @click="confirmation={}"><Trash2 :size="15"/>清空回收站</NButton></div></div>
  <p class="trash-hint">回收站不可迁移。这里删除的文件将彻底删除，无法恢复。</p>
  <div class="trash-location"><button class="icon-button" title="返回回收目录上级" :disabled="!directory||busy" @click="goUp"><ArrowLeft :size="16"/></button><span>{{sourceLabel}}<template v-if="directory"> / {{directory}}</template></span></div>
  <p v-if="error" class="trash-error" role="alert">{{error}}</p>
  <div class="trash-body"><div class="trash-entries" role="list" aria-label="回收文件列表"><p v-if="loading" class="trash-empty">正在读取回收站…</p><p v-else-if="!rows.length" class="trash-empty">此回收目录为空</p><button v-for="row in rows" v-else :key="row.id" class="trash-entry" role="listitem" :class="{selected:selected?.id===row.id}" :draggable="false" :disabled="busy" :title="row.path" @click="open(row)" @contextmenu.prevent.stop="context($event,row)"><component :is="row.directory?Folder:FileText" :size="18"/><span><strong>{{row.name}}</strong><small v-if="row.entry">{{row.entry.size}} 字节 · {{new Date(row.entry.modified).toLocaleString('zh-CN')}}</small></span></button></div></div>
  <div v-if="menu" class="trash-context" role="menu" :style="{left:menu.x+'px',top:menu.y+'px'}"><button role="menuitem" @click.stop="askDelete(menu.row)"><Trash2 :size="15"/>彻底删除</button></div>
 </div>
 <ResizableModal class="trash-confirmation" :show="!!confirmation&&show" :title="confirmation?.row?'彻底删除？':'清空回收站？'" preset="card" style="width:460px;max-width:calc(100vw - 40px)" :closable="!busy" :mask-closable="!busy" :close-on-esc="!busy" @update:show="value=>{if(!value&&!busy)confirmation=undefined}" @minimize="()=>{if(!busy)confirmation=undefined}">
  <p class="trash-confirm-path">{{confirmation?.row?.path||'当前工作区的全部笔记回收和文件夹回收内容'}}</p><p class="trash-hint">{{confirmation?.row?.directory?'此目录下展示的全部回收文件将被永久删除，包括同名文件的不同删除记录。':'所选回收内容将被永久删除。'}}此操作无法撤销，也不会进入 Windows 回收站。回收站本身保留。</p><p v-if="error" role="alert" class="trash-error">{{error}}</p>
  <template #footer><div class="trash-actions"><NButton :disabled="busy" @click="confirmation=undefined">取消</NButton><NButton type="error" :loading="busy" @click="permanentlyDelete">{{confirmation?.row?'确认彻底删除':'确认清空回收站'}}</NButton></div></template>
 </ResizableModal>
</template>
<style scoped>
.trash-browser{color:var(--text);outline:none}.trash-toolbar,.trash-sources,.trash-actions{display:flex;align-items:center;gap:8px;flex-wrap:wrap}.trash-toolbar{justify-content:space-between}.trash-actions{justify-content:flex-end}.trash-hint{font-size:.86rem;line-height:1.7;color:var(--muted);margin:14px 0}.trash-location{display:flex;align-items:center;gap:8px;padding:8px 0;border-bottom:1px solid var(--line);font-size:.86rem;overflow-wrap:anywhere}.trash-body{display:grid;grid-template-columns:minmax(230px,1fr) minmax(200px,1fr);min-height:290px;height:46vh;max-height:60vh}.trash-entries{overflow:auto;border-right:1px solid var(--line);padding:9px 10px 9px 0}.trash-entry{display:flex;gap:9px;align-items:center;width:100%;padding:11px 10px;text-align:left;color:var(--text);border:0;background:transparent;border-radius:7px;cursor:pointer}.trash-entry:hover,.trash-entry.selected{background:var(--soft)}.trash-entry:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}.trash-entry svg{flex-shrink:0;color:var(--muted)}.trash-entry span{min-width:0}.trash-entry strong{display:block;font-weight:500;overflow-wrap:anywhere}.trash-entry small{display:block;color:var(--muted);margin-top:5px}.trash-preview{padding:12px 0 12px 16px;overflow:auto}.trash-preview-heading{display:flex;justify-content:space-between;gap:8px;align-items:center;overflow-wrap:anywhere}.trash-preview pre{white-space:pre-wrap;overflow-wrap:anywhere;font:inherit;font-size:.88rem;line-height:1.8}.trash-empty{color:var(--muted);font-size:.86rem;padding:15px 8px;line-height:1.8}.trash-error{color:var(--danger,#c54d61);overflow-wrap:anywhere}.trash-confirm-path{overflow-wrap:anywhere;color:var(--text)}.trash-context{position:fixed;z-index:4000;background:var(--soft);border:1px solid var(--line);padding:5px;border-radius:8px;box-shadow:0 8px 30px #0003}.trash-context button{display:flex;align-items:center;gap:8px;padding:10px 14px;border:0;border-radius:5px;background:transparent;color:var(--danger,#c54d61);cursor:pointer}.trash-context button:hover{background:var(--accent-bg)}
</style>

<style scoped>
.trash-browser{display:flex;flex-direction:column;flex:1;min-height:0;min-width:0;overflow:auto;padding-bottom:12px}.trash-toolbar{gap:10px;align-items:stretch}.trash-sources{display:grid;grid-template-columns:1fr 1fr;width:100%;gap:6px}.trash-sources :deep(.n-button){padding:0 6px;font-size:.78rem}.trash-toolbar>.trash-actions{justify-content:space-between;width:100%;flex-wrap:nowrap}.trash-toolbar>.trash-actions :deep(.n-button){font-size:.78rem}.trash-hint{font-size:.76rem;margin:10px 0}.trash-location{font-size:.76rem}.trash-location>span{min-width:0;overflow-wrap:anywhere}.trash-body{display:flex;flex:1;min-height:160px;height:auto;max-height:none;flex-direction:column}.trash-entries{flex:1;min-height:120px;border-right:0;padding:8px 0;overflow:auto}.trash-entry{padding:10px 6px;font-size:.82rem}.trash-preview{flex-shrink:0;max-height:42%;overflow:auto;padding:10px 0 0;border-top:1px solid var(--line)}.trash-preview-heading{font-size:.82rem;flex-wrap:wrap}.trash-preview pre{font-size:.8rem}.trash-empty{font-size:.76rem;padding:8px 0}
</style>

<style scoped>.trash-entry strong{font-weight:400}.trash-entry small{font-size:.72rem}.trash-body{min-height:0}.trash-preview{display:none}</style>
