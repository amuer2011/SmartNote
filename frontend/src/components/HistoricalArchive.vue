<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { NButton } from 'naive-ui'
import ResizableModal from './ResizableModal.vue'
import { api, type ArchiveResult, type ArchiveSuggestion, type Note } from '../services/api'
import { useWorkspace } from '../stores/workspace'

const props = defineProps<{ show: boolean; workspaceId: string }>()
const store = useWorkspace()
const emit = defineEmits<{
  'update:show': [value: boolean]
  applied: [results: ArchiveResult[]]
  undone: [note: Note]
}>()

interface Row {
  note: Note
  suggestion?: ArchiveSuggestion
  targetDirectory: string
  selected: boolean
  error: string
  applyError: string
  applied?: ArchiveResult
  working: boolean
}

const rows = ref<Row[]>([])
const revision = ref('')
const revisionReady = ref(false)
const error = ref('')
const analyzing = ref(false)
const applying = ref(false)
const done = ref(0)
const total = ref(0)
const interrupted = ref(false)
const onlyEligible = ref(true)
const query = ref('')
const page = ref(0)
const pageSize = 80
const queue = ref<Note[]>([])
let generation = 0
let workspace = ''

const folderOf = (path: string) => path.includes('/') ? path.slice(0, path.lastIndexOf('/')) : '.'
const normalized = (path: string) => {
  const parts: string[] = []
  for (const part of path.trim().replace(/\\/g, '/').split('/')) {
    if (!part || part === '.') continue
    if (part === '..' && parts.length && parts[parts.length - 1] !== '..') parts.pop()
    else parts.push(part)
  }
  return parts.join('/') || '.'
}
const recommended = (row: Row) => !!row.suggestion && !row.error && row.suggestion.source === 'semantic' && normalized(row.suggestion.directory) !== folderOf(row.note.path) && !row.applied
const canArchive = (row: Row) => recommended(row) && normalized(row.targetDirectory) !== folderOf(row.note.path)
const eligible = computed(() => rows.value.filter(canArchive))
const selected = computed(() => rows.value.filter(row => row.selected && canArchive(row)))
const matchingRows = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase()
  return rows.value.filter(row => {
    if (onlyEligible.value && !recommended(row) && !row.applied && !row.error) return false
    return !needle || `${row.note.title}\n${row.note.path}\n${row.targetDirectory}`.toLocaleLowerCase().includes(needle)
  })
})
const pageCount = computed(() => Math.max(1, Math.ceil(matchingRows.value.length / pageSize)))
const visibleRows = computed(() => matchingRows.value.slice(page.value * pageSize, (page.value + 1) * pageSize))
const unchanged = computed(() => rows.value.filter(row => row.suggestion?.source === 'semantic' && folderOf(row.note.path) === normalized(row.suggestion.directory)).length)
const lowConfidence = computed(() => rows.value.filter(row => row.suggestion?.source !== 'semantic' && !row.error).length)
const failures = computed(() => rows.value.filter(row => !!row.error || !!row.applyError).length)
const completed = computed(() => rows.value.filter(row => !!row.applied).length)
watch([onlyEligible, query], () => { page.value = 0 })

async function analyze(reset: boolean, preserveApplied = false) {
  if (analyzing.value || applying.value || !props.workspaceId) return
  const seq = ++generation
  error.value = ''
  analyzing.value = true
  try {
    if (reset) {
      const appliedRows = preserveApplied ? rows.value.filter(row => !!row.applied) : []
      const appliedIDs = new Set(appliedRows.map(row => row.note.id))
      rows.value = appliedRows
      revision.value = ''
      revisionReady.value = false
      done.value = 0
      total.value = 0
      interrupted.value = false
      page.value = 0
      await store.save()
      if (seq !== generation) return
      if (store.dirty) throw new Error('请先处理未保存的笔记修改，再分析历史笔记')
      const snapshot = await api.Snapshot()
      if (seq !== generation) return
      if (props.workspaceId !== snapshot.settings.activeId) throw new Error('工作区已切换，请重新打开历史整理')
      queue.value = snapshot.notes.filter(note => !note.taskArchived && !note.path.toLocaleLowerCase().startsWith('已完成待办/')).filter(note => !appliedIDs.has(note.id))
    }
    const notes = queue.value
    total.value = notes.length
    for (let start = done.value; start < notes.length; start += 4) {
      if (seq !== generation) return
      const group = notes.slice(start, start + 4)
      const result = await api.SuggestSemanticArchiveBatch(props.workspaceId, group.map(note => ({ noteId: note.id, hash: note.hash, path: note.path })))
      if (seq !== generation) return
      if (revision.value && revision.value !== result.revision) throw new Error('分类配置已变化，请重新开始分析')
      revision.value = result.revision
      revisionReady.value = true
      const byID = new Map(group.map(note => [note.id, note]))
      for (const item of result.items) {
        const note = byID.get(item.noteId)
        if (!note) continue
        rows.value.push({ note, suggestion: item.suggestion, targetDirectory: item.suggestion?.directory || folderOf(note.path), selected: false, error: item.error || '', applyError: '', working: false })
      }
      done.value = Math.min(start + group.length, notes.length)
    }
    interrupted.value = false
  } catch (cause) {
    if (seq === generation) error.value = String(cause)
  } finally {
    if (seq === generation) analyzing.value = false
  }
}

