import { api, desktop, type HolidayYear, type LunarYear } from './api'
// Successful and failed requests are shared for this session. Failures are retried
// on the next launch, or by an explicit calendar retry; disk caches live in Go.
const holidays = new Map<number, Promise<HolidayYear>>()
const lunar = new Map<number, Promise<LunarYear>>()
export async function calendarYear(year:number,retry=false) {
 if(retry) { holidays.delete(year);lunar.delete(year) }
 let h=holidays.get(year),l=lunar.get(year)
 if(!h){h=api.GetHolidayYear(year);holidays.set(year,h)}
 if(!l){l=api.GetLunarYear(year);lunar.set(year,l)}
 return Promise.allSettled([h,l] as const)
}
export function warmCalendar() {
 if(!desktop)return
 const year=new Date().getFullYear()
 // Never block startup or display background prefetch failures.
 void Promise.allSettled([calendarYear(year),calendarYear(year+1)])
}
