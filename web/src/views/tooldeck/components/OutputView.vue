<template>
  <section class="output-view">
    <div class="output-heading">
      <div class="format-label">{{ label }}</div>
      <ElButton v-if="copyable" link type="primary" @click="copyOutput">复制结果</ElButton>
    </div>
    <ElTabs v-if="format === 'html' && typeof value === 'string'" v-model="tab">
      <ElTabPane label="页面预览" name="preview"
        ><p class="hint">静态预览：脚本、表单、外部资源和链接跳转已禁用。</p
        ><iframe title="工具 HTML 结果预览" sandbox="" referrerpolicy="no-referrer" :srcdoc="html"
      /></ElTabPane>
      <ElTabPane label="HTML 源码" name="source">
        <pre>{{ value }}</pre>
      </ElTabPane>
    </ElTabs>
    <ElTabs v-else-if="format === 'markdown' && typeof value === 'string'" v-model="tab">
      <ElTabPane label="页面预览" name="preview"
        ><p class="hint">静态预览：原始 HTML、外部资源和链接跳转已禁用。</p
        ><div class="preview-scroll"
          ><MdPreview :model-value="value" :sanitize="sanitizeOutputHtml" :theme="siteTheme" /></div
      ></ElTabPane>
      <ElTabPane label="Markdown 源码" name="source">
        <pre>{{ value }}</pre>
      </ElTabPane>
    </ElTabs>
    <ElTabs v-else-if="format === 'csv' && typeof value === 'string'" v-model="tab">
      <ElTabPane label="表格预览" name="preview"
        ><div v-if="rows" class="csv-table"
          ><table
            ><tbody
              ><tr v-for="(row, i) in rows" :key="i"
                ><td v-for="(cell, j) in row" :key="j">{{ cell }}</td></tr
              ></tbody
            ></table
          ></div
        ><p v-else class="hint"
          >无法预览，或超过 100,000 字符 / 200 行 / 40 列限制，请查看 CSV 源码。</p
        ></ElTabPane
      >
      <ElTabPane label="CSV 源码" name="source">
        <pre>{{ value }}</pre>
      </ElTabPane>
    </ElTabs>
    <template v-else
      ><p v-if="requiresText && typeof value !== 'string'" class="hint"
        >该格式需要字符串，当前结果按 JSON 展示。</p
      ><pre>{{ text }}</pre>
    </template>
  </section>
</template>
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { ElMessage } from 'element-plus'
  import { MdPreview } from 'md-editor-v3'
  import 'md-editor-v3/lib/preview.css'
  import { useSettingStore } from '@/store/modules/setting'
  import {
    csvPreview,
    htmlPreview,
    normalizeOutputType,
    outputFormats,
    sanitizeOutputHtml
  } from './outputPresentation'
  const settingStore = useSettingStore()
  const props = defineProps<{ value: unknown; type?: string }>()
  const siteTheme = computed(() => (settingStore.isDark ? 'dark' : 'light'))
  const format = computed(() => normalizeOutputType(props.type))
  const label = computed(() => outputFormats.find((item) => item.value === format.value)?.label)
  const requiresText = computed(() =>
    ['html', 'markdown', 'text', 'stream', 'csv', 'xml'].includes(format.value)
  )
  const text = computed(() =>
    typeof props.value === 'string'
      ? props.value
      : (JSON.stringify(props.value, null, 2) ?? String(props.value))
  )
  const copyable = computed(() => format.value !== 'image-gallery')
  const html = computed(() =>
    format.value === 'html' && typeof props.value === 'string' ? htmlPreview(props.value) : ''
  )
  const rows = computed(() =>
    format.value === 'csv' && typeof props.value === 'string' ? csvPreview(props.value) : null
  )
  const tab = ref('preview')
  async function copyOutput() {
    try {
      await navigator.clipboard.writeText(text.value)
      ElMessage.success('运行结果已复制')
    } catch {
      ElMessage.error('复制失败，请检查浏览器剪贴板权限')
    }
  }
  watch(
    () => props.type,
    () => {
      tab.value = 'preview'
    }
  )
</script>
<style scoped>
  .output-view {
    margin-top: 16px;
  }
  .format-label,
  .hint {
    font-size: 12px;
    color: var(--el-text-color-secondary);
    margin: 8px 0;
  }
  .output-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }
  pre {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: var(--el-fill-color-light);
    padding: 16px;
    border-radius: 8px;
    max-height: 480px;
    overflow: auto;
  }
  iframe {
    width: 100%;
    height: 480px;
    border: 1px solid var(--el-border-color);
    border-radius: 8px;
    background: white;
  }
  .csv-table {
    max-height: 480px;
    overflow: auto;
  }
  .preview-scroll {
    max-height: 480px;
    overflow: auto;
    border-radius: 8px;
    background: var(--el-fill-color-light);
  }
  .preview-scroll :deep(.md-editor-previewOnly) {
    --md-bk-color: var(--el-fill-color-light);
    --md-color: var(--el-text-color-primary);
  }
  .preview-scroll :deep(.md-editor-preview) {
    padding: 20px 24px;
  }
  @media (max-width: 600px) {
    .preview-scroll :deep(.md-editor-preview) {
      padding: 16px;
    }
  }
  table {
    border-collapse: collapse;
    width: 100%;
  }
  td {
    border: 1px solid var(--el-border-color);
    padding: 8px 12px;
    white-space: pre-wrap;
    min-width: 80px;
    max-width: 400px;
    overflow-wrap: anywhere;
  }
</style>
