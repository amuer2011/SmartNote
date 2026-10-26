<script setup lang="ts">
import { nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Minus, X } from 'lucide-vue-next'
const props=defineProps<{show:boolean;title:string;initialWidth?:number;initialHeight?:number;storageKey?:string}>()
const emit=defineEmits<{close:[];minimize:[]}>()
const host=ref<HTMLElement>(),x=ref(0),y=ref(0),width=ref(960),height=ref(720)
let initialized=false,observer:ResizeObserver|undefined,gesture:{kind:string;x:number;y:number;left:number;top:number;width:number;height:number}|undefined
let saveTimer:ReturnType<typeof setTimeout>|undefined
function saveBounds(){if(!props.storageKey||!initialized)return;try{localStorage.setItem('smartnote-window:'+props.storageKey,JSON.stringify({x:x.value,y:y.value,width:width.value,height:height.value}))}catch{}}
watch([x,y,width,height],()=>{clearTimeout(saveTimer);saveTimer=setTimeout(saveBounds,200)})
const clamp=(n:number,min:number,max:number)=>Math.max(min,Math.min(max,n))
function fit() {
 const maxWidth=Math.max(1,window.innerWidth-24),maxHeight=Math.max(1,window.innerHeight-24)
 width.value=clamp(width.value,Math.min(420,maxWidth),maxWidth);height.value=clamp(height.value,Math.min(280,maxHeight),maxHeight)
 x.value=clamp(x.value,12,Math.max(12,window.innerWidth-width.value-12));y.value=clamp(y.value,12,Math.max(12,window.innerHeight-height.value-12))
}
watch(()=>props.show,async show=>{if(!show) return; if(!initialized) {width.value=Math.min(props.initialWidth??960,window.innerWidth-24);height.value=Math.min(props.initialHeight??720,window.innerHeight-24);x.value=(window.innerWidth-width.value)/2;y.value=(window.innerHeight-height.value)/2;if(props.storageKey){try{const saved=JSON.parse(localStorage.getItem('smartnote-window:'+props.storageKey)||'null');if(saved&&['x','y','width','height'].every(key=>Number.isFinite(saved[key]))){x.value=saved.x;y.value=saved.y;width.value=saved.width;height.value=saved.height}}catch{}};initialized=true}; await nextTick();fit()},{immediate:true})
function start(event:PointerEvent,kind:string) {
 if(event.button!==0 || kind==='move' && (event.target as HTMLElement).closest('button')) return
 event.preventDefault();gesture={kind,x:event.clientX,y:event.clientY,left:x.value,top:y.value,width:width.value,height:height.value}
 ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}
