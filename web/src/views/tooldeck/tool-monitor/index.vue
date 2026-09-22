<template>
  <section class="monitor">
    <header class="heading tooldeck-page-hero">
      <div><span class="page-eyebrow">ADMIN MONITOR</span><h1>工具监控</h1><p>查看所有工具版本与任务状态；仅管理员可访问。</p></div>
      <ElButton :loading="loading" @click="load">刷新</ElButton>
    </header>
    <ElAlert v-if="error" :title="error" type="error" :closable="false" />
    <div class="summary">
      <ElCard v-for="item in summary" :key="item.label"><span>{{ item.label }}</span><strong>{{ item.value }}</strong></ElCard>
    </div>
    <ElTabs v-model="tab" @tab-change="load">
      <ElTabPane label="工具概览" name="tools">
        <div class="filters"><ElInput v-model="search" clearable placeholder="搜索名称、作者或工具 ID" @keyup.enter="toolPage = 1; loadTools()" /><ElButton @click="toolPage = 1; loadTools()">搜索</ElButton></div>
        <ElTable :data="tools.items" v-loading="loading" empty-text="暂无工具">
          <ElTableColumn label="工具 / 版本" min-width="220"><template #default="{ row }"><strong>{{ row.tool.manifest.title }}</strong><div class="muted">{{ row.tool.manifest.name }} · {{ row.tool.manifest.version }}</div></template></ElTableColumn>
          <ElTableColumn label="作者" prop="tool.owner" min-width="120" />
          <ElTableColumn label="发布" width="125"><template #default="{ row }"><ElTag :type="row.tool.withdrawn ? 'info' : row.tool.default_version ? 'success' : 'warning'">{{ row.tool.withdrawn ? '已下架' : row.tool.default_version ? '默认版本' : row.tool.review_status || '未发布' }}</ElTag></template></ElTableColumn>
          <ElTableColumn label="构建" prop="tool.build_status" width="100" />
          <ElTableColumn label="运行总数" prop="total" width="100" />
          <ElTableColumn label="排队 / 运行" width="120"><template #default="{ row }">{{ row.statuses.queued || 0 }} / {{ (row.statuses.running || 0) + (row.statuses.canceling || 0) }}</template></ElTableColumn>
          <ElTableColumn label="成功 / 失败" width="120"><template #default="{ row }">{{ row.statuses.succeeded || 0 }} / {{ (row.statuses.failed || 0) + (row.statuses.timed_out || 0) }}</template></ElTableColumn>
          <ElTableColumn label="最近运行" width="175"><template #default="{ row }">{{ date(row.last_run_at) }}</template></ElTableColumn>
          <ElTableColumn label="操作" width="90"><template #default="{ row }"><ElButton link type="primary" @click="selectTool(row as ToolRow)">明细</ElButton></template></ElTableColumn>
        </ElTable>
        <ElPagination v-model:current-page="toolPage" :page-size="20" :total="tools.total" layout="total, prev, pager, next" @current-change="loadTools" />
      </ElTabPane>
      <ElTabPane label="运行状态" name="runs">
        <div class="filters"><ElSelect v-model="runTool" clearable filterable placeholder="全部工具版本" @change="runPage = 1; loadRuns()"><ElOption v-for="row in allTools" :key="row.id" :label="`${row.title} · ${row.version}`" :value="row.id" /></ElSelect><ElSelect v-model="runStatus" clearable placeholder="全部状态" @change="runPage = 1; loadRuns()"><ElOption v-for="status in statuses" :key="status" :label="status" :value="status" /></ElSelect></div>
        <ElTable :data="runs.items" v-loading="loading" empty-text="暂无运行记录">
          <ElTableColumn prop="run_id" label="任务 ID" min-width="250" /><ElTableColumn label="工具" min-width="170"><template #default="{ row }">{{ toolLabel(row.tool_id) }}</template></ElTableColumn>
          <ElTableColumn prop="owner" label="调用者" min-width="125" /><ElTableColumn prop="status" label="状态" width="110" /><ElTableColumn prop="duration_ms" label="耗时 ms" width="100" /><ElTableColumn label="创建时间" width="175"><template #default="{ row }">{{ date(row.created_at) }}</template></ElTableColumn>
          <ElTableColumn label="操作" width="150"><template #default="{ row }"><ElButton link type="primary" @click="selectRun(row.run_id)">查看</ElButton><ElButton v-if="active(row.status)" link type="danger" @click="terminate(row.run_id)">强制终止</ElButton></template></ElTableColumn>
        </ElTable>
        <ElPagination v-model:current-page="runPage" :page-size="20" :total="runs.total" layout="total, prev, pager, next" @current-change="loadRuns" />
      </ElTabPane>
    </ElTabs>
    <ElDrawer v-model="toolOpen" title="工具明细" size="min(720px, 96vw)">
      <template v-if="selectedTool"><ElDescriptions :column="1" border><ElDescriptionsItem label="名称">{{ selectedTool.tool.manifest.title }} · {{ selectedTool.tool.manifest.version }}</ElDescriptionsItem><ElDescriptionsItem label="工具 ID">{{ selectedTool.tool.id }}</ElDescriptionsItem><ElDescriptionsItem label="作者">{{ selectedTool.tool.owner }}</ElDescriptionsItem><ElDescriptionsItem label="运行环境">{{ selectedTool.tool.manifest.runtime }} {{ selectedTool.tool.manifest.runtime_version }}</ElDescriptionsItem><ElDescriptionsItem label="执行模式">{{ selectedTool.tool.manifest.execution.mode }}</ElDescriptionsItem><ElDescriptionsItem label="超时 / 内存">{{ selectedTool.tool.manifest.execution.timeout_seconds }} 秒 / {{ selectedTool.tool.manifest.execution.memory_mb }} MiB</ElDescriptionsItem><ElDescriptionsItem label="API 调用">{{ selectedTool.tool.api_enabled === false ? '未开放' : '开放' }}</ElDescriptionsItem><ElDescriptionsItem label="可见范围">{{ selectedTool.tool.public === false ? '私有' : '公开' }}</ElDescriptionsItem><ElDescriptionsItem label="构建状态">{{ selectedTool.tool.build_status }}</ElDescriptionsItem><ElDescriptionsItem label="审核状态">{{ selectedTool.tool.review_status }}</ElDescriptionsItem><ElDescriptionsItem label="发布状态">{{ selectedTool.tool.withdrawn ? '已下架' : selectedTool.tool.default_version ? '默认版本' : '非默认版本' }}</ElDescriptionsItem><ElDescriptionsItem label="灰度比例">{{ selectedTool.tool.canary_percent || 0 }}%</ElDescriptionsItem><ElDescriptionsItem label="总运行数">{{ selectedTool.total }}</ElDescriptionsItem><ElDescriptionsItem label="最近运行">{{ date(selectedTool.last_run_at) }}</ElDescriptionsItem></ElDescriptions><p class="description">{{ selectedTool.tool.manifest.description }}</p><ElButton type="primary" @click="runTool = selectedTool.tool.id; runPage = 1; tab = 'runs'; toolOpen = false; loadRuns()">查看该工具运行记录</ElButton></template>
    </ElDrawer>
    <ElDrawer v-model="runOpen" title="运行明细" size="min(760px, 96vw)"><RunResult :run="selectedRun" @update="selectedRun = $event" /><ElCollapse v-if="selectedRun"><ElCollapseItem title="执行输入"><pre>{{ JSON.stringify(selectedRun.input, null, 2) }}</pre></ElCollapseItem></ElCollapse></ElDrawer>
  </section>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { td, type Run, type Tool } from '@/api/tooldeck'
