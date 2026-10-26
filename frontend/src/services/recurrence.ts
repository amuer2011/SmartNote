import type { Task, Recurrence } from './api'
const DATE_RE=/^\d{4}-\d{2}-\d{2}$/
function untilOf(r:Recurrence){const u=(r as Recurrence&{until?:string}).until;return typeof u==='string'&&DATE_RE.test(u)?u:''}
export function recurrenceLabel(r?:Recurrence) {if(!r)return '';const unit=({day:'天',week:'周',month:'月',year:'年'} as Record<string,string>)[r.unit]||r.unit;const base=`每${r.interval===1?'':r.interval}${unit}`,until=untilOf(r);return until?`${base}（截止 ${until}）`:`${base}（无截止日期）`}
export function occurrenceDate(r:Recurrence,index:number) {
 if(r.interval<1||!Number.isFinite(r.interval))return ''
 const a=new Date(r.anchor+'T12:00:00'),step=index*r.interval,day=r.day||a.getDate()
 if(r.unit==='day'||r.unit==='week')a.setDate(a.getDate()+step*(r.unit==='week'?7:1))
 else {const m=a.getMonth(),y=a.getFullYear();a.setDate(1);if(r.unit==='month')a.setMonth(m+step);else a.setFullYear(y+step);const last=new Date(a.getFullYear(),a.getMonth()+1,0).getDate();a.setDate(Math.min(day,last))}
 if(!Number.isFinite(a.getTime())||a.getFullYear()>9999)return ''
 const date=`${a.getFullYear()}-${String(a.getMonth()+1).padStart(2,'0')}-${String(a.getDate()).padStart(2,'0')}`
 const until=untilOf(r)
 return until&&date>until?'':date
}
export function taskOccurrences(task:Task,start:string,end:string):Task[] {
 if(!task.recurrence)return task.dueAt>=start&&task.dueAt<=end?[task]:[]
 const out:Task[]=[],done=new Set(task.completedDates||[])
 for(let i=0;i<200000;i++){const date=occurrenceDate(task.recurrence,i);if(!date||date>end)break;if(date>=start)out.push({...task,dueAt:date,done:done.has(date)})}
 return out
}
