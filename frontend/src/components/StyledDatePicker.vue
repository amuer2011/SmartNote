<script setup lang="ts">
import { computed } from 'vue'
import { NDatePicker } from 'naive-ui'

type PickerType = 'date' | 'month'

const MIN_YEAR = 1900
const MAX_YEAR = 2100

const props = withDefaults(defineProps<{
  modelValue: string
  type?: PickerType
  label: string
  clearable?: boolean
}>(), {
  type: 'date',
  clearable: true,
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
}>()

// NDatePicker 通过 formatted-value + value-format 输出受控字符串。
const valueFormat = computed(() => (props.type === 'month' ? 'yyyy-MM' : 'yyyy-MM-dd'))

// '' 需要转成 null，否则会被当成非法日期解析。
const formattedValue = computed<string | null>(() => (props.modelValue ? props.modelValue : null))

// date 面板点击某天即提交，无需确认按钮；month 面板必须依赖确认按钮提交。
const actions = computed<Array<'clear' | 'confirm' | 'now'>>(() => {
  const result: Array<'clear' | 'confirm' | 'now'> = ['now']
  if (props.clearable) result.push('clear')
  if (props.type === 'month') result.push('confirm')
  return result
})

const yearRange: [number, number] = [MIN_YEAR, MAX_YEAR]

function isDateDisabled(timestamp: number): boolean {
  const year = new Date(timestamp).getFullYear()
  return year < MIN_YEAR || year > MAX_YEAR
}

function handleFormattedUpdate(value: unknown) {
  emit('update:modelValue', typeof value === 'string' ? value : '')
}
</script>

<template>
  <label class="smartnote-styled-date-picker"><span class="date-picker-label">{{label}}</span><NDatePicker to="body"
    :formatted-value="formattedValue"
    :value-format="valueFormat"
    :type="type"
    :placeholder="label"
    :clearable="clearable"
    :actions="actions"
    :is-date-disabled="isDateDisabled"
    :year-range="yearRange"
    @update:formatted-value="handleFormattedUpdate"
  /></label>
</template>

<style>
/* 零特异性选择器：默认宽度 160px，外部 class/style 可随时覆盖。 */
:where(.smartnote-styled-date-picker) {
  display:inline-block;vertical-align:middle;width:160px;
}
.date-picker-label{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%);white-space:nowrap}
</style>
