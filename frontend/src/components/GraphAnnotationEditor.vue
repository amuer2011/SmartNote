<script setup lang="ts">
import { nextTick, onMounted, ref } from 'vue'
const props = defineProps<{text:string;left:number;top:number;width:number;maxHeight:number;kind:'node'|'edge';saving:boolean;error:string}>()
const emit = defineEmits<{save:[text:string];cancel:[]}>()
const draft = ref(props.text), input = ref<HTMLTextAreaElement>()
onMounted(() => nextTick(() => {input.value?.focus();input.value?.select()}))
function keydown(event:KeyboardEvent) {
 event.stopPropagation()
 if(event.isComposing || event.keyCode===229)return
 if(event.key==='Escape'){event.preventDefault();if(!props.saving)emit('cancel')}
 if(event.key==='Enter'&&!event.shiftKey){event.preventDefault();emit('save',draft.value)}
}
</script>
<template>
 <section class="graph-inline-editor" role="dialog" aria-label="编辑拓扑备注" :style="{left:left+'px',top:top+'px',width:width+'px',maxHeight:maxHeight+'px'}" @pointerdown.stop @click.stop @wheel.stop @keydown="keydown">
  <strong>{{kind==='node'?'笔记备注':'连线备注'}}</strong>
  <textarea ref="input" v-model="draft" :aria-label="kind==='node'?'图上笔记备注':'图上连线备注'" maxlength="4000" :readonly="saving" placeholder="输入备注…" />
  <p v-if="error" role="alert">{{error}}</p>
  <footer><small>Enter 保存 · Shift+Enter 换行 · Esc 取消</small><div><button class="secondary-button" :disabled="saving" @click="emit('cancel')">取消编辑</button><button class="primary-button" :disabled="saving" @click="emit('save',draft)">{{saving?'保存中…':'保存备注'}}</button></div></footer>
 </section>
</template>
<style scoped>
.graph-inline-editor{position:absolute;z-index:6;box-sizing:border-box;overflow:auto;padding:12px;border:1px solid var(--accent);border-radius:10px;background:var(--bg);color:var(--text);box-shadow:0 8px 30px #0002;font-size:12px}.graph-inline-editor strong{display:block;margin-bottom:8px;color:var(--accent)}textarea{box-sizing:border-box;display:block;width:100%;height:96px;min-height:40px;resize:vertical;border:1px solid var(--line);border-radius:6px;background:var(--soft);color:var(--text);font:inherit;line-height:1.6;padding:8px;outline-color:var(--accent)}footer{display:flex;flex-direction:column;gap:9px;margin-top:9px}small{color:var(--muted);font-size:11px}footer div{display:flex;justify-content:flex-end;gap:8px}button{padding:5px 10px;font-size:12px}p{color:var(--danger,#c35);overflow-wrap:anywhere;line-height:1.6}
</style>
