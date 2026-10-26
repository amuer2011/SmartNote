<script setup lang="ts">
import {computed,nextTick,watch,ref,onMounted,onBeforeUnmount} from 'vue'
import {Plus,X,FileText} from 'lucide-vue-next'
import {contextMenuPosition} from '../services/context-menu'
const props=defineProps<{tabs:{key:string;title:string;path?:string}[];active:string;disabled:boolean}>()
const emit=defineEmits<{select:[key:string];close:[key:string];new:[];closeGroup:[mode:'all'|'others'|'right',key:string]}>()
const root=ref<HTMLElement>()
function revealActive(){
 const container=root.value,selected=container?.querySelector<HTMLElement>('[aria-selected=true]');if(!container||!selected)return
 const bounds=container.getBoundingClientRect(),tab=selected.getBoundingClientRect()
 if(tab.top<bounds.top)container.scrollTop+=tab.top-bounds.top
 else if(tab.bottom>bounds.bottom)container.scrollTop+=tab.bottom-bounds.bottom
}
watch(()=>props.active,()=>void nextTick(revealActive))
let observer:ResizeObserver|undefined
onMounted(()=>{if(root.value){observer=new ResizeObserver(revealActive);observer.observe(root.value)}})
onBeforeUnmount(()=>{observer?.disconnect();closeMenu()})

type CloseGroupMode='all'|'others'|'right'
const menuKey=ref<string>()
const menuPosition=ref({x:0,y:0})
const menuReady=ref(false)
const menuEl=ref<HTMLElement>()
const menuTabEl=ref<HTMLElement>()
const menuIndex=computed(()=>props.tabs.findIndex(tab=>tab.key===menuKey.value))
const canCloseOthers=computed(()=>props.tabs.length>1)
const canCloseRight=computed(()=>menuIndex.value>=0&&menuIndex.value<props.tabs.length-1)
let anchorX=0,anchorY=0,menuListening=false

