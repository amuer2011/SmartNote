<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton, NSelect } from 'naive-ui'
import { ListTodo, FileText, ArrowUpRight, Check, Sparkles, CalendarDays, SlidersHorizontal, ChevronLeft, ChevronRight, Sun, Search } from 'lucide-vue-next'
import { api, type Task, type Note, type HolidayYear, type LunarYear } from '../services/api'
import { calendarYear } from '../services/calendar-cache'
import CompletedTaskArchive from './CompletedTaskArchive.vue'
import StyledDatePicker from './StyledDatePicker.vue'
import { recurrenceLabel, taskOccurrences } from '../services/recurrence'
import ResizableModal from './ResizableModal.vue'
import { useWorkspace } from '../stores/workspace'
const archiveRevision=ref(0)
const props=defineProps<{view:string}>()
const emit = defineEmits<{ open: [note: Note, line: number]; minimizeTask:[value:{taskId:string;dueAt:string;priority:string}] }>()
const store = useWorkspace(), project = ref(''), priority = ref(''), tag = ref(''), busy = ref(false)
const manage=ref(false), selectedSeries=ref<string[]>([]), deleteNotesConfirm=ref(false), lastRemoved=ref<{workspaceID:string;ids:string[]}|null>(null)
const recurringTasks=computed(()=>filtered.value.filter(t=>!!t.recurrence&&!t.candidate))
const selectedRecurring=computed(()=>recurringTasks.value.filter(t=>selectedSeries.value.includes(t.id)))
function openManager(){selectedSeries.value=[];deleteNotesConfirm.value=false;manage.value=true}
async function removeSeries(){const id=store.workspace?.id;if(!id)return;const ids=[...selectedSeries.value];await perform(async()=>{await api.SetRecurringEnabled(id,ids,false);lastRemoved.value={workspaceID:id,ids};manage.value=false})}
async function restoreSeries(){const last=lastRemoved.value;if(!last||store.workspace?.id!==last.workspaceID)return;await perform(async()=>{await api.SetRecurringEnabled(last.workspaceID,last.ids,true);lastRemoved.value=null})}
async function deleteSeriesNotes(){const workspaceID=store.workspace?.id;const notes=[...new Set(selectedRecurring.value.map(t=>t.noteId))].map(id=>store.data.notes.find(n=>n.id===id)).filter((n):n is Note=>!!n);let count=0;await perform(async()=>{try{for(const n of notes){if(store.workspace?.id!==workspaceID)throw Error('工作区已切换');await api.DeleteNote(n.id,n.hash);count++}manage.value=false;deleteNotesConfirm.value=false}catch(e){await store.refresh();throw Error(`已删除 ${count} 篇，剩余未删除：${String(e)}`)}})}
const editing = ref<Task | null>(null), dueAt = ref(''), editPriority = ref('normal')
const today = () => { const now = new Date(); return `${now.getFullYear()}-${String(now.getMonth()+1).padStart(2,'0')}-${String(now.getDate()).padStart(2,'0')}` }
const pending = computed(() => store.data.tasks.filter(t => !t.done && !t.candidate).length)
const candidates = computed(() => store.data.tasks.filter(t => t.candidate).length)
const options = (values: string[]) => [{label:'全部',value:''}, ...[...new Set(values.filter(Boolean))].sort().map(value => ({label:value,value}))]
const projects = computed(() => options(store.data.tasks.map(t => t.project)))
const tags = computed(() => options(store.data.tasks.flatMap(t => t.tags || [])))
const query=ref(''), selected=ref(today()), month=ref(today().slice(0,7))
const holidays=ref<Record<number,HolidayYear>>({}), lunar=ref<Record<number,LunarYear>>({}), calendarError=ref(''), calendarLoading=ref(false)
const requests=new Map<number,Promise<void>>()
let calendarSequence=0
async function loadYear(year:number,retry=false) {
 if(retry)requests.delete(year)
 if(requests.has(year))return requests.get(year)!
 const job=(async()=>{
  const results=await calendarYear(year,retry)
  const h=results[0],l=results[1]
  if(h.status==='fulfilled')holidays.value[year]=h.value
  if(l.status==='fulfilled')lunar.value[year]=l.value
  const errors=results.filter((r):r is PromiseRejectedResult=>r.status==='rejected').map(r=>String(r.reason))
  if(errors.length)throw Error(errors.join('；'))
 })()
 requests.set(year,job)
 try {await job} catch(e){requests.delete(year);throw e}
}
function dateValue(value:string) {return new Date(value+'T12:00:00')}
function dateKey(date:Date) {return `${date.getFullYear()}-${String(date.getMonth()+1).padStart(2,'0')}-${String(date.getDate()).padStart(2,'0')}`}
function offset(value:string,days:number) {const d=dateValue(value);d.setDate(d.getDate()+days);return dateKey(d)}
const week=computed(()=>{const d=dateValue(selected.value),monday=offset(selected.value,1-(d.getDay()||7));return Array.from({length:7},(_,i)=>offset(monday,i))})
const cells=computed(()=>{const first=(/^\d{4}-(0[1-9]|1[0-2])$/.test(month.value)?month.value:today().slice(0,7))+'-01';const start=offset(first,1-(dateValue(first).getDay()||7));return Array.from({length:42},(_,i)=>offset(start,i))})
watch([()=>props.view,month],async()=>{
 if(props.view!=='calendar'||!/^\d{4}-(0[1-9]|1[0-2])$/.test(month.value))return
 const seq=++calendarSequence;calendarLoading.value=true;calendarError.value=''
 try{await Promise.all([...new Set(cells.value.map(key=>Number(key.slice(0,4))))].filter(y=>y>=1900&&y<=2100).map(y=>loadYear(y)))}catch(e){if(seq===calendarSequence)calendarError.value='节假日或农历加载失败：'+String(e)+'。可重试，已有缓存仍可使用。'}finally{if(seq===calendarSequence)calendarLoading.value=false}
},{immediate:true})
function normalizeDates() {if(!month.value)month.value=today().slice(0,7);if(!selected.value)selected.value=today()}
function changeMonth(delta:number) {const d=dateValue(month.value+'-01');d.setMonth(d.getMonth()+delta);const key=dateKey(d).slice(0,7);if(Number(key.slice(0,4))>=1900&&Number(key.slice(0,4))<=2100)month.value=key}
async function retryCalendar() {calendarError.value='';calendarLoading.value=true;try{await Promise.all([...new Set(cells.value.map(key=>Number(key.slice(0,4))))].filter(y=>y>=1900&&y<=2100).map(y=>loadYear(y,true)))}catch(e){calendarError.value=String(e)}finally{calendarLoading.value=false}}
function dayInfo(key:string) {const y=Number(key.slice(0,4));const data=holidays.value[y];const day=data?.holiday[key.slice(5)];return data?.type[key] || (day?{type:day.holiday?2:3,name:day.name,week:dateValue(key).getDay()||7}:undefined)}
function lunarLabel(key:string) {const d=lunar.value[Number(key.slice(0,4))]?.days[key];return d ? d.term || (d.day==='初一'?d.month:d.day) : ''}
const filtered=computed(()=>store.data.tasks.filter(t=>{
 if(project.value&&t.project!==project.value || priority.value&&(t.priority||'normal')!==priority.value || tag.value&&!t.tags?.includes(tag.value))return false
 return !query.value || (t.title+' '+t.noteTitle).toLowerCase().includes(query.value.toLowerCase())
}).sort((a,b)=>(a.dueAt||'9999').localeCompare(b.dueAt||'9999') || ({high:0,normal:1,low:2}[a.priority as 'high'|'normal'|'low']??1)-({high:0,normal:1,low:2}[b.priority as 'high'|'normal'|'low']??1)))
const groups=computed(()=>{
 const list=filtered.value,now=today()
 if(props.view==='day')return [{name: selected.value===now?'今日待办':selected.value+' 的待办',tasks:list.filter(t=>!t.candidate).flatMap(t=>taskOccurrences(t,selected.value,selected.value))}]
 if(props.view==='recurring')return [{name:'循环计划',tasks:list.filter(t=>!!t.recurrence&&!t.candidate)}]
 if(props.view==='inbox')return [{name:'未安排日期',tasks:list.filter(t=>!t.candidate&&!t.done&&!t.dueAt)}]
 if(props.view==='done')return [{name:'已完成',tasks:list.flatMap(t=>t.recurrence?(t.completedDates||[]).map(date=>({...t,dueAt:date,done:true})):t.done?[t]:[])}]
 if(props.view==='candidate')return [{name:'候选任务',tasks:list.filter(t=>t.candidate)}]
 return [
  {name:'逾期未完成',tasks:list.filter(t=>!t.candidate&&!t.done&&!!t.dueAt&&t.dueAt<now)},
  {name:'今天',tasks:list.filter(t=>!t.candidate&&!t.done&&t.dueAt===now)},
  {name:'未来 7 天',tasks:list.filter(t=>!t.candidate&&!t.done&&t.dueAt>now&&t.dueAt<=offset(now,7))},
  {name:'后续日程',tasks:list.filter(t=>!t.candidate&&!t.done&&t.dueAt>offset(now,7))},
  {name:'未安排日期',tasks:list.filter(t=>!t.candidate&&!t.done&&!t.dueAt)},
  {name:'已完成',tasks:list.filter(t=>!t.candidate&&t.done)}
 ].filter(g=>g.tasks.length)
})
const calendarTasks=computed(()=>{const map:Record<string,Task[]>={};for(const t of filtered.value)if(!t.candidate)for(const item of taskOccurrences(t,cells.value[0],cells.value[41]))(map[item.dueAt]??=[]).push(item);return map})
const viewTitle=computed(()=>({recent:'最近待办',day:'当日待办',calendar:'日程概览',inbox:'待办箱',done:'完成归档',candidate:'候选任务',recurring:'循环计划'}[props.view]||'待办事项'))
async function perform(fn: () => Promise<void>) { if(busy.value||store.switching)return;const workspaceID=store.workspace?.id;busy.value=true; try { await store.save();if(store.switching||store.workspace?.id!==workspaceID)return; if(store.dirty) { store.error='请先处理未保存的笔记修改。';return }; await fn();if(store.workspace?.id!==workspaceID)return; await store.refresh();archiveRevision.value++ } catch(e) {if(store.workspace?.id===workspaceID)store.report(e)} finally {busy.value=false} }
function toggleTask(task:Task) {return perform(()=>task.recurrence?api.SetTaskOccurrence(store.workspace!.id,task.id,task.dueAt,!task.done):api.SetTask(task.id,!task.done,false))}
function confirmTask(task:Task) {return perform(()=>task.needsConfirmation?api.ResolveTaskIntent({workspaceId:store.workspace!.id,taskId:task.id,sourceRevision:task.sourceRevision||'',accept:true}):api.SetTask(task.id,false,true))}
function chooseDay(key:string) {selected.value=key}
function edit(task: Task) { if(task.recurrence)task=store.data.tasks.find(t=>t.id===task.id)||task;editing.value=task;dueAt.value=task.dueAt || '';editPriority.value=task.priority || 'normal' }
async function saveDetails() { if (!editing.value) return; await perform(async () => { await api.UpdateTaskDetails({id:editing.value!.id,dueAt:dueAt.value,priority:editPriority.value});editing.value=null }) }
function openArchived(noteId:string,line:number) {const n=store.data.notes.find(n=>n.id===noteId);if(n)emit('open',n,line);else store.error='原笔记已删除，完成记录仍保存在归档中。'}
function open(task: Task) { const n=store.data.notes.find(n => n.id===task.noteId);if(n)emit('open',n,task.line) }
function minimize() {if(editing.value) {emit('minimizeTask',{taskId:editing.value.id,dueAt:dueAt.value,priority:editPriority.value});editing.value=null}}
function restoreDetails(value:{taskId:string;dueAt:string;priority:string}) {const task=store.data.tasks.find(task=>task.id===value.taskId);if(task) {editing.value=task;dueAt.value=value.dueAt;editPriority.value=value.priority}}
defineExpose({restoreDetails})
</script>
<template>
 <section class="schedule-view" :aria-label="viewTitle">
  <header class="schedule-toolbar"><div><span class="eyebrow">NOTES INTO ACTION</span><h1>{{viewTitle}}</h1></div><span v-if="view!=='done'" class="schedule-count">{{pending}} 项待完成</span><button v-if="view!=='done'" class="secondary-button" :disabled="!recurringTasks.length" @click="openManager">批量管理循环</button><button v-if="view!=='done' && lastRemoved?.workspaceID===store.workspace?.id" class="text-button" @click="restoreSeries">恢复上次移除</button>
   <div v-if="view==='calendar'" class="month-switch"><button class="icon-button" aria-label="上个月" @click="changeMonth(-1)"><ChevronLeft :size="17"/></button><StyledDatePicker v-model="month" type="month" label="日历月份" :clearable="false"/><button class="icon-button" aria-label="下个月" @click="changeMonth(1)"><ChevronRight :size="17"/></button><button class="text-button" @click="month=today().slice(0,7);selected=today()">今天</button></div>
  </header>
  <div v-if="view==='day'" class="day-strip"><button class="icon-button" aria-label="上一周" @click="selected=offset(selected,-7)"><ChevronLeft :size="17"/></button><button v-for="(key,i) in week" :key="key" :class="{chosen:key===selected,today:key===today()}" @click="selected=key"><small>{{['一','二','三','四','五','六','日'][i]}}</small><strong>{{key===today()?'今':Number(key.slice(-2))}}</strong></button><button class="icon-button" aria-label="下一周" @click="selected=offset(selected,7)"><ChevronRight :size="17"/></button><StyledDatePicker v-model="selected" label="待办日期" :clearable="false"/><button class="text-button" @click="selected=today()">今天</button><Sun :size="18"/></div>
  <div v-if="view!=='done'" class="schedule-search"><Search :size="17"/><input v-model="query" placeholder="搜索待办或关联笔记…" aria-label="搜索待办"/><span>自动关联原笔记</span></div>
  <div v-if="view!=='done'" class="schedule-filters"><select v-model="project" aria-label="项目筛选"><option v-for="o in projects" :key="o.value" :value="o.value">{{o.value?o.label:'全部项目'}}</option></select><select v-model="priority" aria-label="优先级筛选"><option value="">所有优先级</option><option value="high">高优先级</option><option value="normal">普通优先级</option><option value="low">低优先级</option></select><select v-model="tag" aria-label="标签筛选"><option v-for="o in tags" :key="o.value" :value="o.value">{{o.value?'# '+o.label:'全部标签'}}</option></select><span v-if="view==='calendar'">{{calendarLoading?'正在加载年度日历…':'农历 · 节气 · 节假日 · 调休'}}</span></div>
  <template v-if="view==='calendar'">
   <div v-if="calendarError" class="calendar-warning" role="status">{{calendarError}}<button class="text-button" :disabled="calendarLoading" @click="retryCalendar">重试</button></div>
   <div class="calendar-week"><span v-for="day in ['星期一','星期二','星期三','星期四','星期五','星期六','星期日']" :key="day">{{day}}</span></div>
   <div class="schedule-calendar" role="grid" aria-label="日程月历"><div v-for="key in cells" :key="key" role="gridcell" :aria-label="key" :aria-selected="selected===key" class="calendar-cell" :class="{outside:key.slice(0,7)!==month,today:key===today(),selected:key===selected}" @click="chooseDay(key)" @dblclick="selected=key"><button class="calendar-date" :aria-label="'选择 '+key" @click.stop="chooseDay(key)"><strong>{{Number(key.slice(-2))}}</strong><span>{{lunarLabel(key)}}</span><b v-if="dayInfo(key)?.type===2" class="rest" :title="dayInfo(key)?.name">休</b><b v-else-if="dayInfo(key)?.type===3" class="work" :title="dayInfo(key)?.name">班</b><b v-else-if="dayInfo(key)?.type===1" class="weekend" title="周末">休</b></button><small v-if="dayInfo(key)?.type===2 || dayInfo(key)?.type===3" class="holiday-name">{{dayInfo(key)?.name}}</small><button v-for="task in calendarTasks[key]||[]" :key="task.id" class="calendar-task" :class="{done:task.done,high:task.priority==='high'}" :title="task.title+' · 点击打开原笔记'" @click.stop="open(task)">{{task.done?'✓ ':task.recurrence?'↻ ':''}}{{task.title}}</button></div></div>
   <div class="calendar-legend"><span><i class="rest"/>法定休假</span><span><i class="work"/>调休补班</span><span>{{selected}} · {{(calendarTasks[selected]||[]).length}} 项日程</span></div>
   <div v-if="calendarTasks[selected]?.length" class="selected-agenda"><div v-for="task in calendarTasks[selected]" :key="task.id"><button class="task-checkbox" :class="{checked:task.done}" :disabled="busy" :aria-label="task.done?'标记未完成':'完成任务'" @click="toggleTask(task)"><Check v-if="task.done" :size="12"/></button><button @click="open(task)">{{task.title}}</button><button class="icon-button" title="设置日期和优先级" @click="edit(task)"><SlidersHorizontal :size="15"/></button></div></div>
  </template>
  <CompletedTaskArchive v-else-if="view==='done'" :workspace-id="store.workspace?.id||''" :revision="archiveRevision+store.data.tasks.length" @open="openArchived"/>
  <div v-else class="schedule-list"><section v-for="group in groups" :key="group.name" class="schedule-group"><h2>{{group.name}}<span>{{group.tasks.length}}</span></h2><div v-for="task in group.tasks" :key="task.id+task.dueAt" class="task-row"><button v-if="!task.candidate" class="task-checkbox" :class="{checked:task.done}" :disabled="busy" :aria-label="task.done?'标记未完成':'完成任务'" @click="toggleTask(task)"><Check v-if="task.done" :size="12"/></button><Sparkles v-else :size="17" class="candidate-dot"/><div class="task-info"><button class="task-title" :class="{completed:task.done}" @click="open(task)">{{task.title}}</button><button @click="open(task)"><FileText :size="12"/>{{task.noteTitle}}<span v-if="task.inferred" class="intent-label">笔记待办</span><ArrowUpRight :size="11"/></button><div class="task-badges"><span v-if="task.recurrence" class="intent-label">↻ {{recurrenceLabel(task.recurrence)}}</span><span v-if="task.project">{{task.project}}</span><span v-for="t in task.tags||[]" :key="t">#{{t}}</span></div></div><span class="schedule-due" :class="{overdue:!task.done&&task.dueAt&&task.dueAt<today()}">{{task.dueAt||'未安排日期'}}</span><span v-if="task.priority==='high'" class="priority-badge high">高</span><button v-if="task.candidate" class="secondary-button" :disabled="busy" @click="confirmTask(task)">确认为任务</button><button v-else class="icon-button" title="设置日期和优先级" @click="edit(task)"><SlidersHorizontal :size="16"/></button></div></section><div v-if="!groups.some(g=>g.tasks.length)" class="empty-state schedule-empty"><CalendarDays :size="48" :stroke-width="1"/><h3>暂无{{view==='day'?'当日':'匹配'}}日程</h3><p>在笔记标题或正文写下“下周五提醒我 VPN 延期”，<br/>即可自动关联到待办列表，原笔记保持不变。</p></div></div>
 <ResizableModal @minimize="minimize" :show="!!editing" preset="card" title="任务详情" style="width:460px" @update:show="editing=null"><p class="muted small">{{editing?.title}}</p><label class="field-label">截止日期</label><StyledDatePicker v-model="dueAt" class="task-detail-date" label="截止日期" :clearable="!editing?.recurrence" /><button v-if="!editing?.recurrence" class="text-button" @click="dueAt=''">清除截止日期</button><p v-if="editing?.recurrence" class="muted small">↻ {{recurrenceLabel(editing.recurrence)}} · 完成后自动进入下一次。修改日期会重新开始此循环，并重置当前循环进度，历史完成归档仍保留。</p><label class="field-label">优先级</label><NSelect v-model:value="editPriority" :options="[{label:'高',value:'high'},{label:'普通',value:'normal'},{label:'低',value:'low'}]" /><template #footer><div class="modal-actions"><NButton @click="editing=null">取消</NButton><NButton type="primary" :loading="busy" @click="saveDetails">保存任务设置</NButton></div></template></ResizableModal>
 <ResizableModal :show="manage" title="批量管理循环笔记" preset="card" style="width:620px" @update:show="manage=false;deleteNotesConfirm=false">
 <template v-if="!deleteNotesConfirm"><p class="muted small">选择循环计划。移除提醒保留原笔记；删除原笔记会将对应文件移入本地回收目录。</p><label class="series-select-all"><input type="checkbox" :checked="!!recurringTasks.length && selectedSeries.length===recurringTasks.length" @change="selectedSeries=selectedSeries.length===recurringTasks.length?[]:recurringTasks.map(t=>t.id)"/>全选当前筛选结果 · {{recurringTasks.length}} 项</label><div class="series-manager"><label v-for="task in recurringTasks" :key="task.id"><input v-model="selectedSeries" type="checkbox" :value="task.id"/><span><strong>{{task.title}}</strong><small>{{recurrenceLabel(task.recurrence)}} · 下次 {{task.dueAt}}<br/>{{store.data.notes.find(n=>n.id===task.noteId)?.path}}</small></span></label></div></template>
 <template v-else><p>确认删除以下 {{selectedRecurring.length}} 个循环计划对应的原笔记？文件将移入本地回收目录，循环提醒随之消失。</p><ul><li v-for="task in selectedRecurring" :key="task.id">{{store.data.notes.find(n=>n.id===task.noteId)?.path}}</li></ul></template>
 <template #footer><div class="modal-actions"><NButton :disabled="busy" @click="deleteNotesConfirm?deleteNotesConfirm=false:manage=false">{{deleteNotesConfirm?'返回':'取消'}}</NButton><NButton v-if="!deleteNotesConfirm" :disabled="!selectedSeries.length||busy" @click="deleteNotesConfirm=true">删除原笔记…</NButton><NButton v-if="!deleteNotesConfirm" type="primary" :disabled="!selectedSeries.length" :loading="busy" @click="removeSeries">移除所选循环提醒</NButton><NButton v-else type="error" :loading="busy" @click="deleteSeriesNotes">确认删除原笔记</NButton></div></template>
 </ResizableModal>
 </section>
</template>
