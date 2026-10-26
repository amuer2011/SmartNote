<script setup lang="ts">
import {ref,watch,computed} from 'vue'
import {api,type SemanticConfig,type SemanticCategory,type EmbeddingStatus,type Note,type ArchiveResult} from '../services/api'
import {saveSemanticThresholds} from '../services/semantic-settings'
import DirectoryPicker from './DirectoryPicker.vue'
import HistoricalArchive from './HistoricalArchive.vue'
import DirectoryLearning from './DirectoryLearning.vue'
import {useWorkspace} from '../stores/workspace'
const props=defineProps<{workspaceId:string;directories:string[]}>()
const emit=defineEmits<{saved:[workspaceId:string];'batch-applied':[results:ArchiveResult[]];'batch-undone':[note:Note]}>()
const store=useWorkspace()
const modelStatus=ref<EmbeddingStatus>()
const config=ref<SemanticConfig>(),error=ref(''),status=ref(''),busy=ref(false),baseline=ref('')
let sequence=0
const dirty=computed(()=>!!config.value&&JSON.stringify(config.value)!==baseline.value)
const drafts=new Map<string,SemanticConfig>();let loaded=''
const historyOpen=ref(false)
const learningOpen=ref(false),learningID=ref('')
const learningCategory=computed(()=>config.value?.categories.find(c=>c.id===learningID.value))
watch(()=>props.workspaceId,()=>{learningOpen.value=false;learningID.value=''})
async function openLearning(category:SemanticCategory){
 if(dirty.value){error.value='请先保存当前分类修改，再学习目录。';return}
 const id=props.workspaceId,seq=sequence
 await store.save();if(id!==props.workspaceId||seq!==sequence)return
 if(store.dirty){error.value='请先处理未保存的笔记修改。';return}
 learningID.value=category.id;learningOpen.value=true
}
function learned(id:string,value:SemanticConfig){
 if(id===props.workspaceId){config.value=value;baseline.value=JSON.stringify(value);drafts.delete(id);status.value=value.categories.find(c=>c.id===learningID.value)?.examples?.length?'目录学习样本已保存，后续分类自动使用。':'已清除目录学习，恢复仅参考分类说明。'}
 emit('saved',id)
}
async function load(force=false){
 if(config.value&&dirty.value&&loaded) drafts.set(loaded,JSON.parse(JSON.stringify(config.value)))
 const id=props.workspaceId,seq=++sequence;loaded=id;config.value=undefined;error.value='';status.value=''
 if(!id)return
 busy.value=true
 try{const [value,model]=await Promise.all([api.GetSemanticConfig(id),api.EmbeddingStatus()]);if(seq!==sequence)return;modelStatus.value=model;baseline.value=JSON.stringify(value);config.value=force?value:(drafts.get(id)||value);if(force)drafts.delete(id)}catch(e){if(seq===sequence)error.value=String(e)}finally{if(seq===sequence)busy.value=false}
}
watch(()=>props.workspaceId,()=>void load(),{immediate:true})
function add(){config.value?.categories.push({id:crypto.randomUUID(),name:'新分类',description:'',directory:'Inbox',enabled:true})}
async function save(){if(!config.value||busy.value)return;const id=props.workspaceId,seq=sequence;busy.value=true;error.value='';status.value='';try{const result=await api.SaveSemanticConfig(id,JSON.parse(JSON.stringify(config.value)));drafts.delete(id);emit('saved',id);if(seq===sequence){config.value=result;baseline.value=JSON.stringify(result);status.value='语义分类已保存'}}catch(e){if(seq===sequence)error.value=String(e)}finally{if(seq===sequence)busy.value=false}}
async function saveThresholds(){
 if(!config.value || busy.value)return
 const id=props.workspaceId,seq=sequence,{threshold}=config.value
 if(!Number.isFinite(threshold)||threshold<0||threshold>1){error.value='相似度须在 0 到 1 之间';return}
 busy.value=true;error.value='';status.value='正在自动保存分类门槛…'
 try{
  const result=await saveSemanticThresholds(id,threshold)
  if(seq===sequence && config.value){
   const base=JSON.parse(baseline.value) as SemanticConfig
   config.value={...config.value,revision:result.revision,threshold:result.threshold}
   baseline.value=JSON.stringify({...base,revision:result.revision,threshold:result.threshold})
   status.value='分类门槛已自动保存，智能整理将使用最新设置'
  }
  emit('saved',id)
 }catch(e){if(seq===sequence){error.value=String(e);status.value='门槛保存失败，请重试'}}finally{if(seq===sequence)busy.value=false}
}
async function test(){
 if(!config.value)return
 busy.value=true;error.value='';status.value='正在检查内置模型…';const seq=sequence,id=props.workspaceId
 try{
  const model=await api.CheckEmbedding();if(seq!==sequence)return;modelStatus.value=model
  const snapshot=await api.Snapshot();if(seq!==sequence)return
  const note=snapshot.notes[0]
  if(note){const result=await api.SuggestSemanticArchive(id,note.id,note.hash);if(seq===sequence)status.value=result.reasons.join('；')}
  else status.value=model.message+'。创建笔记后即可自动分类。'
 }catch(e){if(seq===sequence){error.value=String(e);status.value='';try{modelStatus.value=await api.EmbeddingStatus()}catch{}}}finally{if(seq===sequence)busy.value=false}
}
async function openHistory(){
 if(dirty.value){error.value='请先保存当前分类修改，再分析历史笔记。';return}
 await store.save()
 if(store.dirty){error.value='请先处理未保存的笔记修改，再分析历史笔记。';return}
 await store.refresh()
 historyOpen.value=true
}
</script>
<template>
<section class="semantic-settings">
<h2>Embedding 语义分类</h2>
<p>分类依据包括分类说明和你确认的目录样本，可填写典型内容、主题和例子，或点击“学习目录”选择已有笔记。每个工作区独立保存；删除分类不会删除文件。</p>
<div class="model-status"><strong>内置离线中文模型</strong><p>{{modelStatus?.message || '模型随程序提供，无需安装或配置'}}</p><p>无需下载模型、填写地址或安装其他软件；笔记内容始终留在本机。</p></div>
<p v-if="error" role="alert" class="error">{{error}}</p><p v-if="status" role="status">{{status}}</p>
<fieldset v-if="config" :disabled="busy">

