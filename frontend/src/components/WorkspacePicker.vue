<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,ref,watch} from 'vue'
import {Check,ChevronDown,ChevronsUpDown,Pencil,Plus,Unplug} from 'lucide-vue-next'
import type {Workspace} from '../services/api'

const props=defineProps<{workspaces:Workspace[];currentId?:string;disabled?:boolean}>()
const emit=defineEmits<{select:[id:string];create:[];rename:[];remove:[]}>()
const trigger=ref<HTMLButtonElement>(),menu=ref<HTMLElement>(),opened=ref(false),expanded=ref(false)
const position=ref({left:'0px',top:'0px',width:'300px',maxHeight:'400px'})
const current=computed(()=>props.workspaces.find(w=>w.id===props.currentId))
const visible=computed(()=>expanded.value?props.workspaces:props.workspaces.slice(0,5))
function fit(){
 if(!trigger.value)return
 const rect=trigger.value.getBoundingClientRect(),gap=8
 const width=Math.min(Math.max(rect.width,300),window.innerWidth-gap*2)
 const top=Math.min(rect.bottom+6,window.innerHeight-120)
 position.value={left:Math.max(gap,Math.min(rect.left,window.innerWidth-width-gap))+'px',top:Math.max(gap,top)+'px',width:width+'px',maxHeight:Math.max(100,window.innerHeight-top-gap)+'px'}
}
function close(restore=false){opened.value=false;if(restore)trigger.value?.focus()}
async function open(){
 if(props.disabled)return
 expanded.value=false;fit();opened.value=true
 await nextTick();menu.value?.querySelector<HTMLButtonElement>('[aria-checked="true"], [role="menuitemradio"], .workspace-create')?.focus()
}
function select(id:string){close(true);emit('select',id)}
function action(type:'create'|'rename'|'remove'){close(true);if(type==='create')emit('create');else if(type==='rename')emit('rename');else emit('remove')}
async function expand(){
 expanded.value=true;await nextTick()
 menu.value?.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]')[5]?.focus()
}
function keys(event:KeyboardEvent){
 if(event.key==='Escape'){event.preventDefault();event.stopPropagation();close(true);return}
 if(event.key==='Tab'){close();return}
 if(!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return
 event.preventDefault();event.stopPropagation()
 const items=Array.from(menu.value?.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')||[])
 const index=items.indexOf(document.activeElement as HTMLButtonElement)
 const next=event.key==='Home'?0:event.key==='End'?items.length-1:(index+(event.key==='ArrowDown'?1:-1)+items.length)%items.length
 items[next]?.focus()
}
function outside(event:PointerEvent){if(!menu.value?.contains(event.target as Node)&&!trigger.value?.contains(event.target as Node))close()}
watch(opened,value=>{
 if(value){document.addEventListener('pointerdown',outside);window.addEventListener('resize',fit);window.addEventListener('blur',blur)}
 else removeListeners()
})
function blur(){close()}
function removeListeners(){document.removeEventListener('pointerdown',outside);window.removeEventListener('resize',fit);window.removeEventListener('blur',blur)}
watch(()=>props.disabled,value=>{if(value)close()})
watch(()=>props.currentId,()=>close())
onBeforeUnmount(removeListeners)
</script>

<template>
 <button ref="trigger" class="workspace-picker" aria-label="切换工作区" aria-haspopup="menu" aria-controls="workspace-menu" :aria-expanded="opened" :disabled="disabled" @click="opened?close():open()" @keydown.down.prevent="open()">
  <span class="workspace-avatar">{{current?.name.slice(0,1)||'S'}}</span><span><strong>{{current?.name||'我的工作空间'}}</strong><small>{{current?'本地工作区':'选择一个目录，开始记录'}}</small></span><ChevronsUpDown :size="15"/>
 </button>
 <Teleport to="body">
  <div v-if="opened" id="workspace-menu" ref="menu" class="workspace-menu" role="menu" aria-label="已打开的工作区" :style="position" @keydown="keys">
   <header><span>工作区 <small>{{workspaces.length}}</small></span><div><button v-if="current" role="menuitem" class="icon-button" aria-label="重命名当前工作区" title="重命名当前工作区" @click="action('rename')"><Pencil :size="14"/></button><button v-if="current" role="menuitem" class="icon-button" aria-label="移除当前工作区注册" title="移除注册，保留本地文件" @click="action('remove')"><Unplug :size="14"/></button></div></header>
   <div class="workspace-menu-list">
    <button v-for="workspace in visible" :key="workspace.id" role="menuitemradio" :aria-checked="workspace.id===currentId" :data-workspace-id="workspace.id" class="workspace-option" :title="workspace.path" @click="select(workspace.id)">
     <span class="workspace-avatar">{{workspace.name.slice(0,1)}}</span><span class="workspace-option-label"><strong>{{workspace.name}}</strong><small>{{workspace.path}}</small></span><Check v-if="workspace.id===currentId" :size="16"/>
    </button>
    <p v-if="!workspaces.length" class="workspace-menu-empty">选择本地目录，创建第一个工作区。</p>
   </div>
   <footer><button role="menuitem" class="workspace-create" @click="action('create')"><Plus :size="16"/><span>新建工作区</span></button><button v-if="workspaces.length>5&&!expanded" role="menuitem" class="workspace-expand" aria-label="展示全部工作区" title="展示全部工作区" @click="expand"><ChevronDown :size="17"/></button></footer>
  </div>
 </Teleport>
</template>

<style scoped>
.workspace-menu{position:fixed;z-index:3200;box-sizing:border-box;display:flex;flex-direction:column;background:var(--bg);color:var(--text);border:1px solid var(--line);border-radius:10px;box-shadow:0 12px 40px #22243625;overflow:hidden;font-family:var(--ui-font-family,inherit)}
.workspace-menu header{display:flex;align-items:center;justify-content:space-between;gap:8px;padding:8px 12px 6px;color:var(--secondary);font-size:.78rem;flex-shrink:0}.workspace-menu header>div{display:flex;gap:2px}.workspace-menu header small{color:var(--muted);margin-left:6px}
.workspace-menu-list{overflow-y:auto;overscroll-behavior:contain;min-height:0;padding:4px 6px;scrollbar-width:thin;scrollbar-color:var(--line) transparent}
.workspace-option{display:flex;align-items:center;gap:10px;text-align:left;width:100%;padding:9px 8px;box-sizing:border-box;border-radius:7px;color:var(--text)}.workspace-option:hover{background:var(--hover)}.workspace-option[aria-checked=true]{background:var(--accent-bg);color:var(--accent)}.workspace-option>svg{flex-shrink:0}.workspace-option-label{min-width:0;flex:1}.workspace-option-label strong,.workspace-option-label small{display:block;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.workspace-option-label strong{font-size:.86rem;font-weight:600;line-height:1.5}.workspace-option-label small{font-size:.71rem;color:var(--muted);margin-top:3px;line-height:1.5}.workspace-menu footer{display:flex;align-items:center;gap:4px;flex-shrink:0;border-top:1px solid var(--line);padding:6px}.workspace-create{flex:1;display:flex;align-items:center;gap:10px;text-align:left;padding:10px;color:var(--accent);border-radius:6px;font-size:.84rem}.workspace-expand{display:grid;place-items:center;align-self:stretch;width:36px;flex-shrink:0;color:var(--secondary);border-radius:6px}.workspace-menu footer button:hover{background:var(--hover)}.workspace-menu button:focus-visible{outline:2px solid var(--accent);outline-offset:-2px}.workspace-menu-empty{padding:10px;color:var(--muted);font-size:.8rem;line-height:1.7}
</style>
