import type {AssistantAttachment} from './api'
export const longTextThreshold=2000
export const attachmentSize=(a:AssistantAttachment)=>Math.floor(a.data.length*3/4)-(a.data.endsWith('==')?2:a.data.endsWith('=')?1:0)
export const attachmentImage=(a:AssistantAttachment)=>/^image\/(png|jpeg|gif|webp)$/.test(a.mime)?`data:${a.mime};base64,${a.data}`:undefined
export async function readChatAttachment(file:File):Promise<AssistantAttachment>{
 if(!file.size)throw Error('不能添加空文件：'+file.name)
 if(file.size>8*1024*1024)throw Error('单个附件不能超过 8MB：'+file.name)
 const data=await new Promise<string>((resolve,reject)=>{const reader=new FileReader();reader.onload=()=>resolve(String(reader.result).split(',')[1]!);reader.onerror=()=>reject(Error('无法读取文件：'+file.name));reader.readAsDataURL(file)})
 return {id:crypto.randomUUID().replaceAll('-',''),name:file.name,mime:file.type||'application/octet-stream',data}
}
