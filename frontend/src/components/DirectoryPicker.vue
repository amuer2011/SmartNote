<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { Check, ChevronDown, Folder } from 'lucide-vue-next'

const ROOT_VALUE = '.'
const ROOT_LABEL = '工作区根目录（.）'

const props = defineProps<{ modelValue: string; directories: string[] }>()
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

type DirectoryOption = { value: string; label: string; root: boolean }

const open = ref(false)
const filtering = ref(false)
const activeIndex = ref(-1)
const rootRef = ref<HTMLElement | null>(null)
const listRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLInputElement | null>(null)

const uid = useId()
const listId = `directory-picker-list-${uid}`
const optionId = (index: number) => `${listId}-option-${index}`

const query = computed(() => filtering.value ? (props.modelValue ?? '').trim() : '')

const options = computed<DirectoryOption[]>(() => {
  const q = query.value.toLowerCase()
  const result: DirectoryOption[] = []
  const rootMatches = !q || ROOT_VALUE.includes(q) || ROOT_LABEL.toLowerCase().includes(q)
  if (rootMatches) result.push({ value: ROOT_VALUE, label: ROOT_LABEL, root: true })
  const seen = new Set<string>([ROOT_VALUE])
  for (const entry of props.directories || []) {
    const value = String(entry ?? '').trim().replace(/\/+$/, '')
    if (!value || value === ROOT_VALUE || seen.has(value)) continue
    if (q && !value.toLowerCase().includes(q)) continue
    seen.add(value)
    result.push({ value, label: value, root: false })
  }
  return result
})

const activeOptionId = computed(() => (open.value && activeIndex.value >= 0 ? optionId(activeIndex.value) : undefined))

watch(activeIndex, async () => {
  await nextTick()
  listRef.value?.querySelector<HTMLElement>('.active')?.scrollIntoView({ block: 'nearest' })
})

watch(options, () => {
  if (activeIndex.value >= options.value.length) activeIndex.value = -1
})

function openList(showAll = true) {
  filtering.value = !showAll
  if (open.value) return
  open.value = true
  activeIndex.value = -1
}

function closeList() {
  open.value = false
  activeIndex.value = -1
}

function toggleList() {
  if (open.value) closeList()
  else {
    openList()
    inputRef.value?.focus()
  }
}

function select(value: string) {
  if (value !== props.modelValue) emit('update:modelValue', value)
  closeList()
  inputRef.value?.focus()
}

function onInput(event: Event) {
  emit('update:modelValue', (event.target as HTMLInputElement).value)
  openList(false)
  activeIndex.value = -1
}

function moveActive(step: number) {
  const total = options.value.length
  if (!total) return
  if (!open.value) {
    open.value = true
    activeIndex.value = step > 0 ? 0 : total - 1
    return
  }
  if (activeIndex.value < 0) activeIndex.value = step > 0 ? 0 : total - 1
  else activeIndex.value = Math.min(total - 1, Math.max(0, activeIndex.value + step))
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'ArrowDown') {
    event.preventDefault()
    moveActive(1)
    return
  }
  if (event.key === 'ArrowUp') {
    event.preventDefault()
    moveActive(-1)
    return
  }
  if (event.key === 'Enter') {
    if (!open.value) return
    event.preventDefault()
    const option = options.value[activeIndex.value]
    if (option) select(option.value)
    else closeList()
    return
  }
  if (event.key === 'Escape') {
    if (!open.value) return
    event.preventDefault()
    event.stopPropagation()
    closeList()
    return
  }
  if (event.key === 'Tab') closeList()
}

function onDocumentPointerDown(event: PointerEvent) {
  const target = event.target as Node | null
  if (open.value && rootRef.value && target && !rootRef.value.contains(target)) closeList()
}

onMounted(() => document.addEventListener('pointerdown', onDocumentPointerDown, true))
onBeforeUnmount(() => document.removeEventListener('pointerdown', onDocumentPointerDown, true))
</script>

