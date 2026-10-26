<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton } from 'naive-ui'
import ResizableModal from './ResizableModal.vue'
import DirectoryPicker from './DirectoryPicker.vue'
import { api, type Note, type ArchiveSuggestion, type ArchiveResult, type SemanticConfig } from '../services/api'
import { waitForSemanticSettings } from '../services/semantic-settings'
import { useWorkspace } from '../stores/workspace'

const props=defineProps<{show:boolean;workspaceId:string;note:Note|null;directories:string[];configRevision?:number}>()
const emit=defineEmits<{'update:show':[value:boolean];minimize:[];applied:[result:ArchiveResult]}>()
const store=useWorkspace()
const suggestion=ref<ArchiveSuggestion>(),directory=ref('.'),selectedTags=ref<string[]>([]),extraTags=ref(''),error=ref(''),loading=ref(false),applying=ref(false)
const semantic=ref(true)
const semanticConfig=ref<SemanticConfig>()
let sequence=0
function reasonParts(reason:string){
 const pattern=/(「[^」]+」|-?\d+(?:\.\d+)?|未分类原因|相似度不足|最低余弦相似度|保留原目录|建议|调至|或更低|降低门槛会增加[^，。；]*)/g
 return reason.split(pattern).filter(Boolean).map(text=>({text,highlight:/^(?:「[^」]+」|-?\d+(?:\.\d+)?|未分类原因|相似度不足|最低余弦相似度|保留原目录|建议|调至|或更低|降低门槛会增加[^，。；]*)$/.test(text)}))
}
const key=computed(()=>`${props.workspaceId}:${props.note?.id}:${props.note?.hash}`)
async function load() {
 if(!props.show || !props.note || !props.workspaceId) return
 const request=++sequence,requestKey=key.value
 loading.value=true;error.value='';suggestion.value=undefined;semanticConfig.value=undefined
 try {
  if(semantic.value){
   await waitForSemanticSettings(props.workspaceId)
   if(request!==sequence || requestKey!==key.value)return
   const config=await api.GetSemanticConfig(props.workspaceId)
   if(request!==sequence || requestKey!==key.value)return
   semanticConfig.value=config
  }
  const value=await (semantic.value?api.SuggestSemanticArchive:api.SuggestArchive)(props.workspaceId,props.note.id,props.note.hash)
  if(request!==sequence || requestKey!==key.value) return
  suggestion.value=value;directory.value=value.directory;selectedTags.value=[...value.tags];extraTags.value=''
 } catch(e) {if(request===sequence) error.value=String(e)} finally {if(request===sequence) loading.value=false}
}
watch([()=>props.show,key,()=>props.configRevision],()=>{if(props.show) void load();else {sequence++;loading.value=false}},{immediate:true})
const allTags=computed(()=>[...new Set([...selectedTags.value,...extraTags.value.split(/[,，]/).map(tag=>tag.trim()).filter(Boolean)])])
const targetPath=computed(()=>[directory.value.trim().replace(/\\/g,'/').replace(/^\.\/?$/,''),props.note?.path.split('/').pop()].filter(Boolean).join('/'))
const changed=computed(()=>!!props.note && (targetPath.value!==props.note.path || allTags.value.some(tag=>!props.note!.tags.some(old=>old.toLowerCase()===tag.toLowerCase()))))
async function apply() {
 const value=suggestion.value
 if(!value || applying.value || loading.value) return
 applying.value=true;error.value=''
 try {
  await store.save()
  if(store.dirty) throw new Error('请先处理未保存的修改')
  if(props.workspaceId!==value.workspaceId || store.workspace?.id!==value.workspaceId || store.active?.id!==value.noteId || store.active.hash!==value.hash) throw new Error('笔记或工作区已变化，请重新获取建议')
  const result=await api.ApplyArchive({workspaceId:value.workspaceId,noteId:value.noteId,hash:value.hash,directory:directory.value,tags:allTags.value})
  emit('applied',result);emit('update:show',false)
 } catch(e) {error.value=String(e)} finally {applying.value=false}
}
</script>
<template>
 <ResizableModal :show="show" preset="card" title="智能整理" style="width:650px" :mask-closable="!applying" :close-on-esc="!applying" @update:show="emit('update:show',$event)" @minimize="emit('minimize')">
  <p class="archive-help">可选择 Embedding 语义分类或关键词规则。手动确认前不会移动文件或修改标签。</p>
  <label class="archive-help"><input v-model="semantic" type="checkbox" :disabled="loading || applying" @change="load"/>使用内置模型语义分类</label>
  <p v-if="loading" role="status">正在分析当前笔记…</p>
  <p v-if="error" class="archive-error" role="alert">{{error}}</p>
  <template v-if="suggestion && !loading">
   <strong>{{note?.title}}</strong>
   <p v-if="semantic && semanticConfig" class="archive-help" role="status">当前配置：最低余弦相似度 <strong class="archive-highlight">{{semanticConfig.threshold}}</strong>。与设置中的已保存配置同步，门槛修改后自动保存并重新分析；每次打开自动获取最新建议。</p>
   <ul class="archive-reasons"><li v-for="reason in suggestion.reasons" :key="reason"><span v-for="(part,index) in reasonParts(reason)" :key="index" :class="{'archive-highlight':part.highlight}">{{part.text}}</span></li></ul>
   <label class="field-label">目标目录（可手动修改）</label><DirectoryPicker v-model="directory" :directories="directories"/>
   <div v-if="suggestion.tags.length" class="archive-tags"><span class="field-label">建议新增标签</span><label v-for="tag in suggestion.tags" :key="tag"><input v-model="selectedTags" type="checkbox" :value="tag" :aria-label="'采用标签 '+tag"/>{{tag}}</label></div>
   <label class="field-label" for="archive-extra-tags">其他新增标签（逗号分隔）</label><input id="archive-extra-tags" v-model="extraTags" class="archive-text" aria-label="其他新增标签"/>
   <p class="archive-help">已有标签会保留。Front Matter 更新时可能调整 YAML 排版，正文保持原有内容；移动会同步更新链接。</p>
   <div class="archive-preview"><span>原文件：{{note?.path}}</span><strong>整理后：{{targetPath}}</strong><span v-if="allTags.length">新增标签：{{allTags.join('、')}}</span></div>
   <p v-if="!changed" class="archive-help">目录和标签均未变化，无需整理。</p>
  </template>
  <template #footer><div class="archive-actions"><NButton :disabled="applying || loading" @click="load">重新获取建议</NButton><NButton :disabled="applying" @click="emit('update:show',false)">取消</NButton><NButton type="primary" :loading="applying" :disabled="loading || !suggestion || !changed" @click="apply">确认整理</NButton></div></template>
 </ResizableModal>
</template>
<style scoped>
.archive-highlight{color:#d03050;font-weight:700;background:rgba(208,48,80,.08);border-radius:3px;padding:0 2px;overflow-wrap:anywhere}:global([data-theme="dark"]) .archive-highlight{color:#ff899b;background:rgba(255,137,155,.1)}

.archive-help{font-size:.857rem;color:var(--muted);line-height:1.8;margin:12px 0}.archive-error{color:#cf5555;overflow-wrap:anywhere}.archive-reasons{font-size:.9rem;line-height:1.9;padding-left:20px;margin:15px 0}.archive-tags{display:flex;gap:12px;flex-wrap:wrap;align-items:center}.archive-tags>.field-label{width:100%}.archive-tags label{display:flex;align-items:center;gap:5px}.archive-text{width:100%;background:var(--soft);border:1px solid var(--line);border-radius:5px;padding:9px;color:var(--text);font:inherit;box-sizing:border-box}.archive-preview{display:flex;flex-direction:column;gap:9px;background:var(--soft);border:1px solid var(--line);padding:14px;border-radius:6px;overflow-wrap:anywhere;font-size:.857rem}.archive-actions{display:flex;gap:8px;justify-content:flex-end;flex-wrap:wrap}
</style>
