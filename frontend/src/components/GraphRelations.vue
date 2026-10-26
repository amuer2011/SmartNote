<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {NSelect} from 'naive-ui'
import {X} from 'lucide-vue-next'
import {api,type Edge,type Note,type GraphAnnotations} from '../services/api'
const props=defineProps<{workspaceId?:string;notes:Note[];edges:Edge[];annotations:GraphAnnotations;selected?:string;inspected?:Edge;node?:string;pair?:{source:string;target:string}}>()
const emit=defineEmits<{changed:[];open:[id:string];close:[]}>()
const source=ref<string>(),target=ref<string>(),saving=ref(false),error=ref('')
const noteMap=computed(()=>new Map(props.notes.map(n=>[n.id,n])))
const options=computed(()=>props.notes.map(n=>({label:n.title+' · '+n.path,value:n.id})))
const targetOptions=computed(()=>options.value.filter(n=>n.value!==source.value))
const pairKey=(a:string,b:string)=>JSON.stringify([a,b].sort())
const relation=computed(()=>props.edges.find(e=>source.value&&target.value&&pairKey(e.source,e.target)===pairKey(source.value,target.value)))
const hasManual=computed(()=>props.edges.some(e=>e.type==='USER_CONFIRMED'&&source.value&&target.value&&pairKey(e.source,e.target)===pairKey(source.value,target.value)))
watch(()=>[props.inspected,props.node,props.pair],()=>{
 error.value=''
 source.value=props.pair?.source||props.inspected?.source||props.selected
 target.value=props.pair?.target||props.inspected?.target
},{immediate:true})
async function save(){
 const workspace=props.workspaceId;if(!workspace||saving.value)return
 saving.value=true;error.value=''
 try{
  {
   const a=noteMap.value.get(source.value||''),b=noteMap.value.get(target.value||'');if(!a||!b||a.id===b.id)throw Error('请选择两篇不同的笔记')
   // The link is confirmed by this explicit save; no model inference is involved.
   if(!relation.value)await api.SetGraphRelation({workspaceId:workspace,source:a.id,target:b.id,sourceHash:a.hash,targetHash:b.hash,confirmed:true})
  }
  if(workspace===props.workspaceId)emit('changed')
 }catch(e){if(workspace===props.workspaceId)error.value=String(e)}finally{saving.value=false}
}
async function remove(){
 const a=noteMap.value.get(source.value||''),b=noteMap.value.get(target.value||''),workspace=props.workspaceId
 if(!a||!b||!workspace||saving.value)return
 saving.value=true;error.value=''
 try{await api.SetGraphRelation({workspaceId:workspace,source:a.id,target:b.id,sourceHash:a.hash,targetHash:b.hash,confirmed:false});if(workspace===props.workspaceId)emit('changed')}catch(e){error.value=String(e)}finally{saving.value=false}
}
</script>
<template>
 <aside class="graph-relations" aria-label="拓扑关联与备注">
  <header><strong>手动关联</strong><button class="icon-button" aria-label="关闭关联与备注" @click="emit('close')"><X :size="17"/></button></header>
  <label>来源笔记</label><NSelect v-model:value="source" :options="options" filterable aria-label="关联来源笔记" placeholder="选择来源笔记"/><label>关联笔记</label><NSelect v-model:value="target" :options="targetOptions" filterable aria-label="关联目标笔记" placeholder="选择关联笔记"/>
  <p class="help">{{relation?.type==='MARKDOWN_LINK'?'此连线来自笔记中的链接。删除链接请编辑原笔记。':'在图上依次点击两篇笔记，或在这里选择笔记建立关联。'}}</p><p class="help">编辑备注：在图上选中节点或连线，按空格键。</p>
  <p v-if="error" class="error" role="alert">{{error}}</p>
  <div class="actions"><button v-if="hasManual" class="secondary-button" :disabled="saving" @click="remove">删除手动关联</button><button class="primary-button" :disabled="saving||(!source||!target||source===target)" @click="save">{{saving?'保存中…':'保存关联'}}</button></div>
 </aside>
</template>
<style scoped>
.graph-relations{width:300px;max-width:42%;min-width:230px;min-height:0;overflow:auto;border-left:1px solid var(--line);padding:12px 0 12px 18px;color:var(--secondary);font-size:.85rem}.graph-relations header,.actions{display:flex;align-items:center;justify-content:space-between;gap:10px}.graph-relations header strong{color:var(--text);font-size:1rem}.graph-relations h3{margin:18px 0 8px;overflow-wrap:anywhere;font-size:1rem}.graph-relations label{display:block;margin:18px 0 9px}.help{color:var(--muted);font-size:.8rem;line-height:1.8;margin:12px 0;overflow-wrap:anywhere}.error{color:var(--danger,#c35b69);overflow-wrap:anywhere;line-height:1.8}.actions{margin-top:18px;flex-wrap:wrap;justify-content:flex-end}.actions button{font-size:.8rem;padding:7px 10px}.text-button{display:inline-flex;gap:4px;align-items:center}
</style>
