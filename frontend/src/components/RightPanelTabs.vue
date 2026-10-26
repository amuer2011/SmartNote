<script setup lang="ts">
import {computed,nextTick,ref} from 'vue'
import {X} from 'lucide-vue-next'

const props=defineProps<{
  tabs:Array<{id:string;label:string;icon:any}>
  modelValue:string
}>()

const emit=defineEmits<{
  'update:modelValue':[id:string]
  close:[id:string]
  reorder:[ids:string[]]
}>()

const tabElements=new Map<string,HTMLElement>()
const dragId=ref<string|null>(null)
const dropTarget=ref<string|null>(null)
const dropPosition=ref<'before'|'after'>('before')

const activeId=computed(()=>props.tabs.some(tab=>tab.id===props.modelValue)?props.modelValue:props.tabs[0]?.id??'')

function setTabElement(id:string,element:unknown){
  if(element)tabElements.set(id,element as HTMLElement)
  else tabElements.delete(id)
}

function select(id:string){
  if(id!==props.modelValue)emit('update:modelValue',id)
}

function activateFromTab(event:KeyboardEvent,id:string){
  if(event.target!==event.currentTarget)return
  event.preventDefault()
  select(id)
}

function onTablistKeydown(event:KeyboardEvent){
  const keys=['ArrowRight','ArrowLeft','Home','End']
  if(!keys.includes(event.key)||!props.tabs.length)return
  event.preventDefault()
  event.stopPropagation()
  const index=Math.max(0,props.tabs.findIndex(tab=>tab.id===activeId.value))
  const next=event.key==='Home'?0
    :event.key==='End'?props.tabs.length-1
    :event.key==='ArrowRight'?(index+1)%props.tabs.length
    :(index+props.tabs.length-1)%props.tabs.length
  const id=props.tabs[next]?.id
  if(!id)return
  select(id)
  void nextTick(()=>tabElements.get(id)?.focus())
}

function onDragStart(event:DragEvent,id:string){
  dragId.value=id
  dropTarget.value=null
  if(event.dataTransfer){
    event.dataTransfer.effectAllowed='move'
    event.dataTransfer.setData('text/plain',id)
  }
}

function onDragOver(event:DragEvent,id:string){
  if(!dragId.value)return
  event.preventDefault()
  if(event.dataTransfer)event.dataTransfer.dropEffect='move'
  if(dragId.value===id){
    dropTarget.value=null
    return
  }
  const rect=(event.currentTarget as HTMLElement).getBoundingClientRect()
  dropTarget.value=id
  dropPosition.value=event.clientX>rect.left+rect.width/2?'after':'before'
}

function onDragLeave(event:DragEvent,id:string){
  const current=event.currentTarget as HTMLElement
  if(event.relatedTarget instanceof Node&&current.contains(event.relatedTarget))return
  if(dropTarget.value===id)dropTarget.value=null
}

function onDrop(event:DragEvent,targetId:string){
  event.preventDefault()
  const source=dragId.value
  const position=dropPosition.value
  dragId.value=null
  dropTarget.value=null
  if(!source||source===targetId)return

  const ids=props.tabs.map(tab=>tab.id)
  const from=ids.indexOf(source)
  if(from<0)return
  ids.splice(from,1)
  let to=ids.indexOf(targetId)
  if(to<0)return
  if(position==='after')to+=1
  ids.splice(to,0,source)

  if(ids.some((id,index)=>id!==props.tabs[index]?.id))emit('reorder',ids)
}

function onDragEnd(){
  dragId.value=null
  dropTarget.value=null
}
</script>

<template>
  <div
    class="right-panel-tabs"
    role="tablist"
    aria-label="右侧栏页签"
    aria-orientation="horizontal"
    @keydown="onTablistKeydown"
  >
    <div
      v-for="tab in tabs"
      :id="'right-tab-'+tab.id"
      :key="tab.id"
      :ref="element=>setTabElement(tab.id,element)"
      class="right-panel-tab"
      role="tab"
      :aria-label="tab.label"
      :aria-selected="activeId===tab.id"
      :aria-controls="'right-content-'+tab.id"
      :tabindex="activeId===tab.id?0:-1"
      :title="tab.label"
      draggable="true"
      :class="{
        dragging:dragId===tab.id,
        'drop-before':dropTarget===tab.id&&dropPosition==='before',
        'drop-after':dropTarget===tab.id&&dropPosition==='after'
      }"
      @click="select(tab.id)"
      @keydown.enter="activateFromTab($event,tab.id)"
      @keydown.space="activateFromTab($event,tab.id)"
      @dragstart="onDragStart($event,tab.id)"
      @dragover="onDragOver($event,tab.id)"
      @dragleave="onDragLeave($event,tab.id)"
      @drop="onDrop($event,tab.id)"
      @dragend="onDragEnd"
    >
      <component :is="tab.icon" :size="15" aria-hidden="true" />
      <span class="tab-label">{{tab.label}}</span>
      <button
        type="button"
        class="tab-close"
        :aria-label="'关闭'+tab.label"
        :title="'关闭'+tab.label"
        @click.stop="emit('close',tab.id)"
        @mousedown.stop
        @dragstart.stop.prevent
      >
        <X :size="12" aria-hidden="true" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.right-panel-tabs{display:flex;align-self:stretch;flex:1;min-width:0;align-items:stretch;gap:2px;overflow-x:auto;overflow-y:hidden;scrollbar-width:none}
.right-panel-tabs::-webkit-scrollbar{display:none}
.right-panel-tab{display:flex;align-items:center;justify-content:center;gap:5px;position:relative;flex:0 0 auto;padding:0 6px 0 8px;border:0;border-radius:4px;background:transparent;color:var(--muted);font:inherit;font-size:.76rem;white-space:nowrap;cursor:pointer;user-select:none;transition:background-color .12s,color .12s,opacity .12s}
.right-panel-tab:hover{background:color-mix(in srgb,var(--line) 55%,transparent);color:var(--text)}
.right-panel-tab[aria-selected=true]{color:var(--accent)}
.right-panel-tab[aria-selected=true]::after{content:'';position:absolute;bottom:-1px;left:8px;right:8px;height:2px;border-radius:2px;background:var(--accent)}
.right-panel-tab:focus-visible{outline:2px solid var(--accent);outline-offset:-3px}
.right-panel-tab.dragging{opacity:.4;cursor:grabbing}
.right-panel-tab.drop-before::before,.right-panel-tab.drop-after::before{content:'';position:absolute;top:4px;bottom:4px;width:2px;border-radius:2px;background:var(--accent)}
.right-panel-tab.drop-before::before{left:-1px}
.right-panel-tab.drop-after::before{right:-1px}
.tab-label{overflow:hidden;text-overflow:ellipsis}
.tab-close{display:inline-flex;align-items:center;justify-content:center;flex:0 0 auto;width:16px;height:16px;padding:0;border:0;border-radius:3px;background:transparent;color:var(--muted);opacity:.65;cursor:pointer;transition:background-color .12s,color .12s,opacity .12s}
.tab-close:hover{background:var(--line);color:var(--text);opacity:1}
.tab-close:focus-visible{outline:2px solid var(--accent);outline-offset:-2px;opacity:1}
</style>
