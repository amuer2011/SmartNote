<script setup lang="ts">
import {watch,onBeforeUnmount,ref,computed} from 'vue'
import {Cloud,CloudAlert,RefreshCw} from 'lucide-vue-next'
import {api,desktop,type SyncStatus} from '../services/api'
const props=defineProps<{workspaceId:string}>(),emit=defineEmits<{open:[];changed:[]}>()
const status=ref<SyncStatus>();let seq=0,disposed=false,flight=false
const label=computed(()=>({syncing:'正在同步',synced:'已同步',conflict:'同步冲突',error:'同步未完成'})[status.value?.state||'']||'本地模式')
async function poll(){if(!desktop||!props.workspaceId||flight||disposed)return;const token=seq,id=props.workspaceId;flight=true;try{const next=await api.GetSyncStatus(id);if(disposed||seq!==token)return;const previous=status.value;if(previous?.state==='syncing'&&next.state!=='syncing'||next.lastSuccess&&next.lastSuccess!==previous?.lastSuccess)emit('changed');status.value=next}catch{/* An older preview bridge may omit sync. */}finally{flight=false}}
watch(()=>props.workspaceId,()=>{seq++;status.value=undefined;void poll()},{immediate:true})
const timer=setInterval(()=>void poll(),2000);onBeforeUnmount(()=>{disposed=true;seq++;clearInterval(timer)})
</script>
<template><button class="sync-indicator" :class="status?.state" :title="status?.message||'配置 WebDAV 多设备同步'" aria-label="打开同步设置" @click="emit('open')"><RefreshCw v-if="status?.state==='syncing'" :size="14" class="spin"/><CloudAlert v-else-if="status?.state==='error'||status?.state==='conflict'" :size="14"/><Cloud v-else :size="14"/>{{label}}</button></template>
<style scoped>.sync-indicator{border:0;background:transparent;color:var(--muted);display:flex;align-items:center;gap:5px;font-size:.78rem;white-space:nowrap;cursor:pointer}.error,.conflict{color:#cb654d}.spin{animation:sync-spin 1.5s linear infinite}@keyframes sync-spin{to{transform:rotate(360deg)}}</style>