<div class="columns"><label>最低余弦相似度<input v-model.number="config.threshold" @change="saveThresholds" type="number" min="0" max="1" step="0.01"/></label></div>
<p>相似度门槛修改后自动保存并生效；分类名称、说明和目录修改仍需点击“保存语义分类”。</p>
<p>使用余弦相似度最高的分类；最高得分达到最低余弦相似度才归档，低于门槛则保留原位置。</p>
<label class="check"><input v-model="config.automatic" type="checkbox" aria-label="自动语义归档"/>自动整理所有目录的笔记（默认开启）</label>
<p>默认开启。打开或保存工作区任意目录中的笔记，停止编辑约 4 秒后自动分析并归档，无需确认。标题单独参与分类，即使没有正文也会分析；最高相似度达到设置的门槛才移动，匹配不明确时保留原位置。目标目录与当前目录相同时不移动，可随时关闭自动整理。</p>
<div class="history-archive"><div><strong>整理已有笔记</strong><p>只对现有笔记生成分类建议。你可以先检查目标目录，再选择并确认要移动的笔记；分析不会修改文件。</p></div><button class="secondary-button" :disabled="busy || !workspaceId" @click="openHistory">分析历史笔记</button></div>
<article v-for="(item,index) in config.categories" :key="item.id">
<header><label class="check"><input v-model="item.enabled" type="checkbox"/>启用分类</label><button class="text-button" :aria-label="'删除分类 '+item.name" @click="config.categories.splice(index,1)">删除</button></header>
<label>分类名称<input v-model="item.name" maxlength="120" aria-label="分类名称"/></label>
<label>分类说明<textarea v-model="item.description" rows="3" maxlength="6000" aria-label="分类说明"/></label>
<label>目标目录</label><DirectoryPicker v-model="item.directory" :directories="directories"/>
<div class="learning-entry"><span>{{item.examples?.length?'已保存 '+item.examples.length+' 篇学习样本':'尚未学习目录，仅参考分类说明'}}</span><button class="secondary-button" :aria-label="'学习目录 '+item.name" :disabled="busy" @click="openLearning(item)">{{item.examples?.length?'管理学习样本':'学习目录'}}</button></div>
</article>
<button class="secondary-button" @click="add">新增分类</button>
</fieldset>
<p v-if="dirty">有未保存的分类修改</p>
<div class="actions"><button class="primary-button" :disabled="busy||!config" @click="save">保存语义分类</button><button class="secondary-button" :disabled="busy||dirty||!config" @click="test">测试模型与分类</button><button class="text-button" :disabled="busy" @click="load(true)">重新加载</button></div>
</section>
<HistoricalArchive v-model:show="historyOpen" :workspace-id="workspaceId" @applied="emit('batch-applied',$event)" @undone="emit('batch-undone',$event)"/>
<DirectoryLearning v-if="config&&learningCategory" v-model:show="learningOpen" :workspace-id="workspaceId" :category="learningCategory" :revision="config.revision" @saved="learned"/>
</template>
<style scoped>
.learning-entry{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;margin-top:14px}.learning-entry span{font-size:.8rem;color:var(--muted)}.learning-entry .secondary-button{padding:7px 12px;font-size:.82rem}
.semantic-settings{margin-bottom:28px}.model-status{padding:12px 16px;border:1px solid var(--line);border-radius:7px;background:var(--soft)}.semantic-settings p{color:var(--muted);line-height:1.8;font-size:.88rem}.semantic-settings fieldset{border:0;padding:0;min-width:0}.semantic-settings label{display:flex;flex-direction:column;gap:7px;margin:12px 0}.semantic-settings input:not([type=checkbox]),textarea{width:100%;box-sizing:border-box;background:var(--soft);border:1px solid var(--line);border-radius:5px;padding:8px;color:var(--text);font:inherit}.semantic-settings .check{flex-direction:row;align-items:center}.semantic-settings article{border:1px solid var(--line);padding:14px;border-radius:7px;margin:15px 0}.semantic-settings header,.columns,.actions{display:flex;gap:12px;justify-content:space-between;flex-wrap:wrap}.columns label{flex:1;min-width:140px}.actions{justify-content:flex-start;margin-top:16px}.semantic-settings .error{color:#cf5555;overflow-wrap:anywhere}.history-archive{display:flex;align-items:center;justify-content:space-between;gap:16px;margin:18px 0;padding:14px;border:1px solid var(--line);border-radius:8px;background:var(--soft)}.history-archive p{margin:5px 0 0}.history-archive>div{min-width:0}
</style>