function stopAnalysis() {
  if (!analyzing.value) return
  generation++
  analyzing.value = false
  interrupted.value = done.value < total.value
}

function selectAll(value: boolean) {
  for (const row of eligible.value) row.selected = value
}

async function applySelected() {
  if (applying.value || analyzing.value || !revisionReady.value || selected.value.length === 0) return
  applying.value = true
  error.value = ''
  const applyWorkspace = props.workspaceId
  const applyRevision = revision.value
  try {
    await store.save()
    if (store.workspace?.id !== applyWorkspace || props.workspaceId !== applyWorkspace) throw new Error('工作区已切换，请重新打开历史整理')
    if (store.dirty) { error.value = '请先处理未保存的笔记修改，再确认整理';return }
    const successes: ArchiveResult[] = []
    const targets = [...selected.value]
    for (const row of targets) {
      if (store.workspace?.id !== applyWorkspace || props.workspaceId !== applyWorkspace) {
        error.value = '工作区已切换，已停止后续整理'
        break
      }
      row.working = true
      try {
        const result = await api.ApplyReviewedSemanticArchive(applyRevision, {
          workspaceId: applyWorkspace,
          noteId: row.note.id,
          hash: row.suggestion!.hash,
          sourcePath: row.note.path,
          directory: row.targetDirectory,
          tags: row.suggestion!.tags,
        })
        row.applied = result
        row.selected = false
        row.error = ''
        row.applyError = ''
        successes.push(result)
      } catch (cause) {
        row.applyError = String(cause)
        if (row.applyError.includes('分类配置已变化') || row.applyError.includes('工作区已切换')) {
          error.value = row.applyError + '，已停止后续整理'
          break
        }
      } finally {
        row.working = false
      }
    }
    if (successes.length && store.workspace?.id === applyWorkspace && props.workspaceId === applyWorkspace) emit('applied', successes)
  } catch (cause) {
    error.value = String(cause)
  } finally {
    applying.value = false
  }
}

async function undo(row: Row) {
  if (!row.applied || row.working || applying.value) return
  row.working = true
  row.applyError = ''
  try {
    const restored = await api.UndoArchive(props.workspaceId, row.applied.operationId, row.applied.note.hash)
    row.note = restored.note
    if (row.suggestion) row.suggestion.hash = restored.note.hash
    if (row.suggestion) {
      row.suggestion.directory = folderOf(restored.note.path)
      row.suggestion.source = 'current'
    }
    row.targetDirectory = row.suggestion?.directory || folderOf(row.note.path)
    row.applied = undefined
    row.selected = false
    emit('undone', restored.note)
  } catch (cause) {
    row.applyError = String(cause)
  } finally {
    row.working = false
  }
}

async function retryAnalysis(row: Row) {
  if (row.working || applying.value || analyzing.value) return
  row.working = true
  row.error = ''
  try {
    const snapshot = await api.Snapshot()
    if (snapshot.settings.activeId !== props.workspaceId) throw new Error('工作区已切换，请重新打开历史整理')
    const latest = snapshot.notes.find(note => note.id === row.note.id)
    if (!latest) throw new Error('笔记已删除')
    const config = await api.GetSemanticConfig(props.workspaceId)
    if (revisionReady.value && config.revision !== revision.value) throw new Error('分类配置已变化，请重新分析整个列表')
    const customTarget = !!row.suggestion && normalized(row.targetDirectory) !== normalized(row.suggestion.directory)
    const suggestion = await api.SuggestSemanticArchive(props.workspaceId, latest.id, latest.hash)
    row.note = latest
    row.suggestion = suggestion
    row.targetDirectory = customTarget ? row.targetDirectory : suggestion.directory
    row.error = ''
    row.applyError = ''
    row.selected = false
    revision.value = config.revision
    revisionReady.value = true
  } catch (cause) {
    row.error = String(cause)
  } finally {
    row.working = false
  }
}

