<script setup lang="ts">
import {FileText,X} from 'lucide-vue-next'
import type {AssistantAttachment} from '../services/api'
import {attachmentSize,attachmentImage} from '../services/chat-attachments'
defineProps<{items:AssistantAttachment[];removable?:boolean;disabled?:boolean}>()
const emit=defineEmits<{remove:[id:string]}>()
const size=(a:AssistantAttachment)=>{const n=attachmentSize(a);return n<1024?`${n} B`:n<1048576?`${(n/1024).toFixed(1)} KB`:`${(n/1048576).toFixed(1)} MB`}
</script>
<template><div v-if="items.length" class="chat-attachments" aria-label="聊天附件"><div v-for="item in items" :key="item.id" class="chat-attachment" :title="item.name"><img v-if="attachmentImage(item)" :src="attachmentImage(item)" :alt="item.name"/><FileText v-else :size="23"/><span><strong>{{item.name}}</strong><small>{{size(item)}}</small></span><button v-if="removable" class="icon-button" :aria-label="'移除附件 '+item.name" :disabled="disabled" @click="emit('remove',item.id)"><X :size="13"/></button></div></div></template>
<style scoped>
.chat-attachments{display:flex;flex-wrap:wrap;gap:7px;margin:8px 0;max-height:200px;overflow:auto}.chat-attachment{display:flex;align-items:center;gap:8px;max-width:100%;min-width:0;border:1px solid var(--line);background:var(--bg);border-radius:8px;padding:7px 9px;font-size:.76rem}.chat-attachment>svg{color:var(--accent);flex-shrink:0}.chat-attachment img{width:42px;height:42px;object-fit:contain;border-radius:4px}.chat-attachment>span{min-width:0}.chat-attachment strong{display:block;font-weight:500;max-width:190px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.chat-attachment small{display:block;color:var(--muted);font-size:.65rem;margin-top:4px}.chat-attachment button{flex-shrink:0;width:23px;height:23px}
</style>
