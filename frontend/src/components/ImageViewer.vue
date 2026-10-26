<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { Download, Expand, Shrink, Maximize, PanelLeftClose, RotateCw, X, ZoomIn, ZoomOut } from 'lucide-vue-next'

const props = defineProps<{ src: string; name?: string; downloadable?: boolean; downloading?: boolean }>()
const emit = defineEmits<{ close: []; minimize: []; download: [] }>()
const stage = ref<HTMLElement>()
const natural = ref({ width: 0, height: 0 })
const viewport = ref({ width: window.innerWidth, height: window.innerHeight })
const size = ref({ width: 440, height: 160 })
const zoom = ref(1), rotation = ref(0), pan = ref({ x: 0, y: 0 })
const loading = ref(true), failed = ref(false), dragging = ref(false)
const fullscreen = ref(false)
const windowOffset = ref({ x: 0, y: 0 }), windowMoving = ref(false)
let windowGesture: { x:number; y:number; offsetX:number; offsetY:number } | undefined
function clampWindow() {
  const maxX = Math.max(0, (viewport.value.width - Math.min(size.value.width, viewport.value.width - 24)) / 2 - 12)
  const maxY = Math.max(0, (viewport.value.height - Math.min(size.value.height, viewport.value.height - 24)) / 2 - 12)
  windowOffset.value = { x: Math.max(-maxX, Math.min(maxX, windowOffset.value.x)), y: Math.max(-maxY, Math.min(maxY, windowOffset.value.y)) }
}
function startWindowMove(event:PointerEvent) {
  if (event.button !== 0 || fullscreen.value || (event.target as HTMLElement).closest('button')) return
  event.preventDefault()
  windowGesture = { x:event.clientX, y:event.clientY, offsetX:windowOffset.value.x, offsetY:windowOffset.value.y }
  windowMoving.value = true
  ;(event.currentTarget as HTMLElement).setPointerCapture(event.pointerId)
}
function moveWindow(event:PointerEvent) {
  if (!windowGesture) return
  windowOffset.value = { x:windowGesture.offsetX + event.clientX - windowGesture.x, y:windowGesture.offsetY + event.clientY - windowGesture.y }
  clampWindow()
}
function stopWindowMove() { windowGesture = undefined; windowMoving.value = false }
watch([size, viewport], clampWindow, { flush:'sync' })
let restoredSize = { width: 440, height: 160 }
async function toggleFullscreen() {
  if (!fullscreen.value) restoredSize = { ...size.value }
  fullscreen.value = !fullscreen.value
  if (!fullscreen.value) size.value = { width: Math.min(restoredSize.width, viewport.value.width - 24), height: Math.min(restoredSize.height, viewport.value.height - 24) }
  await nextTick()
  if (fitting) fit()
}
function escape(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  event.preventDefault(); event.stopImmediatePropagation()
  if (fullscreen.value) void toggleFullscreen()
  else emit('close')
}
let fitting = true
let resizeGesture: { x: number; y: number; width: number; height: number } | undefined
let observer: ResizeObserver | undefined
let gesture: { x: number; y: number; startX: number; startY: number; blank: boolean; moved: boolean } | undefined
const rotatedSize = computed(() => rotation.value % 180
  ? { width: natural.value.height, height: natural.value.width } : natural.value)
const panelStyle = computed(() => ({ width: `${fullscreen.value ? viewport.value.width : size.value.width}px`, height: `${fullscreen.value ? viewport.value.height : size.value.height}px`,
  transform: fullscreen.value ? undefined : `translate(${windowOffset.value.x}px, ${windowOffset.value.y}px)`,
  maxWidth: `${viewport.value.width - (fullscreen.value ? 0 : 24)}px`, maxHeight: `${viewport.value.height - (fullscreen.value ? 0 : 24)}px` }))
const imageStyle = computed(() => ({ width: `${natural.value.width}px`, height: `${natural.value.height}px`,
  transform: `translate(-50%, -50%) translate(${pan.value.x}px, ${pan.value.y}px) rotate(${rotation.value}deg) scale(${zoom.value})` }))