watch(() => props.workspaceId, value => {
  if (workspace === value) return
  workspace = value
  generation++
  rows.value = []
  revision.value = ''
  revisionReady.value = false
  done.value = 0
  total.value = 0
  error.value = ''
  interrupted.value = false
  if (!applying.value) analyzing.value = false
})

watch(() => props.show, value => {
  if (value) {
    if (rows.value.length === 0 && !analyzing.value) void analyze(true)
  } else if (analyzing.value) {
    stopAnalysis()
  }
})
</script>

<template>
  <ResizableModal :show="show" preset="card" title="整理已有笔记" style="width:min(920px, 94vw);max-height:88vh" :mask-closable="!applying" :close-on-esc="!applying" @update:show="emit('update:show',$event)" @minimize="emit('update:show',false)">
    <p class="history-help">只分析并展示建议，不会自动移动。勾选需要整理的笔记后再确认；每篇移动都检查文件版本、更新笔记链接并保留恢复记录。已完成待办归档笔记会自动跳过。</p>
    <div class="history-progress" role="status">
      <span>已分析 {{ done }} / {{ total }} 篇</span>
      <span>可整理 {{ eligible.length }} 篇</span>
      <span>位置已匹配 {{ unchanged }} 篇</span>
      <span>低于相似度门槛 {{ lowConfidence }} 篇</span>
      <span v-if="failures">异常 {{ failures }} 篇</span>
    </div>
    <div class="history-actions">
      <NButton :disabled="analyzing || applying || !eligible.length" @click="selectAll(true)">全选可整理项</NButton>
      <NButton :disabled="analyzing || applying || !selected.length" @click="selectAll(false)">取消全选</NButton>
      <NButton :disabled="analyzing || applying" @click="onlyEligible=!onlyEligible">{{ onlyEligible ? '显示全部笔记' : '仅显示可整理项' }}</NButton>
      <NButton v-if="analyzing" @click="stopAnalysis">停止分析</NButton>
      <NButton v-else :disabled="applying" @click="interrupted || (done<total && rows.length>0) ? analyze(false) : rows.length ? analyze(true,true) : analyze(true)">{{ interrupted || (done<total && rows.length>0) ? '继续分析' : rows.length ? '重新分析未整理项' : '开始分析' }}</NButton>
    </div>
    <input v-if="rows.length" v-model="query" class="history-search" type="search" placeholder="按笔记标题、路径或目标目录筛选" aria-label="筛选历史笔记" />
    <p v-if="analyzing" class="history-status">已完成 {{ done }} / {{ total }} 篇，正在分析下一批。可随时停止，已完成的分析结果会保留。</p>
    <p v-else-if="interrupted" class="history-status">分析已暂停；已分析结果可以查看，继续后会从剩余笔记开始。</p>
    <p v-if="error" class="history-error" role="alert">{{ error }}</p>
    <div v-if="rows.length" class="history-list">
      <article v-for="row in visibleRows" :key="row.note.id" class="history-row" :class="{ ready: canArchive(row), applied: !!row.applied }">
        <label class="history-select"><input v-model="row.selected" type="checkbox" :disabled="!canArchive(row) || analyzing || applying" :aria-label="'选择整理 '+row.note.title" /></label>
        <div class="history-main">
          <div class="history-heading"><strong>{{ row.note.title }}</strong><span v-if="row.suggestion?.source==='semantic'" class="history-score">余弦相似度 {{ row.suggestion.score.toFixed(4) }}</span><span v-else-if="row.suggestion" class="history-muted">未达到相似度门槛</span><span v-if="row.applied" class="history-success">已整理</span></div>
          <div class="history-path"><span>{{ row.note.path }}</span><span v-if="recommended(row) || row.applied" class="history-arrow">→</span><label v-if="recommended(row)" class="history-destination">归档到 <input v-model="row.targetDirectory" :disabled="applying" aria-label="批量归档目标目录" @input="row.selected=false" /></label><span v-else-if="row.applied" class="history-destination-text">{{ row.applied.note.path }}</span></div>
          <details v-if="row.suggestion" class="history-reasons"><summary>查看分类依据</summary><p v-for="reason in row.suggestion.reasons" :key="reason">{{ reason }}</p></details>
          <p v-if="row.error" class="history-row-error" role="alert">分析失败：{{ row.error }} <button class="history-retry" :disabled="row.working || applying" @click="retryAnalysis(row)">重试分析</button></p>
          <p v-if="row.applyError" class="history-row-error" role="alert">整理失败：{{ row.applyError }}。<button class="history-retry" :disabled="row.working || applying" @click="retryAnalysis(row)">刷新建议</button>后检查再确认。</p>
        </div>
        <NButton v-if="row.applied" size="small" :loading="row.working" :disabled="applying" @click="undo(row)">撤销</NButton>
        <span v-else-if="row.working" class="history-status">处理中…</span>
      </article>
      <p v-if="onlyEligible && !visibleRows.length" class="history-empty">当前没有可整理项。</p>
    </div>
    <p v-else-if="!analyzing" class="history-empty">{{ total === 0 ? '工作区中没有可分析的历史笔记。' : '还没有分析结果。' }}</p>
    <div v-if="matchingRows.length>pageSize" class="history-pages"><span>显示 {{ page*pageSize+1 }}–{{ Math.min((page+1)*pageSize,matchingRows.length) }} / {{ matchingRows.length }} 篇</span><div><NButton size="small" :disabled="page===0" @click="page--">上一页</NButton><span>{{ page+1 }} / {{ pageCount }}</span><NButton size="small" :disabled="page+1>=pageCount" @click="page++">下一页</NButton></div></div>
    <template #footer>
      <div class="history-footer"><span v-if="selected.length">已选择 {{ selected.length }} 篇</span><span v-if="applying">正在整理，成功项不会被重复处理。</span><span v-else-if="completed">本轮已整理 {{ completed }} 篇；可逐篇撤销</span><span v-else-if="!selected.length">确认后才会移动文件</span><div><NButton :disabled="applying" @click="emit('update:show',false)">关闭</NButton><NButton type="primary" :disabled="analyzing || !selected.length || applying" :loading="applying" @click="applySelected">确认整理所选笔记</NButton></div></div>
    </template>
  </ResizableModal>
