<script setup lang="ts">
import {computed,onBeforeUnmount,ref,watch} from 'vue'
import ResizableModal from './ResizableModal.vue'
import {api,type SemanticConfig,type SemanticCategory,type DirectoryLearningPreview} from '../services/api'
const props=defineProps<{show:boolean;workspaceId:string;category:SemanticCategory;revision:string}>()
const emit=defineEmits<{'update:show':[value:boolean];saved:[workspaceId:string,config:SemanticConfig]}>()
const preview=ref<DirectoryLearningPreview>(),selected=ref<string[]>([]),query=ref(''),loading=ref(false),saving=ref(false),error=ref('')
let sequence=0
const visible=computed(()=>preview.value?.notes.filter(n=>(n.title+' '+n.path).toLowerCase().includes(query.value.toLowerCase()))||[])
const signature=computed(()=>JSON.stringify([props.show,props.workspaceId,props.category.id,props.revision]))
async function load(){
 const request=++sequence,id=props.workspaceId;preview.value=undefined;selected.value=[];query.value='';error.value=''
 if(!props.show)return
 loading.value=true
 try{
  const result=await api.PreviewDirectoryLearning(id,props.category.id)
  if(request!==sequence||id!==props.workspaceId)return
  preview.value=result
  selected.value=result.notes.filter(n=>props.category.examples?.some(r=>r.noteId===n.noteId&&r.hash===n.hash&&r.path===n.path)).map(n=>n.noteId)
 }catch(e){if(request===sequence)error.value=String(e)}finally{if(request===sequence)loading.value=false}
}
watch(signature,()=>void load(),{immediate:true})
function close(){if(!saving.value)emit('update:show',false)}
function selectVisible(){selected.value=visible.value.slice(0,20).map(n=>n.noteId)}
async function save(clear=false){
 if(saving.value||loading.value||!preview.value)return
 const id=props.workspaceId,categoryID=props.category.id,revision=props.revision,request=sequence
 const refs=clear?[]:preview.value.notes.filter(n=>selected.value.includes(n.noteId)).map(({noteId,hash,path})=>({noteId,hash,path}))
 if(!clear&&!refs.length){error.value='请选择至少一篇样本，或使用清除学习。';return}
 saving.value=true;error.value=''
 try{
  const config=await api.GetSemanticConfig(id)
  if(request!==sequence||id!==props.workspaceId)return
  if(config.revision!==revision)throw new Error('分类设置已变化，请关闭后重新打开学习窗口。')
  const category=config.categories.find(c=>c.id===categoryID)
  if(!category)throw new Error('分类已不存在，请重新加载设置。')
  category.examples=refs
  const result=await api.SaveSemanticConfig(id,config)
  if(request!==sequence||id!==props.workspaceId)return
  emit('saved',id,result);emit('update:show',false)
 }catch(e){if(request===sequence)error.value=String(e)}finally{if(request===sequence)saving.value=false}
}
onBeforeUnmount(()=>sequence++)
</script>
<template>
 <ResizableModal :show="show" preset="card" title="学习分类目录" style="width:720px" :mask-closable="!saving" :close-on-esc="!saving" :closable="!saving" @update:show="close" @minimize="close">
  <section class="directory-learning">
   <h3>{{category.name}} <span>{{preview?.directory||category.directory}}</span></h3>
   <p>选择最能代表这个分类的笔记，最多20篇。后续分类会将样本语义与分类说明合成原型，仍使用设置中的余弦相似度门槛。</p>
   <p>只列出该目录直属笔记，排除已完成待办归档。不会修改或移动笔记，也不会自动把新归档的笔记加入样本。</p>
   <p v-if="loading" role="status">正在读取目录样本…</p>
   <p v-if="error" role="alert" class="learning-error">{{error}}</p>
   <template v-if="preview">
    <div class="learning-tools"><input v-model="query" aria-label="筛选学习样本" placeholder="按标题或路径筛选样本"/><button class="secondary-button" :disabled="saving||!visible.length" @click="selectVisible">选择前20篇</button><button class="text-button" :disabled="saving" @click="selected=[]">取消选择</button></div>
    <p role="status">已选 {{selected.length}} / 20 篇 · 目录共 {{preview.total}} 篇可用笔记<span v-if="preview.total>preview.notes.length">，当前展示 {{preview.notes.length}} 篇（最近更新及已保存样本）</span></p>
    <div class="learning-samples"><label v-for="note in visible" :key="note.noteId"><input v-model="selected" type="checkbox" :value="note.noteId" :disabled="saving||selected.length>=20&&!selected.includes(note.noteId)" :aria-label="'选择样本 '+note.title"/><span><strong>{{note.title}}</strong><small>{{note.path}}</small></span></label><p v-if="!visible.length">{{preview.total?'没有匹配的样本。':'此目录暂无可学习的直属笔记。'}}</p></div>
   </template>
   <p class="learning-help">确认后保存样本及其版本，后续分类自动使用。未修改的已确认样本保持原目录；样本修改、移动或删除后将被忽略，可再次学习更新。分析样本自身时排除该篇，避免自我匹配。清除学习后恢复仅参考分类说明。</p>
  </section>
  <template #footer><div class="learning-actions"><button class="text-button" :disabled="saving||loading||!category.examples?.length" @click="save(true)">清除学习</button><button class="secondary-button" :disabled="saving" @click="close">取消</button><button class="primary-button" :disabled="saving||loading||!selected.length" @click="save()">{{saving?'保存中…':'确认学习样本'}}</button></div></template>
 </ResizableModal>
</template>
<style scoped>
.directory-learning h3{font-size:1rem;font-weight:500}.directory-learning h3 span{display:block;color:var(--muted);font-size:.8rem;margin-top:8px}.directory-learning p{color:var(--muted);font-size:.82rem;line-height:1.9}.directory-learning .learning-error{color:#cf5555;overflow-wrap:anywhere}.learning-tools,.learning-actions{display:flex;gap:10px;align-items:center;flex-wrap:wrap}.learning-tools>input{flex:1;min-width:160px;border:1px solid var(--line);border-radius:7px;background:var(--soft);color:var(--text);padding:9px;font:inherit}.learning-tools button{font-size:.8rem;padding:8px 10px}.learning-samples{max-height:330px;overflow:auto;border:1px solid var(--line);border-radius:8px;padding:3px 12px}.learning-samples>label{display:flex;gap:12px;align-items:flex-start;padding:12px 0;border-bottom:1px solid var(--line)}.learning-samples label:last-child{border-bottom:0}.learning-samples input{margin-top:4px;accent-color:var(--accent)}.learning-samples label>span{min-width:0}.learning-samples strong{font-weight:500;font-size:.88rem;overflow-wrap:anywhere;display:block}.learning-samples small{display:block;font-size:.76rem;color:var(--muted);margin-top:6px;overflow-wrap:anywhere}.learning-help{margin-bottom:0}.learning-actions{justify-content:flex-end}.learning-actions>.text-button{margin-right:auto}
</style>
