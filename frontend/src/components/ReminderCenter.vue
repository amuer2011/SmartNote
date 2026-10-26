<script setup lang="ts">
import {onBeforeUnmount,ref,watch} from 'vue'
import {Bell} from 'lucide-vue-next'
import FloatingWindow from './FloatingWindow.vue'
import {api,type Task} from '../services/api'
import {useWorkspace} from '../stores/workspace'
const props=defineProps<{workspaceId:string;revision:number}>(),emit=defineEmits<{open:[task:Task]}>(),store=useWorkspace()
const show=ref(false),items=ref<Task[]>([]),error=ref(''),busy=ref(false)
let timer:ReturnType<typeof setTimeout>|undefined,debounce:ReturnType<typeof setTimeout>|undefined,serial=0,alive=true
const seen=new Set<string>()
function key(task:Task){return task.id+':'+task.dueAt+':'+new Date().toLocaleDateString()}
async function refresh(){const id=props.workspaceId,token=++serial;if(!id)return;try{const result=await api.DueReminders(id);if(!alive||token!==serial||id!==props.workspaceId)return;items.value=result||[];error.value='';let fresh=false;for(const task of items.value){if(!seen.has(key(task))){seen.add(key(task));fresh=true}};if(fresh)show.value=true}catch(e){if(alive&&token===serial)error.value=String(e)}}
async function poll(){await refresh();if(alive)timer=setTimeout(()=>void poll(),30000)}
watch(()=>props.workspaceId,()=>{serial++;items.value=[];seen.clear();show.value=false;error.value='';void refresh()})
watch([()=>props.revision,()=>store.data.tasks.map(t=>t.id+':'+t.done+':'+t.dueAt+':'+t.candidate).join('|')],()=>{clearTimeout(debounce);debounce=setTimeout(()=>void refresh(),500)})
void poll()
async function act(task:Task,action:'dismiss'|'snooze'|'complete',minutes=0){if(busy.value)return;const id=props.workspaceId;busy.value=true;error.value='';try{await store.save();if(store.dirty)throw new Error('请先处理未保存的笔记修改');if(id!==props.workspaceId)return;if(action==='complete'){if(task.recurrence)await api.SetTaskOccurrence(id,task.id,task.dueAt,true);else await api.SetTask(task.id,true,false);await store.refresh()}else{await api.ActOnReminder({workspaceId:id,taskId:task.id,dueAt:task.dueAt,action,minutes})};seen.delete(key(task));items.value=items.value.filter(t=>t.id!==task.id);await refresh()}catch(e){error.value=String(e)}finally{busy.value=false}}
function open(task:Task){emit('open',task);show.value=false}
onBeforeUnmount(()=>{alive=false;serial++;clearTimeout(timer);clearTimeout(debounce)})
</script>
<template><button class="icon-button reminder-bell" aria-label="打开任务提醒" :title="`任务提醒（${items.length}）`" :disabled="!workspaceId" @click="show=true;refresh()"><Bell :size="17"/><span v-if="items.length" class="reminder-count">{{items.length}}</span></button><Teleport to="body"><FloatingWindow :show="show" title="任务提醒" :initial-width="500" :initial-height="440" @close="show=false" @minimize="show=false"><section class="reminder-content"><p class="muted small">{{items.length}} 项到期或即将到期任务 · 当前工作区</p><p v-if="error" class="bookmark-error" role="alert">{{error}}</p><article v-for="task in items" :key="task.id" class="reminder-card"><strong>{{task.title}}</strong><p class="muted small">截止 {{task.dueAt}}<span v-if="task.priority==='high'"> · 高优先级</span></p><button class="text-button" :disabled="busy" @click="open(task)">打开来源：{{task.noteTitle}}</button><div class="reminder-actions"><button class="secondary-button" :disabled="busy" @click="act(task,'complete')">完成任务</button><button class="text-button" :disabled="busy" @click="act(task,'snooze',15)">15 分钟后</button><button class="text-button" :disabled="busy" @click="act(task,'snooze',60)">1 小时后</button><button class="text-button" :disabled="busy" @click="act(task,'dismiss')">今日忽略</button></div></article><p v-if="!items.length&&!error" class="muted">当前没有需要提醒的任务。可在设置 → 任务提醒中调整提醒时间。</p><p class="field-help">关闭或最小化后可从顶部铃铛恢复。今日忽略只影响提醒，不会完成任务。</p></section></FloatingWindow></Teleport></template>
<style scoped>.reminder-bell{position:relative}.reminder-count{position:absolute;right:-4px;top:-3px;background:#d65d70;color:white;font-size:10px;min-width:14px;height:14px;padding:0 3px;box-sizing:border-box;border-radius:8px}.reminder-content{padding:14px 20px;width:100%;box-sizing:border-box}.reminder-card{padding:14px 0;border-bottom:1px solid var(--line)}.reminder-card>strong{display:block;overflow-wrap:anywhere;font-size:.95rem}.reminder-actions{display:flex;flex-wrap:wrap;gap:9px;align-items:center;margin-top:11px}</style>
