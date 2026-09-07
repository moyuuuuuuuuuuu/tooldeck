<template>
  <div class="run-page" v-loading="loading">
    <template v-if="tool">
      <header
        ><ElButton text @click="router.push('/tooldeck/tools')">← 返回工具列表</ElButton
        ><div class="tool-heading"
          ><div
            ><span class="eyebrow">{{ tool.manifest.runtime.toUpperCase() }} · v{{ tool.manifest.version }}</span
            ><h1>{{ tool.manifest.title || tool.manifest.name }}</h1
            ><p>{{ tool.manifest.description }}</p></div
          ><div class="management-actions"><ElButton v-if="user.isLogin && tool.manifest.env?.length" type="warning" plain @click="openDetail('env')">配置运行环境变量</ElButton><ElButton v-if="user.isLogin && tool.api_enabled !== false" plain @click="openDetail('api')">API 参数</ElButton></div></div
        ></header
      >
      <main class="workspace">
        <section class="form-panel"
          ><div class="panel-heading"
            ><div><span>INPUT</span><h2>运行参数</h2></div
            ><ElTag v-if="tool.manifest.execution.mode === 'async'" type="info">异步执行</ElTag></div
          >
          <DynamicField v-for="[name, field] in orderedFields" :key="tool.id + name" :label="String(name)" :schema="field" :ui="tool.manifest.ui_schema?.[name] || {}" :required="tool.manifest.input_schema.required?.includes(String(name))" v-model="input[name]" @uploading="uploading += $event ? 1 : -1" />
          <ElAlert v-if="tool.build_status && tool.build_status !== 'ready'" title="工具尚未构建成功，暂时不能运行。" type="warning" :closable="false" />
          <div class="run-action"
            ><ElButton class="run-button" type="primary" size="large" :loading="running" :disabled="uploading > 0 || pending || (!!tool.build_status && tool.build_status !== 'ready')" @click="execute">{{ pending ? '正在执行' : '运行工具' }}</ElButton></div
          >
        </section>
        <section class="result-panel"
          ><div class="panel-heading"
            ><div><span>OUTPUT</span><h2>运行结果</h2></div
            ><ElButton v-if="result" text @click="refresh">刷新</ElButton></div
          ><ElEmpty v-if="!result" description="填写左侧参数并运行，结果将在这里显示" /><RunResult v-else :run="result" @update="result = $event"
        /></section>
      </main>
      <ElDrawer v-model="detailOpen" :title="tool.manifest.title || tool.manifest.name" size="min(680px, 94vw)" destroy-on-close
        ><ElTabs v-model="detailTab"
          ><ElTabPane v-if="user.isLogin && tool.manifest.env?.length" label="运行环境变量" name="env"><ElAlert :title="owner ? '个人配置与共享给使用者的配置分别保存；其他用户的个人值不会向作者或管理员展示。' : '这里保存的是你在此工具中的个人配置，与作者、管理员及其他用户完全隔离；保存后网页运行和你的 API 调用都会自动使用。'" type="warning" :closable="false" style="margin-bottom: 18px" /><ToolEnvironment :tool-id="tool.id" /></ElTabPane
          ><ElTabPane v-if="user.isLogin && tool.api_enabled !== false" label="API 参数" name="api"
            ><div class="api-title"
              ><div><span>POST</span><h3>执行此工具</h3></div
              ><ElButton text type="primary" @click="router.push('/tooldeck/guide?tab=api')">查看通用 API 指南 →</ElButton></div
            ><label class="endpoint-label">请求地址</label
            ><div class="endpoint"
              ><code>{{ apiEndpoint }}</code
              ><ElButton type="primary" plain @click="copyEndpoint">复制地址</ElButton></div
            ><h3>input 参数</h3
            ><ElTable :data="inputParameters" size="small"
              ><ElTableColumn prop="name" label="参数" min-width="130" /><ElTableColumn prop="type" label="类型" width="105" /><ElTableColumn label="必填" width="65"
                ><template #default="{ row }">{{ row.required ? '是' : '否' }}</template></ElTableColumn
              ><ElTableColumn prop="description" label="说明" min-width="180" /></ElTable
            ><template v-if="tool.manifest.env?.length"
              ><h3>可选 env 临时覆盖</h3
              ><ElTable :data="tool.manifest.env" size="small"
                ><ElTableColumn prop="name" label="变量" min-width="150" /><ElTableColumn label="必填" width="65"
                  ><template #default="{ row }">{{ row.required ? '是' : '否' }}</template></ElTableColumn
                ><ElTableColumn prop="description" label="说明" min-width="180" /></ElTable
              ><p class="api-hint">env 可省略并使用已保存的个人配置；传入时仅覆盖本次运行。</p></template
            ><h3>请求体示例</h3><pre>{{ requestBodyExample }}</pre>
          </ElTabPane></ElTabs
        ></ElDrawer
      >
    </template>
    <ElResult v-else-if="!loading" icon="warning" title="工具不可用" sub-title="工具不存在、尚未公开或当前账号无权访问"
      ><template #extra><ElButton type="primary" @click="router.push('/tooldeck/tools')">返回工具列表</ElButton></template></ElResult
    >
  </div>