function fit() {
  if (!stage.value || !natural.value.width) return
  const bounds = stage.value.getBoundingClientRect(), dimensions = rotatedSize.value
  zoom.value = Math.min(1, Math.max(.001, (bounds.width - 24) / dimensions.width), Math.max(.001, (bounds.height - 24) / dimensions.height))
  pan.value = { x: 0, y: 0 }
  fitting = true
}
async function loaded(event: Event) {
  const img = event.target as HTMLImageElement
  natural.value = { width: img.naturalWidth, height: img.naturalHeight }
  loading.value = false
  const scale = Math.min(1, (viewport.value.width - 50) / img.naturalWidth, (viewport.value.height - 100) / img.naturalHeight)
  size.value = { width: Math.max(Math.min(440, viewport.value.width - 24), Math.ceil(img.naturalWidth * scale) + 26),
    height: Math.max(136, Math.ceil(img.naturalHeight * scale) + 76) }
  await nextTick()
  fit()
}
function setZoom(value: number) {
  fitting = false
  zoom.value = Math.max(.01, Math.min(8, value))
}
function original() { setZoom(1); pan.value = { x: 0, y: 0 } }
function rotate() { rotation.value = (rotation.value + 90) % 360; fit() }
function resizeViewport() {
  viewport.value = { width: window.innerWidth, height: window.innerHeight }
  size.value = { width: Math.min(size.value.width, viewport.value.width - 24), height: Math.min(size.value.height, viewport.value.height - 24) }
  nextTick(() => { if (fitting) fit() })
}
function startResize(e: PointerEvent) {
  if (e.button !== 0) return
  resizeGesture = { x: e.clientX, y: e.clientY, ...size.value }
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}
function moveResize(e: PointerEvent) {
  if (!resizeGesture) return
  size.value = { width: Math.min(viewport.value.width - 24, Math.max(360, resizeGesture.width + (e.clientX - resizeGesture.x) * 2)),
    height: Math.min(viewport.value.height - 24, Math.max(136, resizeGesture.height + (e.clientY - resizeGesture.y) * 2)) }
}
function pointerDown(e: PointerEvent) {
  if (e.button !== 0) return
  gesture = { x: e.clientX, y: e.clientY, startX: e.clientX, startY: e.clientY, blank: e.target === e.currentTarget, moved: false }
  dragging.value = true
  ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
}
function pointerMove(e: PointerEvent) {
  if (!gesture) return
  if (Math.hypot(e.clientX - gesture.startX, e.clientY - gesture.startY) > 5) gesture.moved = true
  if (gesture.moved) {
    pan.value = { x: pan.value.x + e.clientX - gesture.x, y: pan.value.y + e.clientY - gesture.y }
    fitting = false
  }
  gesture.x = e.clientX; gesture.y = e.clientY
}
function pointerUp(e: PointerEvent) {
  const saved = gesture
  gesture = undefined; dragging.value = false
  if (saved?.blank && !saved.moved && Math.hypot(e.clientX - saved.startX, e.clientY - saved.startY) <= 5) emit('close')
}
watch(() => props.src, () => {
  natural.value = { width: 0, height: 0 }; loading.value = true; failed.value = false
  windowOffset.value = { x:0, y:0 }; stopWindowMove()
  rotation.value = 0; pan.value = { x: 0, y: 0 }; zoom.value = 1; fitting = true
})
onMounted(() => {
  observer = new ResizeObserver(() => { if (fitting) fit() })
  if (stage.value) observer.observe(stage.value)
  window.addEventListener('resize', resizeViewport)
  window.addEventListener('keydown', escape, true)
})
onBeforeUnmount(() => { observer?.disconnect(); window.removeEventListener('resize', resizeViewport); window.removeEventListener('keydown', escape, true) })
</script>

