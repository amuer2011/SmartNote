import { api, type SemanticConfig } from './api'

const pending = new Map<string, Promise<SemanticConfig>>()
export function saveSemanticThresholds(id:string, threshold:number):Promise<SemanticConfig> {
 const previous=pending.get(id)
 const task=(async()=>{
  if(previous) await previous.catch(()=>undefined)
  const current=await api.GetSemanticConfig(id)
  return api.SaveSemanticConfig(id,{...current,threshold})
 })()
 pending.set(id,task)
 return task
}
export async function waitForSemanticSettings(id:string){
 let task=pending.get(id)
 while(task){
  await task
  const next=pending.get(id)
  if(next===task)return
  task=next
 }
}