</template>
<script setup lang="ts">
  import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  import { ElMessage } from 'element-plus'
  import { useUserStore } from '@/store/modules/user'
  import { td, type Field, type Run, type Tool } from '@/api/tooldeck'
  import DynamicField from '../components/DynamicField.vue'
  import RunResult from '../components/RunResult.vue'
  import ToolEnvironment from '../components/ToolEnvironment.vue'
  defineOptions({ name: 'ToolRun' })
  const route = useRoute(),
    router = useRouter(),
    user = useUserStore()
  const tool = ref<Tool>(),
    input = ref<Record<string, any>>({}),
    result = ref<Run | null>(null),
    loading = ref(true),
    running = ref(false),
    uploading = ref(0),
    detailOpen = ref(false),
    detailTab = ref('api')
  const orderedFields = computed(() => Object.entries(tool.value?.manifest.input_schema.properties || {}).sort(([a], [b]) => (tool.value?.manifest.ui_schema?.[a]?.order ?? 0) - (tool.value?.manifest.ui_schema?.[b]?.order ?? 0)))
  const pending = computed(() => ['queued', 'running'].includes(result.value?.status || ''))
  const owner = computed(() => user.info.roles?.includes('R_SUPER') || tool.value?.owner === String(user.info.id))
  const apiEndpoint = computed(() => `${location.origin}/api/v1/tools/${tool.value?.id || 'TOOL_ID'}/runs`)
  const inputParameters = computed(() =>
    orderedFields.value.map(([name, field]) => ({
      name,
      type: field.type + (field.format ? ` · ${field.format}` : ''),
      required: tool.value?.manifest.input_schema.required?.includes(name),
      description: field.description || field.title || '—'
    }))
  )
  const envExample = computed(() => Object.fromEntries((tool.value?.manifest.env || []).slice(0, 3).map((field) => [field.name, '<本次调用值>'])))
  const requestPayload = computed(() => (Object.keys(envExample.value).length ? { input: input.value, env: envExample.value } : { input: input.value }))
  const requestBodyExample = computed(() => JSON.stringify(requestPayload.value, null, 2))
  let timer: ReturnType<typeof setTimeout> | undefined
  function defaults(s: Field): any {
    if (s.default !== undefined) return structuredClone(s.default)
    if (s.type === 'object')
      return Object.fromEntries(
        Object.entries(s.properties || {})
          .map(([k, v]) => [k, defaults(v)])
          .filter(([, v]) => v !== undefined)
      )
    if (s.type === 'array') return []
    if (s.type === 'boolean') return false
    return undefined
  }
  function stopPoll() {
    if (timer) clearTimeout(timer)
    timer = undefined
  }
  function openDetail(tab: 'env' | 'api') {
    detailTab.value = tab
    detailOpen.value = true
  }
  async function copyEndpoint() {
    try {
      await navigator.clipboard.writeText(apiEndpoint.value)
      ElMessage.success('请求地址已复制')
    } catch {
      ElMessage.error('复制失败，请手动复制')
    }
  }
  async function refresh() {
    if (result.value) result.value = await td.run(result.value.run_id)
  }
  async function poll() {
    if (!result.value) return
    try {
      await refresh()
      if (pending.value) timer = setTimeout(poll, 1500)
    } catch {
      ElMessage.warning('状态刷新失败，请到使用记录查看；任务不会重复提交')
    }
  }
  async function execute() {
    if (!tool.value) return
    running.value = true
    try {
      result.value = await td.execute(tool.value.id, input.value)
      if (pending.value) timer = setTimeout(poll, 1000)
    } finally {
      running.value = false
    }
  }
  onMounted(async () => {
    try {
      const id = String(route.params.id || '')
      let list = await td.tools()
      tool.value = list.find((item) => item.id === id)
      if (!tool.value && user.isLogin) tool.value = (await td.list('my-tools')).find((item) => item.id === id)
      if (tool.value) input.value = defaults(tool.value.manifest.input_schema)
    } finally {
      loading.value = false
    }
  })
  onBeforeUnmount(stopPoll)
