<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NSelect } from 'naive-ui'
import { FileText, Search } from 'lucide-vue-next'
import { api, type TaskArchiveRecord } from '../services/api'

type ArchiveRecord = TaskArchiveRecord

const props=defineProps<{workspaceId:string;revision:number}>()
const emit=defineEmits<{open:[noteId:string,line:number]}>()
const records=ref<ArchiveRecord[]>([]),loading=ref(false),error=ref(''),query=ref(''),year=ref(''),month=ref('')
let sequence=0,lastWorkspaceId=''

async function load(){
 const workspaceId=props.workspaceId
 if(!workspaceId){sequence++;records.value=[];loading.value=false;error.value='';return}
 const request=++sequence
 loading.value=true;error.value=''
 try{
  const value=await api.GetTaskArchive(workspaceId)
  if(request!==sequence)return
  records.value=Array.isArray(value)?value:[]
 }catch(e){
  if(request!==sequence)return
  records.value=[];error.value=String(e)
 }finally{if(request===sequence)loading.value=false}
}

watch([()=>props.workspaceId,()=>props.revision],()=>{
 if(props.workspaceId!==lastWorkspaceId){lastWorkspaceId=props.workspaceId;query.value='';year.value='';month.value='';records.value=[]}
 void load()
},{immediate:true})

function monthKey(value:string){
 const match=/^(\d{4})[-/.](\d{1,2})/.exec(String(value||'').trim())
 return match?`${match[1]}-${match[2].padStart(2,'0')}`:''
}
function monthLabel(key:string){return key==='unknown'?'完成时间未知':`${key.slice(0,4)} 年 ${key.slice(5,7)} 月`}
function formatTime(value:string){
 const text=String(value||'').replace('T',' ')
 return text?text.slice(0,16):'—'
}
function openSource(record:ArchiveRecord){emit('open',record.noteId,record.line)}

const monthKeys=computed(()=>[...new Set(records.value.map(record=>monthKey(record.completedAt)).filter(Boolean))].sort().reverse())
const yearOptions=computed(()=>[{label:'全部年份',value:''},...[...new Set(monthKeys.value.map(key=>key.slice(0,4)))].map(value=>({label:`${value} 年`,value}))])
const monthOptions=computed(()=>{
 const pool=year.value?monthKeys.value.filter(key=>key.startsWith(year.value)):monthKeys.value
 const months=[...new Set(pool.map(key=>key.slice(5,7)))].sort()
 return [{label:year.value?'该年全部月份':'全部月份',value:''},...months.map(value=>({label:`${value} 月`,value}))]
})
watch(year,()=>{if(month.value&&!monthOptions.value.some(option=>option.value===month.value))month.value=''})

const filtered=computed(()=>records.value.filter(record=>{
 const key=monthKey(record.completedAt)
 if(year.value&&!key.startsWith(year.value))return false
 if(month.value&&key.slice(5,7)!==month.value)return false
 const keyword=query.value.trim().toLowerCase()
 if(!keyword)return true
 return (record.title||'').toLowerCase().includes(keyword)||(record.noteTitle||'').toLowerCase().includes(keyword)
}))
const groups=computed(()=>{
 const buckets=new Map<string,ArchiveRecord[]>()
 for(const record of filtered.value){const key=monthKey(record.completedAt)||'unknown';const list=buckets.get(key);if(list)list.push(record);else buckets.set(key,[record])}
 return [...buckets.entries()].map(([key,items])=>({key,label:monthLabel(key),items:[...items].sort((a,b)=>a.completedAt<b.completedAt?1:a.completedAt>b.completedAt?-1:0)})).sort((a,b)=>{
  if(a.key==='unknown')return 1
  if(b.key==='unknown')return -1
  return a.key<b.key?1:-1
 })
})
</script>

