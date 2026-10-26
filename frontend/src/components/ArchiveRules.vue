<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { api, type ArchiveRule, type ArchiveRulesConfig } from '../services/api'
import DirectoryPicker from './DirectoryPicker.vue'

const props=defineProps<{workspaceId:string;directories:string[]}>()
type RuleDraft=Omit<ArchiveRule,'keywords'|'tags'>&{keywords:string;tags:string}
type Draft={revision:string;rules:RuleDraft[];baseline:string}
const rules=ref<RuleDraft[]>([]),revision=ref(''),baseline=ref('[]'),error=ref(''),status=ref(''),loading=ref(false),saving=ref(false)
const cache=new Map<string,Draft>()
let sequence=0,loadedWorkspace=''
const dirty=computed(()=>JSON.stringify(rules.value)!==baseline.value)
const split=(value:string)=>value.split(/[,，]/).map(word=>word.trim()).filter(Boolean)
function stash() {if(loadedWorkspace) cache.set(loadedWorkspace,{revision:revision.value,rules:JSON.parse(JSON.stringify(rules.value)),baseline:baseline.value})}
function accept(value:ArchiveRulesConfig) {revision.value=value.revision;rules.value=value.rules.map(rule=>({...rule,keywords:rule.keywords.join('，'),tags:rule.tags.join('，')}));baseline.value=JSON.stringify(rules.value)}
async function load(force=false) {
 const id=props.workspaceId,request=++sequence
 if(loadedWorkspace!==id) stash()
 loadedWorkspace=id;error.value='';status.value='';loading.value=false
 if(!id) {rules.value=[];revision.value='';baseline.value='[]';return}
 const saved=cache.get(id)
 if(saved && !force) {rules.value=JSON.parse(JSON.stringify(saved.rules));revision.value=saved.revision;baseline.value=saved.baseline;return}
 rules.value=[];loading.value=true
 try {const value=await api.GetArchiveRules(id);if(request===sequence && props.workspaceId===id) {accept(value);cache.delete(id)}}
 catch(e) {if(request===sequence) error.value=String(e)} finally {if(request===sequence) loading.value=false}
}
watch(()=>props.workspaceId,()=>void load(),{immediate:true})
function add() {rules.value.push({id:crypto.randomUUID(),name:'',enabled:true,field:'title',match:'any',keywords:'',directory:'Inbox',tags:''});status.value=''}
function move(index:number,delta:number) {const target=index+delta;if(target<0 || target>=rules.value.length) return;const [rule]=rules.value.splice(index,1);rules.value.splice(target,0,rule!)}
async function save() {
 if(saving.value || loading.value || !props.workspaceId) return
 const id=props.workspaceId,request=sequence
 error.value='';status.value=''
 const snapshot:ArchiveRulesConfig={revision:revision.value,rules:rules.value.map(rule=>({...rule,name:rule.name.trim(),keywords:split(rule.keywords),tags:split(rule.tags)}))}
 if(snapshot.rules.some(rule=>!rule.name || !rule.keywords.length)) {error.value='请填写每条规则的名称和至少一个关键词';return}
 saving.value=true
 try {
  const result=await api.SaveArchiveRules(id,snapshot)
  if(request===sequence && props.workspaceId===id) {accept(result);status.value='归档规则已保存';cache.delete(id)}
  else cache.delete(id)
 } catch(e) {if(request===sequence) error.value=String(e)} finally {saving.value=false}
}
</script>
<template>
 <section class="archive-rules">
  <p class="rule-help">规则从上到下匹配，优先采用第一条命中的规则。这里保留独立的关键词规则模式；语义分类使用上方分类说明，不依赖本规则。</p>
  <p v-if="!workspaceId">请先选择工作区。</p>
  <template v-else>
   <p class="rule-help">规则仅作用于当前工作区，保存规则不会立即移动笔记。</p>
   <p v-if="loading" role="status">正在加载规则…</p><p v-if="error" role="alert" class="rule-error">{{error}}</p><p v-if="status" role="status">{{status}}</p>
   <p v-if="dirty" class="rule-unsaved">有未保存的规则修改</p>
   <fieldset :disabled="saving || loading" class="rule-fields">
    <article v-for="(rule,index) in rules" :key="rule.id" class="archive-rule">
     <header><label><input v-model="rule.enabled" type="checkbox" :aria-label="'启用规则 '+(rule.name || '新规则')"/>启用</label><span>优先级 {{index+1}}</span><div><button class="text-button" :disabled="index===0" :aria-label="'上移规则 '+(rule.name || '新规则')" @click="move(index,-1)">上移</button><button class="text-button" :disabled="index===rules.length-1" :aria-label="'下移规则 '+(rule.name || '新规则')" @click="move(index,1)">下移</button><button class="text-button" :aria-label="'删除规则 '+(rule.name || '新规则')" @click="rules.splice(index,1)">删除</button></div></header>
     <label class="rule-label">规则名称<input v-model="rule.name" aria-label="规则名称" maxlength="120" placeholder="例如：数据库排查"/></label>
     <div class="rule-columns"><label class="rule-label">匹配字段<select v-model="rule.field" aria-label="匹配字段"><option value="title">标题</option><option value="content">正文</option><option value="project">项目</option><option value="tag">已有标签</option></select></label><label class="rule-label">匹配方式<select v-model="rule.match" aria-label="匹配方式"><option value="any">包含任意关键词</option><option value="all">包含全部关键词</option></select></label></div>
     <label class="rule-label">关键词（逗号分隔）<input v-model="rule.keywords" aria-label="关键词" placeholder="数据库，SQL，备份"/></label>
     <span class="rule-label">归档目录</span><DirectoryPicker v-model="rule.directory" :directories="directories"/>
     <label class="rule-label">推荐标签（逗号分隔）<input v-model="rule.tags" aria-label="推荐标签" placeholder="数据库，排查"/></label>
    </article>
    <p v-if="!rules.length && !loading" class="rule-help">还没有归档规则，添加一条开始整理。</p>
    <button class="secondary-button" @click="add">添加规则</button>
   </fieldset>
   <footer><button class="primary-button" :disabled="loading || saving || !dirty" @click="save">{{saving?'正在保存…':'保存归档规则'}}</button><button class="text-button" :disabled="loading || saving" @click="load(true)">重新加载（放弃未保存修改）</button></footer>
  </template>
 </section>
</template>
<style scoped>
.rule-help{font-size:.857rem;color:var(--muted);line-height:1.9;margin:12px 0}.rule-error{color:#cf5555;overflow-wrap:anywhere}.rule-unsaved{color:var(--accent);font-size:.857rem}.rule-fields{border:0;padding:0;margin:0;min-width:0}.archive-rule{background:var(--soft);border:1px solid var(--line);border-radius:8px;padding:16px;margin:18px 0}.archive-rule header{display:flex;align-items:center;gap:14px;flex-wrap:wrap;font-size:.857rem}.archive-rule header>label{display:flex;align-items:center;gap:6px}.archive-rule header>div{margin-left:auto;display:flex;gap:12px}.rule-label{display:flex;flex-direction:column;gap:8px;margin:14px 0 8px;font-size:.857rem;color:var(--secondary)}.rule-label input,.rule-label select{width:100%;min-width:0;box-sizing:border-box;color:var(--text);background:var(--bg);border:1px solid var(--line);border-radius:5px;padding:9px 10px;font:inherit}.rule-columns{display:flex;gap:14px;flex-wrap:wrap}.rule-columns>label{flex:1;min-width:130px}.archive-rules>footer{display:flex;gap:16px;flex-wrap:wrap;margin-top:22px;align-items:center}
</style>
