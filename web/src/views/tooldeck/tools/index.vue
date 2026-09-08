<template>
  <div class="tool-page">
    <header class="tooldeck-page-hero"
      ><div
        ><span class="eyebrow">YOUR PERSONAL TOOLKIT</span><h1>找到工具，让想法即刻发生</h1
        ><p>选择合适的工具，填写参数或上传素材，即刻获得结果。</p></div
      ><div class="actions"
        ><ElButton size="large" @click="openGuide">开发文档</ElButton
        ><ElButton type="primary" size="large" @click="openUpload">＋ 上传工具包</ElButton></div
      ></header
    >
    <div class="filters"
      ><ElInput
        v-model="search"
        placeholder="搜索工具名称或用途"
        clearable
        style="max-width: 340px"
      /><ElSelect
        v-if="isAdmin()"
        v-model="runtime"
        clearable
        placeholder="所有运行环境"
        style="width: 170px"
        ><ElOption
          v-for="r in ['php', 'js', 'node', 'python', 'go']"
          :key="r"
          :label="r"
          :value="r" /></ElSelect
      ><ElButton @click="load">刷新</ElButton></div
    >
    <ElEmpty v-if="!loading && !filtered.length" description="暂时没有可用工具，请稍后再来看看。" />
    <div v-loading="loading" class="cards">
      <ElCard v-for="tool in filtered" :key="tool.id" shadow="never" class="tool-card">
        <div class="card-top">
          <span class="runtime">{{
            isAdmin() ? tool.manifest.runtime.toUpperCase() : '在线工具'
          }}</span>
          <span
            class="visibility"
            :class="
              tool.withdrawn
                ? 'muted'
                : tool.review_status === 'rejected'
                  ? 'rejected'
                  : ['draft', 'pending'].includes(tool.review_status || '')
                    ? 'pending'
                    : tool.public === false
                      ? 'muted'
                      : 'public'
            "
            ><i />{{
              tool.withdrawn
                ? '已下架'
                : tool.review_status === 'draft'
                  ? '待构建/提交'
                  : tool.public === false
                    ? '私有'
                    : tool.review_status === 'pending'
                      ? '审核中'
                      : tool.review_status === 'rejected'
                        ? '已驳回'
                        : '公开'
            }}</span
          >
          <span class="execution-mode">{{
            needsLogin(tool) && !useUserStore().isLogin
              ? '登录后使用'
              : tool.manifest.execution.stream
                ? '流式输出'
                : tool.manifest.execution.mode === 'async'
                  ? '后台处理'
                  : '即时返回'
          }}</span>
        </div>
        <h2 :title="tool.manifest.title || tool.manifest.name">{{
          tool.manifest.title || tool.manifest.name
        }}</h2>
        <p class="card-description" :title="tool.manifest.description">{{
          tool.manifest.description || '暂无说明'
        }}</p>
        <p v-if="tool.review_status === 'rejected' && !tool.withdrawn" class="review-note"
          >审核意见：{{ tool.review_note || '请调整后上传新版本' }}</p
        >
        <ElTag
          v-if="tool.build_status && tool.build_status !== 'ready'"
          class="build-status"
          size="small"
          :type="tool.build_status === 'failed' ? 'danger' : 'warning'"
          >{{ buildLabels[tool.build_status] || tool.build_status }}</ElTag
        >
        <div class="card-bottom"
          ><span class="version">v{{ tool.manifest.version }}</span
          ><ElButton type="primary" plain @click="openTool(tool)"
            >运行工具 <span class="run-arrow" aria-hidden="true">→</span></ElButton
          ></div
        >
      </ElCard>
    </div>
    <ElDialog v-model="uploadOpen" title="上传工具包草稿" width="520px" @closed="resetUploadForm">
      <ElUpload
        ref="uploadRef"
        class="package-upload"
        drag
        accept=".zip"
        :auto-upload="false"
        :limit="1"
        :on-change="onZipChange"
        :on-remove="onZipRemove"
      >
        <p>将 ZIP 拖到这里，或点击选择</p>
        <small>最大 64 MB</small>
      </ElUpload>
      <p class="intro"
        >ZIP 根目录包含 tooldeck.json
        和入口代码。选择文件后自动读取包内声明；之后手动修改的字段会优先保留。本步骤只保存源码草稿，不会自动构建或提交审核。</p
      >
      <ElAlert
        v-if="manifestSummary"
        :title="manifestSummary"
        type="success"
        :closable="false"
        style="margin-bottom: 16px"
      />
      <ElAlert
        v-if="manifestError"
        :title="manifestError"
        type="error"
        :closable="false"
        style="margin-bottom: 16px"
      />
      <ElFormItem label="构建环境版本"
        ><ElSelect
          v-model="buildVersion"
          placeholder="使用代码包声明或平台默认版本"
          clearable
          style="width: 100%"
          @change="markUploadDirty('buildVersion')"
          ><ElOptionGroup
            v-for="(versions, language) in versionOptions"
            :key="language"
            :label="String(language)"
            ><ElOption
              v-for="v in versions"
              :key="language + v"
              :value="String(language) + ':' + v"
              :label="language + ' ' + v" /></ElOptionGroup></ElSelect
      ></ElFormItem>
      <ElFormItem label="构建命令（可选）"
        ><ElInput
          v-model="buildCommand"
          placeholder="例如 npm run build，留空使用包内声明"
          maxlength="512"
          @input="markUploadDirty('buildCommand')"
      /></ElFormItem>
      <p class="intro"
        >不必上传 node_modules 或
        vendor。保存后请到“我上传的工具”主动开始构建；只有构建成功后才能提交审核。</p
      >
      <div style="margin-bottom: 12px"
        ><ElCheckbox v-model="isPublic">公开工具（构建成功后提交审核）</ElCheckbox></div
      >
      <ElCheckbox v-model="thirdParty" @change="markUploadDirty('thirdParty')"
        >使用第三方服务</ElCheckbox
      >
      <ElInput
        v-if="thirdParty"
        v-model="allowedHosts"
        placeholder="允许访问的域名，逗号分隔；留空表示清空包内声明"
        style="margin: 12px 0"
        @input="markUploadDirty('allowedHosts')"
      />
      <ElFormItem label="SSE 流式输出"
        ><ElSelect v-model="streamMode" style="width: 100%" @change="markUploadDirty('streamMode')"
          ><ElOption value="" label="跟随工具包声明（未声明则关闭）" /><ElOption
            value="true"
            label="开启 SSE 流式输出" /><ElOption
            value="false"
            label="关闭 SSE 流式输出" /></ElSelect
      ></ElFormItem>
      <p v-if="streamMode === 'true'" class="intro"
        >工具需按协议发送增量事件；网页实时显示，API 可订阅 SSE。与完成通知独立。</p
      >
      <div style="margin-bottom: 18px"
        ><ElCheckbox v-model="apiEnabled">允许 API 调用</ElCheckbox></div
      >
      <ElSelect
        v-model="uploadMode"
        style="width: 100%; margin-bottom: 18px"
        @change="markUploadDirty('uploadMode')"
        ><ElOption value="" label="使用代码包声明的运行模式" /><ElOption
          value="sync"
          label="同步返回结果" /><ElOption value="async" label="异步执行，通过调用方回调返回结果"
      /></ElSelect>
      <template #footer
        ><ElButton @click="uploadOpen = false">取消</ElButton
        ><ElButton
          type="primary"
          :loading="uploadBusy || manifestLoading"
          :disabled="!zip || !!manifestError"
          @click="publish"
          >上传源码并保存草稿</ElButton
        ></template
      >
    </ElDialog>
  </div>
