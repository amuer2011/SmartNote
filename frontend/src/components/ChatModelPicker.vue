<script setup lang="ts">
import {computed,nextTick,onBeforeUnmount,onMounted,ref,useId,watch} from 'vue'
import {Check,ChevronDown,Search,SlidersHorizontal} from 'lucide-vue-next'
import type {LLMProvider} from '../services/api'
import ReasoningSelect from './ReasoningSelect.vue'
const props=defineProps<{providers:LLMProvider[];providerId:string;model:string;reasoning:string;disabled?:boolean;visible:boolean}>()
const emit=defineEmits<{select:[providerId:string,model:string];'update:reasoning':[value:string];pending:[value:boolean]}>()
const root=ref<HTMLElement>(),menu=ref<HTMLElement>(),search=ref<HTMLInputElement>(),trigger=ref<HTMLButtonElement>()
const open=ref(false),query=ref(''),active=ref(-1),position=ref({left:'0px',top:'0px',width:'400px',maxHeight:'500px'})
const id='model-menu-'+useId()
const provider=computed(()=>props.providers.find(p=>p.id===props.providerId))
const selectedLabel=computed(()=>provider.value&&props.model?`${provider.value.name} / ${props.model}`:'选择模型')
const groups=computed(()=>{
 const term=query.value.trim().toLocaleLowerCase()
 return props.providers.map(p=>({id:p.id,name:p.name,models:[...new Set(p.selectedModels.length?p.selectedModels:[...p.models,...(p.model?[p.model]:[])])].filter(model=>`${p.name} ${model}`.toLocaleLowerCase().includes(term))})).filter(p=>p.models.length)
})
const rows=computed(()=>groups.value.flatMap(p=>p.models.map(model=>({providerId:p.id,model}))))
function place(){
 if(!open.value||!root.value)return
 const r=root.value.getBoundingClientRect(),width=Math.min(430,window.innerWidth-24),above=Math.max(0,r.top-12),below=Math.max(0,window.innerHeight-r.bottom-12),space=Math.max(above,below)
 const maxHeight=Math.min(540,Math.max(180,space)),height=Math.min(menu.value?.scrollHeight||400,maxHeight)
 position.value={left:Math.max(12,Math.min(r.left,window.innerWidth-width-12))+'px',width:width+'px',top:Math.max(8,above>=Math.min(height,260)||above>=below?r.top-height-7:r.bottom+7)+'px',maxHeight:maxHeight+'px'}
}
async function show(){if(props.disabled)return;open.value=true;query.value='';await nextTick();active.value=rows.value.findIndex(r=>r.providerId===props.providerId&&r.model===props.model);await nextTick();place();await nextTick();search.value?.focus();menu.value?.querySelector('[aria-selected=true]')?.scrollIntoView({block:'nearest'})}
function close(focus=false){open.value=false;if(focus)void nextTick(()=>trigger.value?.focus())}
function choose(providerId:string,model:string){if(props.disabled)return;active.value=rowIndex(providerId,model);emit('select',providerId,model)}
function key(event:KeyboardEvent){
 if(event.key==='Escape'){event.preventDefault();event.stopPropagation();close(true);return}
 if((event.target as HTMLElement).matches('input[type=range]'))return
 if(!['ArrowDown','ArrowUp','Home','End','Enter'].includes(event.key)||!rows.value.length)return
 if(['Home','End'].includes(event.key)&&event.target===search.value)return
 event.preventDefault();event.stopPropagation()
 if(event.key==='Enter'){const row=rows.value[active.value];if(row)choose(row.providerId,row.model);return}
 active.value=event.key==='Home'?0:event.key==='End'?rows.value.length-1:(active.value+(event.key==='ArrowDown'?1:rows.value.length-1)+rows.value.length)%rows.value.length
 void nextTick(()=>{const row=menu.value?.querySelector<HTMLElement>('[data-key-active=true]');row?.scrollIntoView({block:'nearest'});if((event.target as HTMLElement).closest('[role=option]'))row?.focus()})
}
function outside(event:PointerEvent){if(open.value&&event.target instanceof Node&&!root.value?.contains(event.target)&&!menu.value?.contains(event.target))close()}
function rowIndex(providerId:string,model:string){return rows.value.findIndex(row=>row.providerId===providerId&&row.model===model)}
watch(query,()=>{active.value=rows.value.length?0:-1;void nextTick(place)})
watch(()=>[props.disabled,props.visible],()=>{if(props.disabled||!props.visible)close()})
watch(()=>[props.providerId,props.model,props.providers],()=>{if(open.value)void nextTick(place)})
onMounted(()=>{document.addEventListener('pointerdown',outside,true);window.addEventListener('resize',place);document.addEventListener('scroll',place,true)})
onBeforeUnmount(()=>{document.removeEventListener('pointerdown',outside,true);window.removeEventListener('resize',place);document.removeEventListener('scroll',place,true)})
</script>
<template><div ref="root" class="ai-model-picker" @keydown.esc.prevent.stop="close(true)"><button ref="trigger" type="button" class="ai-model-trigger" aria-label="供应商与模型" aria-haspopup="dialog" :aria-expanded="open" :aria-controls="id" :title="selectedLabel" :disabled="disabled" @click="open?close():show()" @keydown.down.prevent="show()"><span>{{selectedLabel}}</span><ChevronDown :size="12"/></button></div>
<Teleport to="body"><section v-show="open" :id="id" ref="menu" class="chat-model-menu" :style="position" role="dialog" aria-label="供应商与模型选项" @keydown="key"><header class="model-menu-search"><Search :size="15"/><input ref="search" v-model="query" role="combobox" aria-autocomplete="list" aria-label="搜索供应商或模型" :aria-controls="id+'-list'" :aria-expanded="open" :aria-activedescendant="active>=0?id+'-option-'+active:undefined" autocomplete="off" placeholder="搜索模型或供应商…"/></header><div :id="id+'-list'" class="model-menu-list" role="listbox" aria-label="模型列表"><div v-for="group in groups" :key="group.id" class="model-menu-group" role="group" :aria-label="group.name"><h4>{{group.name}}</h4><button v-for="modelId in group.models" :id="id+'-option-'+rowIndex(group.id,modelId)" :key="modelId" type="button" role="option" :aria-selected="providerId===group.id&&model===modelId" :aria-label="group.name+' / '+modelId" :data-provider-id="group.id" :data-model-id="modelId" :data-key-active="active===rowIndex(group.id,modelId)" :title="group.name+' / '+modelId" @focus="active=rowIndex(group.id,modelId)" @pointermove="active=rowIndex(group.id,modelId)" @click="choose(group.id,modelId)"><Check v-if="providerId===group.id&&model===modelId" :size="14"/><span v-else class="model-check-space"/><span>{{modelId}}</span></button></div><p v-if="!groups.length" class="no-models">{{query?'没有匹配的模型':'请在大模型配置中添加或启用模型。'}}</p></div><footer class="model-menu-effort"><div class="effort-caption"><SlidersHorizontal :size="12"/><span :title="selectedLabel">{{model||'尚未选择模型'}}</span></div><ReasoningSelect :model-value="reasoning" :provider="provider?.provider||'openai'" :model="model" :connection="provider" :disabled="disabled" label="" presentation="slider" @update:model-value="value=>emit('update:reasoning',value)" @pending="value=>emit('pending',value)"/></footer></section></Teleport></template>
<style scoped>
.ai-model-picker{display:flex;align-items:center;min-width:0;gap:3px;flex:1}.ai-model-trigger{display:flex;align-items:center;gap:4px;border-radius:5px;min-width:0;height:28px;font-size:.73rem;color:var(--secondary);padding:3px 4px}.ai-model-trigger{flex:1}.ai-model-trigger>span{white-space:nowrap;overflow:hidden;text-overflow:ellipsis;min-width:0}.ai-model-trigger>svg{flex-shrink:0}.ai-model-trigger:hover{background:var(--hover);color:var(--text)}.chat-model-menu{position:fixed;z-index:21000;display:flex;flex-direction:column;min-height:180px;border:1px solid var(--line);border-radius:11px;background:var(--bg);box-shadow:0 12px 45px #0003;overflow:hidden;color:var(--text);font-family:var(--ui-font-family);font-size:.85rem}.model-menu-search{display:flex;align-items:center;gap:9px;padding:12px 14px;flex-shrink:0;border-bottom:1px solid var(--line);color:var(--muted)}.model-menu-search input{font:inherit;font-size:.79rem;min-width:0;flex:1;border:0;background:transparent;color:var(--text);outline:none}.model-menu-search input::placeholder{color:var(--muted)}.model-menu-list{overflow:auto;overscroll-behavior:contain;flex:1;min-height:60px;padding:4px 0 9px}.model-menu-group h4{font-weight:500;letter-spacing:.8px;font-size:.67rem;text-transform:uppercase;color:var(--muted);margin:11px 17px 5px}.model-menu-group button{display:flex;align-items:center;gap:9px;width:100%;text-align:left;min-width:0;padding:8px 16px;font-size:.84rem;line-height:1.6;border-radius:0;color:var(--text)}.model-menu-group button>span:last-child{overflow:hidden;white-space:nowrap;text-overflow:ellipsis;min-width:0}.model-menu-group button[aria-selected=true]{background:var(--accent-bg);color:var(--accent)}.model-menu-group button[data-key-active=true]:not([aria-selected=true]){background:var(--hover)}.model-check-space{width:14px;flex-shrink:0}.model-menu-group button>svg{flex-shrink:0}.model-menu-effort{padding:10px 16px;border-top:1px solid var(--line);background:var(--soft);flex-shrink:0}.effort-caption{display:flex;gap:6px;align-items:center;font-size:.66rem;color:var(--muted);margin-bottom:5px;min-width:0}.effort-caption>span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.no-models{padding:24px 17px;color:var(--muted);font-size:.79rem;line-height:1.8}
</style>
