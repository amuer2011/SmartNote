<script setup lang="ts">
import {computed,onBeforeUnmount,ref,watch} from 'vue'
import {api,type IndexStatus} from '../services/api'
import {useWorkspace} from '../stores/workspace'
const props=defineProps<{workspaceId:string}>(),store=useWorkspace()
const status=ref<IndexStatus>(),error=ref(''),busy=ref(false)
let timer:ReturnType<typeof setTimeout>|undefined,sequence=0,alive=true
const current=computed(()=>status.value?.workspaceId===props.workspaceId?status.value:undefined)
const running=computed(()=>['queued','running','cancelling'].includes(current.value?.state||''))
const labels:Record<string,string>={idle:'尚未建立',queued:'等待中',running:'建立中',cancelling:'正在停止',cancelled:'已停止',ready:'已就绪',failed:'未完成'}
async function poll(token:number){
 try{const result=await api.IndexStatus();if(alive&&token===sequence){status.value=result;error.value=''}}catch(e){if(alive&&token===sequence)error.value='无法读取索引状态：'+String(e)}
 if(alive&&token===sequence)timer=setTimeout(()=>void poll(token),1000)
}
watch(()=>props.workspaceId,()=>{clearTimeout(timer);status.value=undefined;error.value='';const token=++sequence;if(props.workspaceId)void poll(token)},{immediate:true})
async function action(kind:'update'|'rebuild'|'cancel'){
 if(busy.value)return
 const workspace=props.workspaceId,token=sequence;busy.value=true;error.value=''
 try{
  if(kind!=='cancel'){await store.save();if(store.dirty)throw new Error('请先处理当前笔记保存冲突')}
  if(workspace!==props.workspaceId)return
  const result=kind==='cancel'?await api.CancelIndex():await api.StartIndex(kind==='rebuild')
  if(alive&&token===sequence)status.value=result
 }catch(e){if(alive&&token===sequence)error.value=String(e)}finally{busy.value=false}
}
onBeforeUnmount(()=>{alive=false;sequence++;clearTimeout(timer)})
</script>
<template><section class="index-settings" aria-label="后台检索索引"><div class="setting-row"><div><strong>后台检索索引</strong><p>打开工作区后自动准备，笔记变化时增量更新。</p></div><span class="status-pill">{{labels[current?.state||'idle']||'等待中'}}</span></div><template v-if="workspaceId"><p role="status">{{current?.message||'等待当前工作区索引状态…'}}</p><div v-if="current?.total" class="index-progress"><progress :value="current.completed" :max="current.total" aria-label="段落索引进度"/><span>{{current.completed}} / {{current.total}} 段落 · {{current.notes}} 篇笔记</span></div><div class="index-actions"><button class="secondary-button" :disabled="busy||running" @click="action('update')">{{current?.state==='cancelled'||current?.state==='failed'?'继续更新':'更新索引'}}</button><button class="secondary-button" :disabled="busy||running" @click="action('rebuild')">重新建立索引</button><button v-if="running" class="text-button" :disabled="busy||current?.state==='cancelling'" @click="action('cancel')">停止索引</button></div><p class="field-help">索引在本地运行，可继续编辑。停止会等待当前小批次结束，并保留已完成部分；重新建立只更新检索索引，不修改笔记和附件。</p><p v-if="error" class="bookmark-error" role="alert">{{error}}</p></template><p v-else class="muted">请先打开工作区。</p></section></template>
<style scoped>.index-settings{margin-top:22px;border-top:1px solid var(--line)}.index-settings>p{font-size:.85rem;color:var(--secondary)}.index-progress{display:flex;flex-direction:column;gap:7px;margin:12px 0;color:var(--muted);font-size:.8rem}progress{width:100%;height:9px;accent-color:var(--accent)}.index-actions{display:flex;flex-wrap:wrap;align-items:center;gap:10px;margin:15px 0}.index-actions button:disabled{opacity:.45;cursor:default}</style>
