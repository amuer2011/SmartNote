<script setup lang="ts">
import { ref } from 'vue'
defineProps<{ label:string; value:number }>()
const emit=defineEmits<{ change:[delta:number]; finish:[] }>()
const active=ref(false)
let lastX=0
function start(event:PointerEvent) {
 if(event.button!==0) return
 event.preventDefault(); active.value=true; lastX=event.clientX
 ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}
function move(event:PointerEvent) { if(!active.value) return; const delta=event.clientX-lastX; lastX=event.clientX; emit('change',delta) }
function finish() {if(active.value) {active.value=false; emit('finish')}}
function keys(event:KeyboardEvent) {if(!['ArrowLeft','ArrowRight'].includes(event.key)) return; event.preventDefault(); emit('change',event.key==='ArrowLeft'?-10:10); emit('finish')}
</script>
<template><div class="panel-divider" :class="{resizing:active}" role="separator" aria-orientation="vertical" :aria-label="label" :aria-valuenow="Math.round(value)" tabindex="0" @pointerdown="start" @pointermove="move" @pointerup="finish" @pointercancel="finish" @lostpointercapture="finish" @keydown="keys"/></template>
<style scoped>
.panel-divider{width:6px;flex:0 0 6px;position:relative;cursor:col-resize;touch-action:none;user-select:none;background:var(--bg);z-index:2}
.panel-divider:after{content:'';position:absolute;left:2px;top:0;bottom:0;width:1px;background:var(--line)}
.panel-divider:hover:after,.panel-divider:focus-visible:after,.panel-divider.resizing:after{left:1px;width:4px;background:var(--accent)}
</style>