</template>

<style scoped>
.history-help,.history-status,.history-muted,.history-empty{color:var(--muted);font-size:.88rem;line-height:1.75;margin:10px 0}
.history-progress{display:flex;gap:8px 18px;flex-wrap:wrap;padding:11px 13px;background:var(--soft);border:1px solid var(--line);border-radius:8px;font-size:.84rem;color:var(--secondary)}
.history-actions{display:flex;gap:8px;flex-wrap:wrap;margin:12px 0}.history-search{box-sizing:border-box;width:100%;margin:0 0 10px;padding:8px 10px;border:1px solid var(--line);border-radius:6px;background:var(--soft);color:var(--text);font:inherit}
.history-list{max-height:54vh;overflow:auto;display:flex;flex-direction:column;gap:8px;padding:2px}
.history-row{display:flex;align-items:flex-start;gap:11px;border:1px solid var(--line);border-radius:8px;padding:11px;background:var(--bg);min-width:0}
.history-row.ready{border-color:color-mix(in srgb,var(--accent) 35%,var(--line))}.history-row.applied{background:var(--soft)}
.history-select{padding-top:2px}.history-main{flex:1;min-width:0}.history-heading,.history-path{display:flex;gap:9px;align-items:center;flex-wrap:wrap}.history-heading strong{overflow:hidden;text-overflow:ellipsis;max-width:65%;white-space:nowrap}.history-score{font-size:.77rem;color:var(--accent)}.history-success{font-size:.77rem;color:#2d9a69}
.history-path{margin-top:6px;font-size:.83rem;overflow-wrap:anywhere;color:var(--secondary)}.history-arrow{color:var(--muted)}.history-destination{display:flex;gap:6px;align-items:center;color:var(--muted)}.history-destination input{width:min(270px,48vw);min-width:120px;padding:5px 7px;border:1px solid var(--line);border-radius:5px;background:var(--soft);color:var(--text);font:inherit}
.history-reasons{margin-top:6px;color:var(--muted);font-size:.78rem}.history-reasons summary{cursor:pointer}.history-reasons p{margin:4px 0;line-height:1.55}.history-row-error,.history-error{color:#cf5555;overflow-wrap:anywhere;font-size:.82rem}.history-footer{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap;width:100%;color:var(--muted);font-size:.84rem}.history-footer>div{display:flex;gap:8px}
.history-pages{display:flex;align-items:center;justify-content:space-between;gap:12px;margin:10px 2px;color:var(--muted);font-size:.82rem}.history-pages>div{display:flex;gap:8px;align-items:center}
</style>