import RunResult from '../components/RunResult.vue'
defineOptions({ name: 'ToolMonitor' })
type ToolRow = {tool: Tool; statuses: Record<string, number>; total: number; last_run_at?: string}
type RunRow = {run_id: string; tool_id: string; owner: string; status: string; created_at: string; duration_ms: number}
type Summary = {versions: number; published: number; queued: number; running: number; tools: {id: string; title: string; version: string}[]}
type Page<T> = {items: T[]; total: number}
const tab = ref('tools'), loading = ref(false), error = ref(''), search = ref(''), toolPage = ref(1), runPage = ref(1), runTool = ref(''), runStatus = ref('')
const tools = ref<Page<ToolRow>>({items: [], total: 0}), allTools = ref<Summary['tools']>([]), overview = ref<Summary>({versions: 0, published: 0, queued: 0, running: 0, tools: []}), runs = ref<Page<RunRow>>({items: [], total: 0})
const toolOpen = ref(false), runOpen = ref(false), selectedTool = ref<ToolRow | null>(null), selectedRun = ref<Run | null>(null)
const statuses = ['queued', 'running', 'canceling', 'succeeded', 'failed', 'timed_out', 'canceled', 'cancel_failed']
const date = (value?: string) => value ? new Date(value).toLocaleString() : '—'
const active = (status: string) => ['queued', 'running', 'canceling'].includes(status)
const toolLabel = (id: string) => {const item = allTools.value.find(row => row.id === id); return item ? `${item.title} · ${item.version}` : id}
const summary = computed(() => {
  const data = overview.value
  return [
    {label: '工具版本', value: data.versions},
    {label: '已发布', value: data.published},
    {label: '排队任务', value: data.queued},
    {label: '执行中', value: data.running}
  ]
})
async function loadTools() { const query = new URLSearchParams({page: String(toolPage.value), page_size: '20', search: search.value}); tools.value = await td.get<Page<ToolRow>>('admin/tools?' + query) }
async function loadRuns() { const query = new URLSearchParams({page: String(runPage.value), page_size: '20'}); if (runTool.value) query.set('tool_id', runTool.value); if (runStatus.value) query.set('status', runStatus.value); runs.value = await td.get<Page<RunRow>>('admin/runs?' + query) }
async function load() { loading.value = true; error.value = ''; try { overview.value = await td.get<Summary>('admin/tools/summary'); allTools.value = overview.value.tools; if (tab.value === 'runs') await loadRuns(); else await loadTools() } catch (e) { error.value = e instanceof Error ? e.message : '加载失败' } finally { loading.value = false } }
function selectTool(row: ToolRow) { selectedTool.value = row; toolOpen.value = true }
async function selectRun(id: string) { selectedRun.value = await td.run(id); runOpen.value = true }
async function terminate(id: string) { try { await ElMessageBox.confirm(`强制终止任务 ${id}？将直接停止其容器，不执行工具取消钩子。`, '强制终止', {type: 'warning'}) } catch { return } try { await td.save(`admin/runs/${encodeURIComponent(id)}/terminate`, {}); ElMessage.success('终止命令已提交'); await load() } catch (e) { ElMessage.error(e instanceof Error ? e.message : '终止失败'); await load() } }
let timer: ReturnType<typeof setInterval>
onMounted(() => { load(); timer = setInterval(() => { if (tab.value === 'runs') load().catch(() => {}) }, 5000) })
onBeforeUnmount(() => clearInterval(timer))
</script>
<style scoped>
.heading{display:flex;justify-content:space-between;align-items:center;margin-bottom:22px}.summary{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:12px;margin-bottom:20px}.summary :deep(.el-card__body){display:flex;flex-direction:column;gap:8px}.summary strong{font-size:24px}.summary span,.muted{color:var(--el-text-color-secondary)}.filters{display:flex;gap:12px;margin-bottom:16px}.filters .el-input,.filters .el-select{max-width:320px}.el-pagination{justify-content:flex-end;margin-top:18px}.description{line-height:1.7;white-space:pre-wrap}pre{white-space:pre-wrap;overflow-wrap:anywhere}@media(max-width:800px){.summary{grid-template-columns:repeat(2,minmax(0,1fr))}.filters{flex-wrap:wrap}}
</style>