</template>
<script setup lang="ts">
  import { useUserStore } from '@/store/modules/user'
  const isAdmin = () => useUserStore().info.roles?.includes('R_SUPER')

  import { ref, computed, onMounted } from 'vue'
  import { useRoute, useRouter } from 'vue-router'
  const route = useRoute(),
    router = useRouter()
  import { ElMessage } from 'element-plus'
  import type { UploadInstance } from 'element-plus'
  import JSZip from 'jszip'
  import { td, type Tool } from '@/api/tooldeck'
  defineOptions({ name: 'Tools' })
  const tools = ref<Tool[]>([]),
    loading = ref(false),
    search = ref(''),
    runtime = ref('')
  const buildLabels: Record<string, string> = {
    queued: '等待构建',
    building: '正在构建',
    failed: '构建失败'
  }
  const buildVersion = ref(''),
    buildCommand = ref('')
  const versionOptions: Record<string, string[]> = {
    php: ['8.0', '8.1', '8.2', '8.3'],
    node: ['20', '21', '22', '23'],
    python: ['3.10', '3.11', '3.12'],
    go: ['1.22', '1.23', '1.24']
  }
  const isPublic = ref(true)
  const streamMode = ref('')
  const thirdParty = ref(false),
    allowedHosts = ref(''),
    apiEnabled = ref(true)
  type UploadField =
    'buildVersion' | 'buildCommand' | 'thirdParty' | 'allowedHosts' | 'streamMode' | 'uploadMode'
  type UploadManifest = {
    schema_version?: number
    name?: string
    title?: string
    version?: string
    runtime?: string
    runtime_version?: string
    build_command?: string
    execution?: { mode?: string; stream?: boolean }
    network?: { enabled?: boolean; allowed_hosts?: string[] }
  }
  const uploadOpen = ref(false),
    uploadMode = ref(''),
    zip = ref<File>(),
    uploadBusy = ref(false),
    manifestLoading = ref(false),
    manifestError = ref(''),
    manifestSummary = ref('')
  const uploadRef = ref<UploadInstance>()
  const uploadDirty = new Set<UploadField>()
  const filtered = computed(() =>
    tools.value.filter(
      (t) =>
        (!runtime.value || t.manifest.runtime === runtime.value) &&
        `${t.manifest.name} ${t.manifest.title} ${t.manifest.description}`
          .toLowerCase()
          .includes(search.value.toLowerCase())
    )
  )
  async function load() {
    loading.value = true
    try {
      tools.value = await td.tools()
    } finally {
      loading.value = false
    }
  }
  function loginFor(path: string) {
    router.push({ path: '/auth/login', query: { redirect: path } })
  }
  function needsLogin(tool: Tool) {
    return Boolean(tool.manifest.env?.length || tool.manifest.secrets?.length)
  }
  function openTool(tool: Tool) {
    const target = '/tooldeck/run/' + encodeURIComponent(tool.id)
    if (needsLogin(tool) && !useUserStore().isLogin) {
      loginFor(target)
      return
    }
    if (useUserStore().isLogin) router.push(target)
    else router.push({ path: '/explore', query: { tool: tool.id } })
  }
  function openUpload() {
    if (!useUserStore().isLogin) {
      loginFor('/tooldeck/tools?upload=1')
      return
    }
    uploadOpen.value = true
  }
  function openGuide() {
    if (useUserStore().isLogin) router.push('/tooldeck/guide')
    else router.push({ path: '/explore', query: { view: 'guide' } })
  }
  function markUploadDirty(field: UploadField) {
    uploadDirty.add(field)
  }
  function setFromManifest(field: UploadField, value: unknown) {
    if (uploadDirty.has(field)) return
    if (field === 'buildVersion') buildVersion.value = String(value ?? '')
    else if (field === 'buildCommand') buildCommand.value = String(value ?? '')
    else if (field === 'thirdParty') thirdParty.value = Boolean(value)
    else if (field === 'allowedHosts') allowedHosts.value = String(value ?? '')
    else if (field === 'streamMode') streamMode.value = String(value ?? '')
    else uploadMode.value = String(value ?? '')
  }
  async function onZipChange(file: any) {
    zip.value = file.raw
    manifestError.value = ''
    manifestSummary.value = ''
    if (!zip.value) return
    manifestLoading.value = true
    try {
      const archive = await JSZip.loadAsync(zip.value)
      const entry = archive.file('tooldeck.json')
      if (!entry) throw new Error('ZIP 根目录缺少 tooldeck.json')
      const manifest = JSON.parse(await entry.async('string')) as UploadManifest
      if (!manifest || typeof manifest !== 'object' || manifest.schema_version !== 1)
        throw new Error('tooldeck.json 无效或 schema_version 不是 1')
      const runtime =
        { js: 'node', py: 'python', golang: 'go' }[manifest.runtime || ''] || manifest.runtime || ''
      if (manifest.runtime_version)
        setFromManifest('buildVersion', `${runtime}:${manifest.runtime_version}`)
      else setFromManifest('buildVersion', '')
      setFromManifest('buildCommand', manifest.build_command || '')
      if (manifest.network) {
        setFromManifest('thirdParty', manifest.network.enabled === true)
        setFromManifest(
          'allowedHosts',
          Array.isArray(manifest.network.allowed_hosts)
            ? manifest.network.allowed_hosts.join(', ')
            : ''
        )
      }
      setFromManifest(
        'streamMode',
        typeof manifest.execution?.stream === 'boolean' ? String(manifest.execution.stream) : ''
      )
      setFromManifest(
        'uploadMode',
        ['sync', 'async'].includes(manifest.execution?.mode || '') ? manifest.execution?.mode : ''
      )
      manifestSummary.value =
        `已读取：${manifest.title || manifest.name || '未命名工具'} ${manifest.version ? 'v' + manifest.version : ''}`.trim()
    } catch (error) {
      manifestError.value = error instanceof Error ? error.message : '无法读取 tooldeck.json'
    } finally {
      manifestLoading.value = false
    }
  }
  function onZipRemove() {
    zip.value = undefined
    manifestError.value = ''
    manifestSummary.value = ''
  }
  function resetUploadForm() {
    uploadDirty.clear()
    buildVersion.value = ''
    buildCommand.value = ''
    thirdParty.value = false
    allowedHosts.value = ''
    streamMode.value = ''
    uploadMode.value = ''
    isPublic.value = true
    apiEnabled.value = true
    zip.value = undefined
    manifestError.value = ''
    manifestSummary.value = ''
    uploadRef.value?.clearFiles()
  }
  async function publish() {
    if (!zip.value) return
    uploadBusy.value = true
    try {
      const options: Record<string, string> = {
        public: String(isPublic.value),
        api_enabled: String(apiEnabled.value)
      }
      let mode = ''
      if (uploadDirty.has('uploadMode')) mode = uploadMode.value
      if (uploadDirty.has('buildVersion')) {
        options.build_runtime = buildVersion.value.split(':')[0] || ''
        options.runtime_version = buildVersion.value.split(':')[1] || ''
      }
      if (uploadDirty.has('buildCommand')) options.build_command = buildCommand.value
      if (uploadDirty.has('streamMode') && streamMode.value !== '')
        options.stream = streamMode.value
      if (uploadDirty.has('thirdParty')) options.third_party = String(thirdParty.value)
      if (uploadDirty.has('allowedHosts')) options.allowed_hosts = allowedHosts.value
      await td.upload(zip.value, 'tools', mode, options)
      ElMessage.success('源码草稿已保存，请开始构建')
      uploadOpen.value = false
      await router.push('/tooldeck/my-tools')
    } finally {
      uploadBusy.value = false
    }
  }
  onMounted(async () => {
    await load()
    if (useUserStore().isLogin && route.query.upload === '1') uploadOpen.value = true
  })
