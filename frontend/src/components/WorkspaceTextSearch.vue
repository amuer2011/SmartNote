<script setup lang="ts">
import {computed,ref,watch} from 'vue'
import {useWorkspace} from '../stores/workspace'
import {api,type Note} from '../services/api'
import {textMatches,applyMatches,type TextMatch} from '../services/text-search'
const props=defineProps<{query:string;regex:boolean}>()
const emit=defineEmits<{'update:query':[value:string];open:[note:Note,line:number]}>()
const store=useWorkspace(),caseSensitive=ref(false),replacement=ref(''),replaceOpen=ref(false),error=ref(''),status=ref(''),busy=ref(false)
type Change={note:Note;matches:TextMatch[];content:string}
const plan=ref<Change[]>([])
const found=computed(()=>{try{return {error:'',items:store.data.notes.map(note=>({note,matches:textMatches(note.content,props.query,props.regex,caseSensitive.value,replacement.value)})).filter(item=>item.matches.length)}}catch(e){return {error:'正则表达式无效：'+String(e),items:[]}}})
const count=computed(()=>found.value.items.reduce((n,item)=>n+item.matches.length,0))
watch([()=>props.query,()=>props.regex,caseSensitive,replacement,()=>store.workspace?.id,()=>store.data.notes.map(n=>n.id+':'+n.hash).join('|')],()=>{plan.value=[];status.value='';error.value=''})
function line(note:Note,match:TextMatch){return note.content.slice(0,match.start).split('\n').length-1}
function snippet(note:Note,match:TextMatch){return note.content.slice(Math.max(0,match.start-30),Math.min(note.content.length,match.end+65))}
async function preview(){error.value='';await store.save();if(store.dirty){error.value='请先处理当前笔记保存冲突';return};plan.value=found.value.items.map(item=>({...item,note:{...item.note},content:applyMatches(item.note.content,item.matches)})).filter(item=>item.content!==item.note.content);if(!plan.value.length)status.value='没有需要修改的内容'}
async function apply(changes:Change[]){
 if(busy.value)return
 busy.value=true;error.value='';let files=0,total=0,failure=''
 const workspaceID=store.workspace?.id
 try{
  await store.save();if(store.dirty)throw new Error('请先处理当前笔记保存冲突')
  for(const item of changes){
   if(store.workspace?.id!==workspaceID)throw new Error('工作区已切换，停止替换')
   const current=store.data.notes.find(n=>n.id===item.note.id)
   if(!current||current.hash!==item.note.hash)throw new Error('笔记已改变，请重新预览：'+item.note.path)
   const saved=await api.SaveNote({id:item.note.id,path:item.note.path,hash:item.note.hash,content:item.content})
   const index=store.data.notes.findIndex(n=>n.id===saved.id);if(index>=0)store.data.notes[index]=saved
   if(store.active?.id===saved.id){store.active=saved;store.content=saved.content}
   files++;total+=item.matches.length
  }
 }catch(e){failure=String(e)}finally{
  await store.refresh();plan.value=[];busy.value=false;error.value=failure;status.value=`已替换 ${files} 篇笔记中的 ${total} 处`+(failure?'；其余未修改，请检查错误后重新预览':'')
 }
}
defineExpose({openReplace:()=>{replaceOpen.value=true}})
function replaceOne(note:Note,match:TextMatch){void apply([{note:{...note},matches:[match],content:applyMatches(note.content,[match])}])}
</script>
<template><div class="workspace-text-search"><input :value="query" aria-label="全局查找内容" :placeholder="regex?'输入正则表达式…':'查找工作区 Markdown 原文…'" :disabled="busy" @input="emit('update:query',($event.target as HTMLInputElement).value)"/><div class="search-options"><label><input v-model="caseSensitive" type="checkbox" :disabled="busy"/>区分大小写</label><button class="text-button" :disabled="busy" @click="replaceOpen=!replaceOpen">{{replaceOpen?'收起替换':'展开替换'}}</button></div><div v-if="replaceOpen" class="replace-controls"><input v-model="replacement" aria-label="全局替换内容" placeholder="替换为…（留空则删除）" :disabled="busy"/><p class="muted small">替换当前工作区的 Markdown 原文。<template v-if="regex">支持 $1、$&lt;名称&gt; 捕获组。</template></p><button class="secondary-button" :disabled="busy||!count||!!found.error" @click="preview">预览全部替换</button></div><p v-if="found.error||error" class="bookmark-error" role="alert">{{found.error||error}}</p><p v-if="status" role="status">{{status}}</p><div v-if="plan.length" class="replacement-preview"><strong>将修改 {{plan.length}} 篇笔记，{{plan.reduce((n,item)=>n+item.matches.length,0)}} 处</strong><div v-for="item in plan" :key="item.note.id" class="replace-preview-item"><b>{{item.note.path}}</b><div v-for="(match,i) in item.matches.slice(0,5)" :key="i"><del>{{match.text||'（空位置）'}}</del> → <ins>{{match.replacement||'（删除）'}}</ins></div><small v-if="item.matches.length>5">另有 {{item.matches.length-5}} 处</small></div><button class="primary-button" :disabled="busy" @click="apply([...plan])">{{busy?'正在替换…':'应用以上替换'}}</button><button class="text-button" :disabled="busy" @click="plan=[]">取消</button></div><div class="results-heading">{{found.items.length}} 篇笔记 · {{count}} 处匹配</div><article v-for="item in found.items" :key="item.note.id" class="text-search-result"><strong>{{item.note.title}}</strong><small>{{item.note.path}} · {{item.matches.length}} 处</small><div v-for="(match,i) in item.matches.slice(0,100)" :key="i" class="text-match"><button class="match-open" :disabled="busy" @click="emit('open',item.note,line(item.note,match))"><span>第 {{line(item.note,match)+1}} 行</span><pre>{{snippet(item.note,match)}}</pre></button><button v-if="replaceOpen" class="text-button" :disabled="busy" @click="replaceOne(item.note,match)">替换此处</button></div><small v-if="item.matches.length>100">仅展示前 100 处；全部替换包含所有匹配。</small></article></div></template>
<style scoped>.workspace-text-search{margin-top:15px}.workspace-text-search>input,.replace-controls>input{box-sizing:border-box;width:100%;padding:10px 12px;background:var(--bg);color:var(--text);border:1px solid var(--line);border-radius:7px;font:inherit}.search-options{display:flex;align-items:center;justify-content:space-between;margin:9px 0;font-size:.8rem}.search-options label{display:flex;align-items:center;gap:5px}.replace-controls{margin-bottom:14px}.replacement-preview{border:1px solid var(--accent);border-radius:8px;padding:12px;margin:12px 0}.replace-preview-item{margin:10px 0;overflow-wrap:anywhere;font-size:.8rem}.replace-preview-item b{display:block}del{color:#bd3349}ins{color:#16865d}.text-search-result{border-bottom:1px solid var(--line);padding:14px 0}.text-search-result>small{display:block;color:var(--muted);overflow-wrap:anywhere;margin:5px 0}.text-match{padding:5px 0}.match-open{display:block;text-align:left;width:100%;background:transparent;border:0;color:var(--text);cursor:pointer;padding:5px}.match-open:hover{background:var(--hover)}.match-open span{font-size:.72rem;color:var(--muted)}pre{white-space:pre-wrap;overflow-wrap:anywhere;font:inherit;font-size:.82rem;margin:4px 0}</style>
