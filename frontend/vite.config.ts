import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import {fileURLToPath} from 'node:url'
export default defineConfig({
 plugins: [vue()],
 server: { port: 34115, strictPort: true, fs: {allow: [fileURLToPath(new URL('.',import.meta.url)),fileURLToPath(new URL('../internal/core/llm_reasoning_models.json',import.meta.url))]} },
 build: { rollupOptions: { output: { manualChunks: {
  editor: ['codemirror', '@codemirror/lang-markdown', '@codemirror/view'],
  graph: ['cytoscape'],
  ui: ['naive-ui'],
  markdown: ['markdown-it', 'dompurify'],
  vue: ['vue', 'pinia']
 } } } }
})