<template>
 <div ref="rootRef" class="directory-picker">
  <div class="directory-picker-field" :class="{ open }">
   <Folder :size="15" class="directory-picker-icon" aria-hidden="true" />
   <input
    ref="inputRef"
    class="directory-picker-input"
    type="text"
    :value="modelValue"
    placeholder="选择或输入保存目录，例如 notes/2026"
    aria-label="保存目录"
    role="combobox"
    aria-autocomplete="list"
    aria-haspopup="listbox"
    :aria-expanded="open"
    :aria-controls="listId"
    :aria-activedescendant="activeOptionId"
    autocomplete="off"
    spellcheck="false"
    @input="onInput"
    @keydown="onKeydown"
    @click="openList()"
   />
   <button
    type="button"
    class="directory-picker-toggle"
    aria-label="展开保存目录"
    :aria-expanded="open"
    :aria-controls="listId"
    @click="toggleList"
   >
    <ChevronDown :size="15" :class="{ rotated: open }" aria-hidden="true" />
   </button>
  </div>
  <div v-if="open" class="directory-picker-panel">
   <ul :id="listId" ref="listRef" class="directory-picker-list" role="listbox" aria-label="已有目录">
    <li
     v-for="(option, index) in options"
     :id="optionId(index)"
     :key="option.value"
     class="directory-picker-option"
     role="option"
     :class="{ active: index === activeIndex, selected: option.value === modelValue }"
     :aria-selected="option.value === modelValue"
     @mousedown.prevent
     @click="select(option.value)"
    >
     <Folder :size="14" aria-hidden="true" />
     <span class="directory-picker-path">{{ option.label }}</span>
     <Check v-if="option.value === modelValue" :size="13" class="directory-picker-check" aria-hidden="true" />
    </li>
   </ul>
   <p v-if="!options.length" class="directory-picker-empty">无匹配目录，可直接输入新目录</p>
  </div>
 </div>
</template>

<style scoped>
.directory-picker{position:relative;width:100%}
.directory-picker-field{display:flex;align-items:center;gap:0.5rem;padding:0 0.5rem;background:var(--bg);border:1px solid var(--line);border-radius:6px}
.directory-picker-field:focus-within,.directory-picker-field.open{border-color:var(--accent)}
.directory-picker-icon{color:var(--muted);flex-shrink:0}
.directory-picker-input{flex:1;min-width:0;border:0;background:transparent;color:var(--text);font-size:0.85714rem;padding:0.625rem 0;outline:0}
.directory-picker-input::placeholder{color:var(--muted)}
.directory-picker-toggle{display:inline-flex;align-items:center;justify-content:center;padding:0.25rem;color:var(--muted);border-radius:4px;flex-shrink:0}
.directory-picker-toggle:hover{color:var(--text);background:var(--hover,transparent)}
.directory-picker-toggle svg{transition:transform .15s}
.directory-picker-toggle svg.rotated{transform:rotate(180deg)}
.directory-picker-panel{position:absolute;top:calc(100% + 0.28571rem);left:0;right:0;z-index:40;background:var(--bg);border:1px solid var(--line);border-radius:8px;box-shadow:var(--shadow);overflow:hidden}
.directory-picker-list{margin:0;padding:0.28571rem;list-style:none;max-height:14rem;overflow:auto}
.directory-picker-option{display:flex;align-items:center;gap:0.5rem;padding:0.5rem 0.57143rem;border-radius:5px;font-size:0.85714rem;color:var(--text);cursor:pointer}
.directory-picker-option svg{color:var(--muted);flex-shrink:0}
.directory-picker-option.active{background:var(--accent-bg)}
.directory-picker-option.selected{color:var(--accent)}
.directory-picker-option.selected svg{color:var(--accent)}
.directory-picker-path{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.directory-picker-check{margin-left:auto}
.directory-picker-empty{margin:0;padding:0.57143rem 0.64286rem;font-size:0.78571rem;color:var(--muted)}
</style>