</script>
<style scoped>
  .tool-page {
    padding: 12px 8px;
  }
  header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 30px;
    gap: 20px;
  }
  .eyebrow {
    font-size: 11px;
    letter-spacing: 2px;
    color: var(--el-color-primary);
    font-weight: 700;
  }
  h1 {
    font-size: 30px;
    font-weight: 700;
    margin: 8px 0;
  }
  header p,
  .intro {
    color: var(--el-text-color-secondary);
    margin: 8px 0 20px;
  }
  .filters {
    display: flex;
    gap: 12px;
    margin-bottom: 24px;
  }
  .cards {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 20px;
    min-height: 80px;
  }
  .tool-card {
    border-radius: 14px;
  }
  .card-top,
  .card-bottom {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .runtime {
    font-size: 12px;
    font-weight: 700;
    color: var(--el-color-primary);
    background: var(--el-color-primary-light-9);
    padding: 8px 10px;
    border-radius: 8px;
  }
  h2 {
    font-size: 19px;
    margin: 20px 0 10px;
    font-weight: 600;
  }
  .tool-card p {
    color: var(--el-text-color-secondary);
    min-height: 48px;
    line-height: 1.7;
  }
  .card-bottom {
    margin-top: 24px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
  }
  pre {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    padding: 18px;
    background: var(--el-fill-color-light);
    border-radius: 10px;
    margin: 18px 0;
  }
  @media (max-width: 600px) {
    header {
      align-items: flex-start;
      flex-direction: column;
    }
    .filters {
      flex-wrap: wrap;
    }
  }
</style>

<style scoped>
  .tool-page > header {
    padding: 32px;
    border-radius: 20px;
    background: linear-gradient(
      115deg,
      var(--el-color-primary-light-9),
      var(--el-bg-color) 65%,
      var(--el-color-success-light-9)
    );
    border: 1px solid #e4e9f6;
  }
  .tool-page > header h1 {
    color: #27324d;
    font-size: 32px;
    letter-spacing: -1px;
  }
  .tool-page > header p {
    color: #6a7690;
    margin-bottom: 0;
    line-height: 1.8;
  }
  .filters {
    padding: 16px;
    background: var(--el-bg-color);
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 14px;
  }
  .tool-card {
    border-radius: 18px;
    box-shadow: none;
    border: 1px solid var(--el-border-color-lighter);
    transition:
      transform 0.18s,
      box-shadow 0.18s;
  }
  .tool-card:hover {
    transform: translateY(-3px);
    box-shadow: 0 12px 28px #2639600c;
  }
  .tool-card :deep(.el-card__body) {
    padding: 24px;
  }
  .card-bottom {
    padding-top: 18px;
    border-top: 1px solid var(--el-border-color-lighter);
  }
  @media (max-width: 600px) {
    .tool-page > header {
      padding: 24px 20px;
    }
    .tool-page > header h1 {
      font-size: 26px;
    }
  }
</style>

<style scoped>
  .tool-card {
    border-radius: 16px;
    overflow: hidden;
  }
  .tool-card :deep(.el-card__body) {
    height: 100%;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    padding: 22px 24px;
  }
  .card-top {
    justify-content: flex-start;
    gap: 12px;
    min-height: 26px;
    flex-wrap: wrap;
  }
  .runtime {
    padding: 4px 8px;
    border-radius: 6px;
    font-size: 11px;
    line-height: 18px;
    letter-spacing: 0.3px;
  }
  .visibility {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 12px;
    color: var(--el-text-color-secondary);
    white-space: nowrap;
  }
  .visibility i {
    height: 5px;
    width: 5px;
    border-radius: 50%;
    background: currentColor;
  }
  .visibility.public {
    color: #368775;
  }
  .visibility.pending {
    color: var(--el-color-warning);
  }
  .visibility.rejected {
    color: var(--el-color-danger);
  }
  .execution-mode {
    margin-left: auto;
    font-size: 12px;
    color: var(--el-text-color-placeholder);
    white-space: nowrap;
  }
  .tool-card h2 {
    font-size: 18px;
    line-height: 1.5;
    margin: 18px 0 9px;
    letter-spacing: 0.1px;
    display: -webkit-box;
    -webkit-line-clamp: 2;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .tool-card .card-description {
    font-size: 14px;
    line-height: 1.8;
    margin: 0 0 22px;
    min-height: 76px;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .tool-card .review-note {
    min-height: 0;
    font-size: 12px;
    color: var(--el-color-danger);
    margin: 0 0 14px;
  }
  .build-status {
    align-self: flex-start;
    margin-bottom: 14px;
  }
  .card-bottom {
    margin-top: auto;
    padding-top: 16px;
    gap: 16px;
  }
  .version {
    font-size: 12px;
    color: var(--el-text-color-placeholder);
    font-variant-numeric: tabular-nums;
  }
  .card-bottom .el-button {
    height: 34px;
    padding: 0 13px;
    border-color: transparent;
    background: var(--el-color-primary-light-9);
  }
  .card-bottom .el-button:hover {
    border-color: var(--el-color-primary-light-5);
    background: var(--el-color-primary-light-8);
  }
  .run-arrow {
    margin-left: 10px;
    font-size: 16px;
  }
  @media (max-width: 600px) {
    .tool-card :deep(.el-card__body) {
      padding: 20px;
    }
    .tool-card .card-description {
      min-height: 0;
    }
  }
  .package-upload {
    width: 100%;
    margin-bottom: 20px;
  }
  .package-upload :deep(.el-upload),
  .package-upload :deep(.el-upload-dragger) {
    width: 100%;
  }
</style>
