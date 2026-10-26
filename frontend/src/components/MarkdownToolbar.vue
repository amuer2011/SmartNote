<script setup lang="ts">
import type { Component } from 'vue'
import {commands,binding,formatShortcut} from '../services/shortcuts'
import {
  Undo2, Redo2, Bold, Italic, Strikethrough, Heading, List, ListOrdered,
  Quote, Link, CodeXml, Table, SeparatorHorizontal, Eraser, Paperclip, PanelRight, MessageSquareQuote,
} from 'lucide-vue-next'

const props = withDefaults(defineProps<{ disabled?: boolean; metadataVisible?: boolean;shortcuts?:Record<string,string> }>(), {
  disabled: false,
  metadataVisible: false,
  shortcuts:()=>({}),
})
const emit = defineEmits<{ command: [id: string]; attach: []; toggleMetadata: [] }>()

interface ToolbarItem {
  id: string
  label: string
  icon: Component
  shortcut?: string
}

const groups: ToolbarItem[][] = [
  [
    { id: 'undo', label: '撤销', icon: Undo2, shortcut: 'Ctrl+Z' },
    { id: 'redo', label: '重做', icon: Redo2, shortcut: 'Ctrl+Y' },
  ],
  [
    { id: 'bold', label: '加粗', icon: Bold, shortcut: 'Ctrl+B' },
    { id: 'italic', label: '斜体', icon: Italic, shortcut: 'Ctrl+I' },
    { id: 'strike', label: '删除线', icon: Strikethrough, shortcut: 'Ctrl+Shift+X' },
    { id: 'clearFormatting', label: '清除格式', icon: Eraser, shortcut: 'Ctrl+\\' },
  ],
  [
    { id: 'heading', label: '标题', icon: Heading, shortcut: 'Ctrl+1' },
    { id: 'bulletList', label: '无序列表', icon: List, shortcut: 'Ctrl+Shift+8' },
    { id: 'orderedList', label: '有序列表', icon: ListOrdered, shortcut: 'Ctrl+Shift+7' },
    { id: 'callout', label: '插入提示块', icon: MessageSquareQuote },
    { id: 'quote', label: '引用', icon: Quote, shortcut: 'Ctrl+Shift+Q' },
  ],
  [
    { id: 'link', label: '插入链接', icon: Link, shortcut: 'Ctrl+K' },
    { id: 'codeBlock', label: '代码块', icon: CodeXml, shortcut: 'Ctrl+Shift+`' },
    { id: 'table', label: '插入表格', icon: Table },
    { id: 'horizontalRule', label: '插入分隔线', icon: SeparatorHorizontal },
  ],
]

function titleOf(item: ToolbarItem) {
  const command=commands.find(command=>command.id===item.id)
  const shortcut=command?formatShortcut(binding(command.id,props.shortcuts)):''
  return shortcut?`${item.label}（${shortcut}）`:item.label
}
</script>

<template>
  <div class="markdown-toolbar" role="toolbar" aria-label="Markdown常用工具" aria-orientation="horizontal">
    <template v-for="(group, index) in groups" :key="index">
      <span v-if="index > 0" class="md-toolbar-divider" role="separator" aria-orientation="vertical" />
      <button
        v-for="item in group"
        :key="item.id"
        type="button"
        class="md-toolbar-button"
        :title="titleOf(item)"
        :aria-label="item.label"
        :disabled="props.disabled"
        @mousedown.prevent
        @click="emit('command', item.id)"
      >
        <component :is="item.icon" :size="15" :stroke-width="1.8" aria-hidden="true" />
      </button>
    </template>

    <span class="md-toolbar-divider" role="separator" aria-orientation="vertical" />
    <button
      type="button"
      class="md-toolbar-button"
      title="插入图片或附件"
      aria-label="插入图片或附件"
      :disabled="props.disabled"
      @mousedown.prevent
      @click="emit('attach')"
    >
      <Paperclip :size="15" :stroke-width="1.8" aria-hidden="true" />
    </button>

    <span class="md-toolbar-divider" role="separator" aria-orientation="vertical" />
    <button
      type="button"
      class="md-toolbar-button"
      :class="{ pressed: props.metadataVisible }"
      :title="props.metadataVisible ? '隐藏元数据面板' : '显示元数据面板'"
      :aria-label="props.metadataVisible ? '隐藏元数据面板' : '显示元数据面板'"
      :aria-pressed="props.metadataVisible"
      :disabled="props.disabled"
      @mousedown.prevent
      @click="emit('toggleMetadata')"
    >
      <PanelRight :size="15" :stroke-width="1.8" aria-hidden="true" />
    </button>
  </div>
</template>

<style scoped>
.markdown-toolbar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  align-content: center;
  gap: 2px;
  min-width: 0;
  max-width: 100%;
  padding: 2px 3px;
  color: var(--muted);
  background: transparent;
  border-radius: 6px;
}

.md-toolbar-button {
  display: inline-flex;
  flex: 0 0 auto;
  align-items: center;
  justify-content: center;
  width: 26px;
  height: 26px;
  padding: 0;
  color: var(--muted);
  line-height: 1;
  background: transparent;
  border: 0;
  border-radius: 5px;
  cursor: pointer;
  transition: background-color 0.12s ease, color 0.12s ease;
}

.md-toolbar-button:hover:not(:disabled) {
  color: var(--text);
  background: var(--soft);
}

.md-toolbar-button:active:not(:disabled) {
  color: var(--accent);
  background: var(--soft);
}

.md-toolbar-button.pressed {
  color: var(--accent);
  background: var(--soft);
  box-shadow: inset 0 0 0 1px var(--line);
}

.md-toolbar-button:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: -1px;
}

.md-toolbar-button:disabled {
  cursor: default;
  opacity: 0.4;
}

.md-toolbar-button svg {
  flex-shrink: 0;
  width: 15px;
  height: 15px;
}

.md-toolbar-divider {
  flex: 0 0 auto;
  width: 1px;
  height: 15px;
  margin: 0 3px;
  background: var(--line);
}

@media (max-width: 640px) {
  .markdown-toolbar {
    gap: 1px;
    padding: 1px 2px;
  }

  .md-toolbar-button {
    width: 24px;
    height: 24px;
  }

  .md-toolbar-divider {
    margin: 0 1px;
  }
}
</style>
