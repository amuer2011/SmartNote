<script setup lang="ts">
import {ref} from 'vue'
import {Search,X} from 'lucide-vue-next'

defineProps<{modelValue:string}>()

const emit=defineEmits<{'update:modelValue':[value:string];close:[]}>()
const root=ref<HTMLElement>()
const input=ref<HTMLInputElement>()

function focus(){input.value?.focus();input.value?.select()}

function updateValue(event:Event){emit('update:modelValue',(event.target as HTMLInputElement).value)}

function clear(){emit('update:modelValue','');focus()}

function onEscape(event:KeyboardEvent){
 if(event.isComposing)return
 event.preventDefault();event.stopPropagation();emit('close')
}

defineExpose({focus})
</script>

<template>
 <div ref="root" class="navigation-search" role="search" aria-label="导航栏查询" @keydown.esc.prevent.stop="onEscape">
  <Search :size="15" aria-hidden="true"/>
  <input ref="input" :value="modelValue" type="text" autocomplete="off" spellcheck="false" aria-label="导航栏查询" placeholder="查找文件名或正文关键词…" @input="updateValue"/>
  <button v-if="modelValue" type="button" aria-label="清空导航查询" title="清空" @click="clear"><X :size="14" aria-hidden="true"/></button>
  <button type="button" aria-label="关闭导航查询" title="关闭" @click="emit('close')"><X :size="14" aria-hidden="true"/></button>
 </div>
</template>

<style scoped>
.navigation-search{display:flex;flex-shrink:0;margin-bottom:10px;align-items:center;gap:5px;width:100%;min-width:0;box-sizing:border-box;padding:6px 7px;color:var(--muted);background:var(--soft);border:1px solid var(--line);border-radius:7px;transition:border-color .15s ease}
.navigation-search:focus-within{border-color:var(--accent)}
.navigation-search>svg{flex:0 0 auto;color:var(--muted);transition:color .15s ease}
.navigation-search:focus-within>svg{color:var(--accent)}
.navigation-search input{flex:1 1 auto;width:0;min-width:0;padding:2px 0;border:0;outline:0;background:transparent;color:var(--text);font:inherit;font-size:.82rem;line-height:1.4}
.navigation-search input::placeholder{color:var(--muted);opacity:1}
.navigation-search button{display:inline-flex;align-items:center;justify-content:center;flex:0 0 auto;width:22px;height:22px;padding:0;border:0;border-radius:4px;background:transparent;color:var(--muted);cursor:pointer;transition:color .15s ease}
.navigation-search button:hover{color:var(--accent)}
.navigation-search button:focus-visible{outline:2px solid var(--accent);outline-offset:1px}
</style>