<template>
 <div class="task-archive">
  <p class="arc-count">普通任务完成后将原笔记移动到「已完成待办/年/月」目录。归档笔记可编辑，不再生成待办；循环笔记保留以继续提醒。</p>
  <div class="arc-toolbar">
   <label class="arc-search"><Search :size="15" aria-hidden="true"/><input v-model="query" type="search" placeholder="搜索已完成任务标题" aria-label="搜索已完成任务标题"/></label>
   <div class="arc-filters">
    <NSelect v-model:value="year" class="arc-select" size="small" :options="yearOptions" aria-label="按年份筛选" :consistent-menu-width="false"/>
    <NSelect v-model:value="month" class="arc-select" size="small" :options="monthOptions" aria-label="按月份筛选" :consistent-menu-width="false"/>
    <span class="arc-count">共 {{ filtered.length }} 条</span>
   </div>
  </div>
  <p v-if="!props.workspaceId" class="arc-state">请先打开工作区。</p>
  <p v-else-if="loading" class="arc-state" role="status">正在加载已完成任务…</p>
  <div v-else-if="error" class="arc-error" role="alert"><span>{{error}}</span><NButton size="small" @click="load">重试</NButton></div>
  <p v-else-if="!records.length" class="arc-state">暂无已完成任务。任务完成后会按完成时间归档到这里。</p>
  <p v-else-if="!filtered.length" class="arc-state">没有匹配的已完成任务，试试调整筛选或搜索关键词。</p>
  <template v-else>
   <section v-for="group in groups" :key="group.key" class="arc-group">
    <header class="arc-group-head"><span class="arc-group-title">{{ group.label }}</span><span class="arc-group-count">{{ group.items.length }}</span></header>
    <ul class="arc-list">
     <li v-for="record in group.items" :key="record.id" class="arc-item">
      <div class="arc-main">
       <span class="arc-title">{{ record.title || '(未命名任务)' }}</span>
       <span v-if="record.reopenedAt" class="arc-badge arc-badge-warn" :title="'撤销时间：'+formatTime(record.reopenedAt||'')">已撤销完成</span>
       <span v-if="record.imported" class="arc-badge" title="由历史任务记录补录">历史补录</span>
       <span v-if="record.priority==='high'" class="arc-badge arc-badge-high">高优先级</span>
      </div>
      <div class="arc-meta">
       <span>完成时间 {{ formatTime(record.completedAt) }}</span>
       <span>原截止日期 {{ record.dueAt ? formatTime(record.dueAt) : '未设置' }}</span>
      </div>
      <div class="arc-path">
       <FileText :size="14" aria-hidden="true"/>
       <button type="button" class="arc-open" :title="'在源文件中定位：'+(record.notePath||record.noteTitle)" @click="openSource(record)">{{ record.notePath || record.noteTitle || '查看源文件' }}</button>
      </div>
     </li>
    </ul>
   </section>
  </template>
 </div>
</template>

<style scoped>
.task-archive{display:flex;flex-direction:column;gap:14px;color:var(--text);min-width:0}
.arc-toolbar{display:flex;gap:10px;flex-wrap:wrap;align-items:center;justify-content:space-between}
.arc-search{display:flex;align-items:center;gap:7px;flex:1 1 220px;min-width:200px;padding:7px 10px;background:var(--soft);border:1px solid var(--line);border-radius:8px;color:var(--muted)}
.arc-search input{flex:1;min-width:0;background:none;border:0;outline:none;color:var(--text);font:inherit}
.arc-search input::placeholder{color:var(--muted)}
.arc-search:focus-within{border-color:var(--accent)}
.arc-filters{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.arc-select{width:132px}
.arc-count{font-size:.857rem;color:var(--muted)}
.arc-state{margin:0;padding:20px;text-align:center;color:var(--muted);background:var(--soft);border:1px dashed var(--line);border-radius:8px}
.arc-error{display:flex;align-items:center;justify-content:space-between;gap:10px;padding:12px 14px;background:var(--soft);border:1px solid var(--line);border-left:3px solid var(--accent);border-radius:8px;overflow-wrap:anywhere}
.arc-group{display:flex;flex-direction:column;gap:8px}
.arc-group-head{display:flex;align-items:center;gap:8px;padding-bottom:6px;border-bottom:1px solid var(--line)}
.arc-group-title{font-weight:600;color:var(--accent)}
.arc-group-count{font-size:.8rem;color:var(--accent);background:var(--accent-bg);border-radius:999px;padding:1px 8px}
.arc-list{list-style:none;margin:0;padding:0;display:flex;flex-direction:column;gap:8px}
.arc-item{display:flex;flex-direction:column;gap:6px;padding:11px 13px;background:var(--bg);border:1px solid var(--line);border-left:3px solid var(--accent);border-radius:8px;min-width:0}
.arc-item:hover{background:var(--soft)}
.arc-main{display:flex;align-items:center;gap:8px;flex-wrap:wrap}
.arc-title{font-weight:600;overflow-wrap:anywhere}
.arc-badge{font-size:.75rem;line-height:1.6;padding:1px 7px;border-radius:999px;white-space:nowrap;color:var(--accent);background:var(--accent-bg)}
.arc-badge-warn{color:var(--muted);background:var(--soft);border:1px solid var(--line)}
.arc-badge-high{color:var(--bg);background:var(--accent)}
.arc-meta{display:flex;gap:14px;flex-wrap:wrap;font-size:.857rem;color:var(--muted)}
.arc-path{display:flex;align-items:center;gap:6px;font-size:.857rem;color:var(--muted);min-width:0}
.arc-open{background:none;border:0;padding:0;font:inherit;color:var(--accent);cursor:pointer;text-align:left;overflow-wrap:anywhere}
.arc-open:hover{text-decoration:underline}
</style>