</script>
<style scoped>
  .run-page {
    position: relative;
    left: 50%;
    width: min(1680px, calc(100vw - 48px));
    min-height: calc(100vh - 190px);
    transform: translateX(-50%);
  }
  header {
    display: grid;
    gap: 18px;
    margin-bottom: 24px;
  }
  header > .el-button {
    justify-self: start;
    padding-left: 0;
  }
  .tool-heading {
    display: flex;
    justify-content: space-between;
    align-items: flex-end;
    gap: 28px;
  }
  .management-actions {
    display: flex;
    flex-shrink: 0;
    gap: 10px;
  }
  .eyebrow,
  .panel-heading span {
    font-size: 11px;
    letter-spacing: 1.8px;
    color: var(--el-color-primary);
    font-weight: 700;
  }
  h1 {
    font-size: 30px;
    margin: 8px 0;
  }
  header p {
    margin: 0;
    color: var(--el-text-color-secondary);
    line-height: 1.7;
  }
  .workspace {
    display: grid;
    grid-template-columns: minmax(460px, 0.95fr) minmax(560px, 1.25fr);
    gap: 28px;
    align-items: start;
  }
  .form-panel,
  .result-panel {
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 18px;
    padding: 28px;
    min-width: 0;
  }
  .form-panel {
    position: relative;
  }
  .result-panel {
    position: sticky;
    top: 18px;
    min-height: 520px;
  }
  .panel-heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 22px;
  }
  .panel-heading h2 {
    font-size: 20px;
    margin: 5px 0 0;
  }
  .run-action {
    position: sticky;
    z-index: 3;
    bottom: 0;
    margin: 24px -8px -8px;
    padding: 14px 8px 8px;
    background: linear-gradient(transparent, var(--el-bg-color) 18%);
  }
  .run-button {
    width: 100%;
    box-shadow: 0 8px 24px #409eff2e;
  }
  .form-panel > .el-alert {
    margin-top: 14px;
  }
  .api-title {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 16px;
    margin-bottom: 20px;
  }
  .api-title span {
    font-size: 11px;
    font-weight: 700;
    color: var(--el-color-success);
  }
  .api-title h3 {
    margin: 4px 0 0;
  }
  .endpoint-label {
    display: block;
    margin-bottom: 8px;
    font-size: 13px;
    font-weight: 600;
  }
  .endpoint {
    display: flex;
    align-items: center;
    gap: 10px;
    margin-bottom: 24px;
    padding: 10px 10px 10px 14px;
    border: 1px solid var(--el-border-color);
    border-radius: 10px;
    background: var(--el-fill-color-light);
  }
  .endpoint code {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
  }
  .api-hint {
    color: var(--el-text-color-secondary);
    font-size: 13px;
    line-height: 1.8;
  }
  .run-page :deep(.el-drawer pre) {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    padding: 18px;
    background: var(--el-fill-color-light);
    border-radius: 10px;
    max-height: 460px;
    overflow: auto;
  }
  @media (max-width: 1100px) {
    .workspace {
      grid-template-columns: 1fr;
    }
    .result-panel {
      position: static;
      min-height: 320px;
    }
  }
  @media (max-width: 700px) {
    .run-page {
      width: 100%;
      left: auto;
      transform: none;
    }
    .tool-heading {
      align-items: flex-start;
      flex-direction: column;
    }
    .management-actions {
      width: 100%;
      flex-wrap: wrap;
    }
    .endpoint,
    .api-title {
      align-items: stretch;
      flex-direction: column;
    }
    h1 {
      font-size: 25px;
    }
    .form-panel,
    .result-panel {
      padding: 18px;
      border-radius: 14px;
    }
  }
</style>