function tabElement(key:string){return Array.from(root.value?.querySelectorAll<HTMLElement>('[data-tab-key]')||[]).find(tab=>tab.dataset.tabKey===key)}
function menuButtons(){return menuEl.value?Array.from(menuEl.value.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')):[]}
function focusMenuItem(index:number){void nextTick(()=>{if(menuKey.value)menuButtons()[index]?.focus()})}
function positionMenu(){
 const element=menuEl.value;if(!element)return
 const bounds=element.getBoundingClientRect()
 menuPosition.value=contextMenuPosition(anchorX,anchorY,bounds.width,bounds.height)
}
function openMenu(key:string,x:number,y:number,tab?:HTMLElement){
 if(props.disabled||!props.tabs.some(item=>item.key===key))return
 const reopening=!!menuKey.value
 anchorX=x;anchorY=y;menuKey.value=key;menuTabEl.value=tab||tabElement(key)||undefined
 menuReady.value=reopening
 menuPosition.value=contextMenuPosition(x,y,190,132)
 addMenuListeners()
 void nextTick(()=>{
  if(!menuKey.value)return
  positionMenu();menuReady.value=true
  void nextTick(()=>{if(menuKey.value)menuButtons()[0]?.focus()})
 })
}
function closeMenu(restoreFocus=false){
 const tab=menuTabEl.value,wasOpen=!!menuKey.value
 menuKey.value=undefined;menuTabEl.value=undefined;menuReady.value=false
 removeMenuListeners()
 if(restoreFocus&&wasOpen&&tab?.isConnected)void nextTick(()=>{if(tab.isConnected)tab.focus()})
}
function addMenuListeners(){
 if(menuListening)return
 menuListening=true
 window.addEventListener('keydown',menuKeydown,true)
 document.addEventListener('pointerdown',outsidePointer,true)
 window.addEventListener('scroll',menuScroll,true)
 window.addEventListener('resize',menuResize)
}
function removeMenuListeners(){
 if(!menuListening)return
 menuListening=false
 window.removeEventListener('keydown',menuKeydown,true)
 document.removeEventListener('pointerdown',outsidePointer,true)
 window.removeEventListener('scroll',menuScroll,true)
 window.removeEventListener('resize',menuResize)
}
function menuKeydown(event:KeyboardEvent){
 if(!menuKey.value)return
 if(event.key==='Escape'){event.preventDefault();event.stopPropagation();closeMenu(true);return}
 if(!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return
 event.preventDefault();event.stopPropagation()
 const buttons=menuButtons();if(!buttons.length)return
 const index=buttons.indexOf(document.activeElement as HTMLButtonElement)
 const next=event.key==='Home'?0:event.key==='End'?buttons.length-1:index<0?(event.key==='ArrowDown'?0:buttons.length-1):(index+(event.key==='ArrowDown'?1:-1)+buttons.length)%buttons.length
 buttons[next]?.focus()
}
function outsidePointer(event:PointerEvent){const target=event.target;if(target instanceof Node&&menuEl.value?.contains(target))return;closeMenu()}
function menuScroll(event:Event){const target=event.target;if(target instanceof Node&&menuEl.value?.contains(target))return;closeMenu()}
function menuResize(){closeMenu()}
function showMenu(event:MouseEvent,key:string){
 if(props.disabled)return
 const tab=event.currentTarget as HTMLElement,bounds=tab.getBoundingClientRect(),keyboard=!event.clientX&&!event.clientY
 openMenu(key,keyboard?bounds.left+bounds.width/2:event.clientX,keyboard?bounds.bottom:event.clientY,tab)
}
function runMenu(mode:CloseGroupMode){
 const key=menuKey.value
 if(!key||props.disabled||mode==='others'&&!canCloseOthers.value||mode==='right'&&!canCloseRight.value)return
 emit('closeGroup',mode,key)
 closeMenu(mode!=='all')
}
watch(()=>props.disabled,value=>{if(value)closeMenu()})
watch(()=>props.tabs.map(tab=>tab.key).join('|'),()=>{if(menuKey.value&&!props.tabs.some(tab=>tab.key===menuKey.value))closeMenu()})

function keys(event:KeyboardEvent){
 if(props.disabled)return
 const target=event.target
 const tab=target instanceof Element?target.closest<HTMLElement>('[data-tab-key]'):null
 if(tab&&(event.key==='ContextMenu'||event.shiftKey&&event.key==='F10')){
  event.preventDefault();event.stopPropagation();const bounds=tab.getBoundingClientRect()
  openMenu(tab.dataset.tabKey!,bounds.left+bounds.width/2,bounds.bottom,tab);return
 }
 if(!['ArrowLeft','ArrowRight','Home','End'].includes(event.key)||!props.tabs.length)return
 event.preventDefault();event.stopPropagation();const i=props.tabs.findIndex(t=>t.key===props.active),n=props.tabs.length
 const next=event.key==='Home'?0:event.key==='End'?n-1:(i+(event.key==='ArrowRight'?1:n-1))%n
 emit('select',props.tabs[next]!.key);void nextTick(()=>root.value?.querySelector<HTMLElement>('[aria-selected=true]')?.focus())
}
</script>
<template><div class="note-tabs-bar"><div ref="root" class="note-tabs" role="tablist" aria-label="笔记标签页" @keydown="keys"><div v-for="tab in tabs" :key="tab.key" class="note-tab" role="tab" :data-tab-key="tab.key" :aria-selected="active===tab.key" :tabindex="active===tab.key?0:-1" :title="tab.path||tab.title" :aria-disabled="disabled" @click="!disabled&&emit('select',tab.key)" @keydown.enter.prevent="!disabled&&emit('select',tab.key)" @keydown.space.prevent="!disabled&&emit('select',tab.key)" @auxclick.middle.prevent="!disabled&&emit('close',tab.key)" @contextmenu.prevent.stop="showMenu($event,tab.key)"><FileText :size="14"/><span>{{tab.title}}</span><button class="icon-button" :aria-label="'关闭标签页 '+tab.title" :disabled="disabled" @click.stop="emit('close',tab.key)"><X :size="13"/></button></div></div><button class="icon-button new-tab" aria-label="新标签页" title="新标签页" :disabled="disabled" @click="emit('new')"><Plus :size="17"/></button></div>
<Teleport to="body"><div v-if="menuKey" ref="menuEl" class="entry-context-menu note-tabs-context-menu" role="menu" aria-label="标签页操作" :style="{left:menuPosition.x+'px',top:menuPosition.y+'px',visibility:menuReady?'visible':'hidden'}" @contextmenu.prevent><button type="button" role="menuitem" @click="runMenu('all')">全部关闭</button><button type="button" role="menuitem" :disabled="!canCloseOthers" @click="runMenu('others')">关闭其他</button><button type="button" role="menuitem" :disabled="!canCloseRight" @click="runMenu('right')">关闭右侧全部</button></div></Teleport></template>
<style scoped>
.note-tabs-bar{display:flex;align-items:flex-start;min-height:39px;flex-shrink:0;border-bottom:1px solid var(--line);background:var(--soft);padding:3px 8px 0;gap:4px}.note-tabs{display:flex;min-width:0;flex-wrap:wrap;flex:0 1 auto;align-self:stretch;gap:3px;max-height:30vh;overflow-y:auto;overflow-x:hidden;scrollbar-width:thin}.note-tab{display:flex;align-items:center;gap:7px;min-width:0;max-width:min(220px,100%);min-height:35px;padding:0 8px 0 12px;border:1px solid transparent;border-bottom:0;border-radius:7px 7px 0 0;color:var(--muted);cursor:pointer;font-size:.8rem;flex-shrink:0}.note-tab[aria-selected=true]{background:var(--bg);border-color:var(--line);color:var(--text)}.note-tab:hover{background:var(--bg)}.note-tab>svg{flex-shrink:0;color:var(--accent)}.note-tab>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap;min-width:0;max-width:155px}.note-tab button{height:22px;width:22px;flex-shrink:0;margin-left:5px;opacity:.55}.note-tab:hover button,.note-tab[aria-selected=true] button{opacity:1}.new-tab{flex-shrink:0;margin:3px 0}.note-tab:focus-visible{outline:2px solid var(--accent);outline-offset:-3px}.note-tabs-context-menu{width:190px;min-width:190px}
</style>