function move(event:PointerEvent) {
 if(!gesture) return
 const dx=event.clientX-gesture.x,dy=event.clientY-gesture.y
 if(gesture.kind==='move') {x.value=gesture.left+dx;y.value=gesture.top+dy}
 else {
  const minW=Math.min(420,window.innerWidth-24),minH=Math.min(280,window.innerHeight-24)
  if(gesture.kind.includes('e'))width.value=clamp(gesture.width+dx,minW,window.innerWidth-gesture.left-12)
  if(gesture.kind.includes('s'))height.value=clamp(gesture.height+dy,minH,window.innerHeight-gesture.top-12)
  if(gesture.kind.includes('w')){x.value=clamp(gesture.left+dx,12,gesture.left+gesture.width-minW);width.value=gesture.left+gesture.width-x.value}
  if(gesture.kind.includes('n')){y.value=clamp(gesture.top+dy,12,gesture.top+gesture.height-minH);height.value=gesture.top+gesture.height-y.value}
 }
 fit()
}
function resizeKeys(event:KeyboardEvent) {
 if(!['ArrowLeft','ArrowRight','ArrowUp','ArrowDown'].includes(event.key)) return
 event.preventDefault();width.value+=(event.key==='ArrowRight'?10:event.key==='ArrowLeft'?-10:0);height.value+=(event.key==='ArrowDown'?10:event.key==='ArrowUp'?-10:0);fit()
}
onMounted(()=>{
 window.addEventListener('resize',fit)
 observer=new ResizeObserver(()=>{if(!props.show || !host.value) return; const rect=host.value.getBoundingClientRect();if(!rect.width || !rect.height) return;width.value=Math.round(rect.width);height.value=Math.round(rect.height);fit()})
 if(host.value) observer.observe(host.value)
})
onBeforeUnmount(()=>{window.removeEventListener('resize',fit);observer?.disconnect();clearTimeout(saveTimer);saveBounds()})
</script>
<template>
 <!-- 脱离侧栏的层叠上下文，主题变量继续从 :root 继承。 -->
 <Teleport to="body">
  <section v-show="show" ref="host" class="floating-window" role="dialog" :aria-label="title" :style="{left:x+'px',top:y+'px',width:width+'px',height:height+'px'}" @pointermove="move" @pointerup="gesture=undefined" @pointercancel="gesture=undefined" @lostpointercapture="gesture=undefined">
   <header class="floating-window-header" @pointerdown="start($event,'move')"><strong>{{title}}</strong><div><button class="icon-button" :aria-label="'最小化'+title" @click="emit('minimize')"><Minus :size="17"/></button><button class="icon-button" :aria-label="'关闭'+title" @click="emit('close')"><X :size="17"/></button></div></header>
   <div class="floating-window-content"><slot/></div>
   <div v-for="edge in ['n','s','e','w','ne','nw','sw']" :key="edge" class="floating-resize-edge" :class="'resize-'+edge" aria-hidden="true" @pointerdown="start($event,edge)"/>
   <div class="floating-resize-grip" role="separator" :aria-label="'调整'+title+'窗口大小'" tabindex="0" @pointerdown="start($event,'se')" @keydown="resizeKeys"/>
  </section>
 </Teleport>
</template>
<style scoped>
.floating-window{position:fixed;z-index:1200;display:flex;flex-direction:column;resize:none;overflow:hidden;min-width:min(420px,calc(100vw - 24px));min-height:min(280px,calc(100vh - 24px));max-width:calc(100vw - 24px);max-height:calc(100vh - 24px);background:var(--bg);color:var(--text);border:1px solid var(--line);border-radius:10px;box-shadow:0 12px 60px #0004}
.floating-window-header{flex:0 0 44px;display:flex;align-items:center;justify-content:space-between;padding:6px 12px 6px 20px;border-bottom:1px solid var(--line);cursor:move;user-select:none;touch-action:none;font-size:1rem}.floating-window-header>div{display:flex;gap:6px}
.floating-window-content{flex:1;min-height:0;display:flex;overflow:auto;padding-bottom:12px}
.floating-resize-grip{position:absolute;bottom:2px;right:2px;width:15px;height:15px;cursor:nwse-resize;touch-action:none;background:linear-gradient(135deg,transparent 40%,var(--muted) 41%,var(--muted) 48%,transparent 49%,transparent 65%,var(--muted) 66%,var(--muted) 73%,transparent 74%);opacity:.65}
.floating-resize-edge{position:absolute;touch-action:none;z-index:2}.resize-n,.resize-s{height:6px;left:12px;right:12px;cursor:ns-resize}.resize-n{top:0}.resize-s{bottom:0}.resize-e,.resize-w{width:6px;top:12px;bottom:12px;cursor:ew-resize}.resize-e{right:0}.resize-w{left:0}.resize-ne,.resize-nw,.resize-sw{width:12px;height:12px}.resize-ne{right:0;top:0;cursor:nesw-resize}.resize-nw{left:0;top:0;cursor:nwse-resize}.resize-sw{left:0;bottom:0;cursor:nesw-resize}.floating-resize-grip{z-index:3}
</style>
