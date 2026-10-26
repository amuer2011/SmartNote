<script setup lang="ts">
import {Download,Paperclip,FileText} from 'lucide-vue-next'
import type {Attachment,AttachmentPreview} from '../services/api'
defineProps<{attachment:Attachment;preview?:AttachmentPreview;loading?:boolean;error?:string;downloading?:boolean}>()
const emit=defineEmits<{download:[]}>()
</script>
<template>
 <section class="attachment-content" aria-label="附件内容预览"><header><div><h2>{{attachment.name}}</h2><p>{{(attachment.size/1024).toFixed(1)}} KB · {{attachment.mime}}</p></div><button class="secondary-button" :disabled="downloading" aria-label="下载当前附件" @click="emit('download')"><Download :size="16"/>下载附件</button></header>
 <p v-if="loading" class="attachment-message">正在读取附件…</p><p v-else-if="error" class="attachment-message" role="alert">{{error}}</p><pre v-else-if="preview?.kind==='text'" class="attachment-text">{{preview.content}}</pre><div v-else class="attachment-file"><Paperclip :size="64" :stroke-width="1.2"/><strong>{{attachment.name}}</strong><p>{{preview?.message||'此附件可下载后查看。'}}</p></div></section>
</template>
<style scoped>
.attachment-content{display:flex;flex:1;flex-direction:column;min-width:0;min-height:0;height:100%;color:var(--text)}header{display:flex;align-items:flex-start;justify-content:space-between;gap:15px;flex-wrap:wrap;padding:20px 26px;border-bottom:1px solid var(--line);flex-shrink:0}h2{font-size:1.1rem;margin:0 0 7px;overflow-wrap:anywhere}header p{font-size:.78rem;color:var(--muted);margin:0;overflow-wrap:anywhere}.attachment-text{margin:0;flex:1;overflow:auto;white-space:pre-wrap;overflow-wrap:anywhere;padding:24px 28px;font-family:var(--note-font-family,monospace);font-size:var(--note-font-size,14px);line-height:1.8;user-select:text}.attachment-file{display:flex;flex:1;min-height:220px;flex-direction:column;align-items:center;justify-content:center;padding:25px;gap:17px;text-align:center;color:var(--muted)}.attachment-file strong{color:var(--text);overflow-wrap:anywhere}.attachment-file p{font-size:.86rem}.attachment-message{padding:20px 26px;color:var(--muted)}
</style>