<template>
  <Teleport to="body">
    <div class="image-backdrop" aria-label="图片查看器背景" @click.self="emit('close')">
      <section class="image-viewer" :class="{fullscreen}" role="dialog" aria-modal="true" aria-label="图片查看器" :style="panelStyle">
        <div class="image-toolbar" :class="{ moving:windowMoving }" :title="fullscreen ? undefined : '拖动移动图片窗口'" @pointerdown="startWindowMove" @pointermove="moveWindow" @pointerup="stopWindowMove" @pointercancel="stopWindowMove" @lostpointercapture="stopWindowMove">
          <span class="image-name" :title="name || '原始图片'">{{ name || '原始图片' }}</span>
          <button v-if="downloadable" title="下载图片附件" aria-label="下载图片附件" :disabled="downloading" @click="emit('download')"><Download :size="17" /></button>
          <button title="隐藏图片查看器" aria-label="隐藏图片查看器" @click="emit('minimize')"><PanelLeftClose :size="17" /></button>
          <button title="缩小" aria-label="缩小" :disabled="loading || failed" @click="setZoom(zoom / 1.2)"><ZoomOut :size="17" /></button>
          <button class="image-scale" title="原始尺寸（100%）" aria-label="原始尺寸" :disabled="loading || failed" @click="original">{{ Math.round(zoom * 100) }}%</button>
          <button title="放大" aria-label="放大" :disabled="loading || failed" @click="setZoom(zoom * 1.2)"><ZoomIn :size="17" /></button>
          <button title="适应窗口" aria-label="适应窗口" :disabled="loading || failed" @click="fit"><Maximize :size="17" /></button>
          <button :title="fullscreen ? '恢复图片窗口' : '全屏显示图片'" :aria-label="fullscreen ? '恢复图片窗口' : '全屏显示图片'" :aria-pressed="fullscreen" @click="toggleFullscreen"><Shrink v-if="fullscreen" :size="17"/><Expand v-else :size="17"/></button>
          <button title="旋转" aria-label="旋转" :disabled="loading || failed" @click="rotate"><RotateCw :size="17" /></button>
          <button title="关闭" aria-label="关闭图片查看器" @click="emit('close')"><X :size="20" /></button>
        </div>
        <div v-if="!fullscreen" class="image-resize" title="调整图片窗口大小" aria-label="调整图片窗口大小" @pointerdown.stop.prevent="startResize" @pointermove="moveResize" @pointerup="resizeGesture = undefined" @pointercancel="resizeGesture = undefined" />
        <div ref="stage" class="image-stage" :class="{ dragging }" @pointerdown="pointerDown" @pointermove="pointerMove" @pointerup="pointerUp" @pointercancel="gesture = undefined; dragging = false" @dblclick="zoom === 1 ? fit() : original()" @wheel.prevent="setZoom(zoom * Math.exp(-$event.deltaY * .001))">
          <img :key="src" :src="src" :alt="name || '原始图片'" draggable="false" :style="imageStyle" :class="{ 'image-pending': loading || failed }" @load="loaded" @error="loading = false; failed = true" />
          <span v-if="loading || failed" class="image-message">{{ failed ? '图片加载失败，请检查图片地址或网络连接' : '正在加载图片…' }}</span>
        </div>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.image-backdrop{position:fixed;inset:0;z-index:6000;display:flex;align-items:center;justify-content:center;background:#14121c80;backdrop-filter:blur(3px)}
.image-viewer{position:relative;display:flex;flex-direction:column;box-sizing:border-box;min-width:min(360px,calc(100vw - 24px));min-height:min(136px,calc(100vh - 24px));overflow:hidden;background:var(--bg);border:1px solid var(--line);border-radius:10px;box-shadow:0 16px 64px #0004;color:var(--text)}
.image-viewer.fullscreen{border:0;border-radius:0}
.image-resize{position:absolute;right:0;bottom:0;width:18px;height:18px;z-index:1;cursor:nwse-resize;touch-action:none;background:repeating-linear-gradient(135deg,transparent 0 3px,var(--muted) 3px 4px);clip-path:polygon(100% 0,100% 100%,0 100%);opacity:.5}
.image-toolbar{display:flex;align-items:center;flex-shrink:0;gap:4px;height:50px;padding:0 10px;border-bottom:1px solid var(--line);box-sizing:border-box;color:var(--secondary)}
.image-toolbar{cursor:grab;touch-action:none;user-select:none}.image-toolbar.moving{cursor:grabbing}.fullscreen .image-toolbar{cursor:default}
.image-name{flex:1;min-width:0;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:.85714rem;margin-right:8px}
.image-toolbar>button{display:inline-flex;align-items:center;justify-content:center;flex-shrink:0;width:28px;height:30px;border-radius:5px;color:inherit;padding:0}
.image-toolbar>button:hover{background:var(--hover);color:var(--accent)}
.image-toolbar>button:disabled{opacity:.4;cursor:default}
.image-toolbar>button.image-scale{width:48px;font-size:.78571rem;font-variant-numeric:tabular-nums}
.image-stage{position:relative;flex:1;min-height:0;overflow:hidden;cursor:grab;touch-action:none;background:var(--soft)}
.image-stage.dragging{cursor:grabbing}
.image-stage img{position:absolute;left:50%;top:50%;max-width:none;max-height:none;user-select:none;transform-origin:center}
.image-stage img.image-pending{visibility:hidden}
.image-message{position:absolute;inset:0;display:flex;align-items:center;justify-content:center;padding:16px;text-align:center;font-size:.85714rem;color:var(--muted);pointer-events:none}
</style>
