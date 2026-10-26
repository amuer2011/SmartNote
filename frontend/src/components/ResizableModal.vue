<script setup lang="ts">
import { h } from 'vue'
import { NModal } from 'naive-ui'
import { Minus } from 'lucide-vue-next'
defineOptions({inheritAttrs:false})
const props=defineProps<{show:boolean;title?:string}>()
const emit=defineEmits<{'update:show':[value:boolean];minimize:[]}>()
const renderTitle=()=>h('div',{class:'resizable-modal-title'},[
 h('span',props.title),h('button',{class:'icon-button','aria-label':'隐藏'+props.title,title:'隐藏窗口，可在面板布局中恢复',onClick:()=>emit('minimize')},[h(Minus,{size:17})])
])
</script>
<template>
 <NModal v-bind="$attrs" :show="show" :title="renderTitle"  @update:show="emit('update:show',$event)">
  <slot/>
  <template v-if="$slots.footer" #footer><slot name="footer"/></template>
 </NModal>
</template>
<style scoped>.resizable-modal-title{display:flex;align-items:center;justify-content:space-between;gap:16px;padding-right:25px;min-width:0}.resizable-modal-title>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}</style>