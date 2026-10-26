<script setup lang="ts">
import {computed,onBeforeUnmount,ref,useId,watch} from 'vue'
import {NSelect} from 'naive-ui'
import {selectTheme} from '../services/select-theme'
import {api,type LLMProvider,type LLMReasoningCapability} from '../services/api'
import {effortLabels,normalizeReasoning,reasoningCapability} from '../services/llm-reasoning'
import ReasoningSlider from './ReasoningSlider.vue'
defineOptions({inheritAttrs:false})
const props=withDefaults(defineProps<{modelValue:string;provider:'openai'|'ollama';model:string;connection?:LLMProvider;secret?:string;clearSecret?:boolean;disabled?:boolean;label?:string;help?:boolean;presentation?:'select'|'slider'}>(),{label:'思考强度',secret:'',clearSecret:false,presentation:'select'})
const emit=defineEmits<{'update:modelValue':[value:string];pending:[value:boolean]}>()
const id=useId()
const discovered=ref<LLMReasoningCapability>(),loading=ref(false),fallback=ref('')
watch(loading,value=>emit('pending',value),{flush:'sync'})
const capability=computed(()=>discovered.value||reasoningCapability(props.provider,props.model))
const message=computed(()=>loading.value?'正在读取此模型的思考选项…':fallback.value||capability.value.message)
const selectOptions=computed(()=>{
 const options=capability.value.efforts.map(value=>({value,label:effortLabels[value]||`${value.replace(/^level:/,'')}（模型档位）`}))
 return [{value:'',label:'模型默认'},...(capability.value.known?options:[{type:'group' as const,key:'compatibility',label:'兼容档位 · 需供应商支持',children:options}])]
})
let sequence=0,timer:ReturnType<typeof setTimeout>|undefined
function normalize(){const value=normalizeReasoning(props.modelValue,capability.value);if(value!==props.modelValue)emit('update:modelValue',value)}
watch(()=>[props.provider,props.model,props.connection?.id,props.connection?.endpoint,props.connection?.revision],()=>{
 const token=++sequence;clearTimeout(timer);discovered.value=undefined;fallback.value='';loading.value=false
 if(props.provider!=='ollama'||!props.model||!props.connection){normalize();return}
 const connection={...props.connection,model:props.model}
 // Keep saved values intact while reading metadata; parents suspend sends/saves.
 loading.value=true
 timer=setTimeout(async()=>{
  try{
   const result=await api.GetLLMReasoningCapability(connection,props.secret,props.clearSecret)
   if(token!==sequence)return
   if(result.known)discovered.value=result
   else fallback.value='此服务未提供思考元数据。'+capability.value.message
  }catch{if(token!==sequence)return;fallback.value='暂时无法读取思考元数据。'+capability.value.message}
  if(token===sequence){loading.value=false;normalize()}
 },250)
},{immediate:true})
watch(()=>props.modelValue,()=>{if(!loading.value)normalize()})
onBeforeUnmount(()=>{sequence++;clearTimeout(timer);emit('pending',false)})
</script>
<template><div class="reasoning-select" :title="message"><label v-if="label&&presentation==='select'" class="field-label" :for="id">{{label}}</label><ReasoningSlider v-if="presentation==='slider'" :model-value="modelValue" :show-help="!help" :efforts="capability.efforts" :known="capability.known" :message="message" :loading="loading" :disabled="disabled||!model" @update:model-value="value=>emit('update:modelValue',value)"/><NSelect v-else :id="id" v-bind="$attrs" :title="message" aria-label="思考强度" :input-props="{'aria-label':'思考强度'}" size="small" :value="modelValue" :options="selectOptions" :disabled="disabled||loading||!model||capability.known&&!capability.efforts.length" :loading="loading" :consistent-menu-width="false" :menu-props="{'aria-label':'思考强度选项'}" :theme-overrides="selectTheme" @update:value="value=>emit('update:modelValue',value)"/><p v-if="help" class="field-help" role="status">{{message}}</p></div></template>
<style scoped>.reasoning-select{min-width:0}.reasoning-select :deep(.n-base-selection){font-size:.78rem}</style>
