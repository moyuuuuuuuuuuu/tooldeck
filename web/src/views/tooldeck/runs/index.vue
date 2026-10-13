<template>
  <div
    ><div class="heading tooldeck-page-hero"
      ><div
        ><span class="page-eyebrow">RUN HISTORY</span><h1>运行记录</h1
        ><p>查看网页、API 与 MCP 调用产生的任务状态和运行结果。</p></div
      ><ElButton @click="load">刷新</ElButton></div
    ><ElSelect
      v-model="statusFilter"
      clearable
      placeholder="全部状态"
      @change="resetFilter"
      style="margin-bottom: 16px"
      ><ElOption
        v-for="(label, value) in statusLabels"
        :key="value"
        :label="label"
        :value="value" /></ElSelect
    ><ElTable :data="runs" v-loading="loading" @row-click="select"
      ><ElTableColumn label="工具 / 任务" min-width="240"
        ><template #default="{ row }"
          ><strong>{{ row.tool_title || row.tool_id }}</strong
          ><div>{{ row.run_id }}</div></template
        ></ElTableColumn
      ><ElTableColumn label="调用方式" width="130"
        ><template #default="{ row }"
          ><ElTag effect="plain">{{ sourceLabels[row.source] || '历史记录' }}</ElTag></template
        ></ElTableColumn
      ><ElTableColumn label="状态" width="130"
        ><template #default="{ row }">{{
          statusLabels[row.status] || row.status
        }}</template></ElTableColumn
      ><ElTableColumn prop="duration_ms" label="耗时 ms" width="130" /><ElTableColumn
        label="创建时间"
        min-width="210"
        ><template #default="{ row }">{{
          new Date(row.created_at).toLocaleString()
        }}</template></ElTableColumn
      ><ElTableColumn label="操作" width="100"
        ><template #default="{ row }"
          ><ElButton link type="primary" @click.stop="select(row as Run)">查看</ElButton></template
        ></ElTableColumn
      ></ElTable
    ><div class="pagination"
      ><ElPagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="total"
        :page-sizes="[10, 20, 50, 100]"
        layout="total, sizes, prev, pager, next"
        @change="load" /></div
    ><ElDrawer v-model="open" title="任务详情" size="min(760px,95vw)"
      ><ElAlert
        v-if="selected"
        :title="`${sourceLabels[selected.source || ''] || '历史调用'}${selected.env_overridden ? ' · 本次使用了临时环境变量覆盖' : ''}`"
        type="info"
        :closable="false"
        style="margin-bottom: 16px"
      /><RunResult :run="selected" @update="selected = $event" /><ElCollapse v-if="selected"
        ><ElCollapseItem title="执行输入">
          <pre>{{ JSON.stringify(selected.input, null, 2) }}</pre>
        </ElCollapseItem></ElCollapse
      ></ElDrawer
    ></div
  >
</template>
<script setup lang="ts">
  import { ref, onMounted, onBeforeUnmount } from 'vue'
  import { td, type Run } from '@/api/tooldeck'
  import RunResult from '../components/RunResult.vue'
  defineOptions({ name: 'Runs' })
  const runs = ref<Run[]>([]),
    loading = ref(false),
    selected = ref<Run | null>(null),
    open = ref(false),
    page = ref(1),
    pageSize = ref(20),
    total = ref(0)
  const statusFilter = ref('')
  const statusLabels: Record<string, string> = {
    queued: '排队中',
    running: '执行中',
    canceling: '正在取消',
    succeeded: '已完成',
    failed: '失败',
    timed_out: '已超时',
    canceled: '已取消',
    cancel_failed: '取消失败'
  }
  const sourceLabels: Record<string, string> = {
    web: '网页调用',
    api_key: 'API Key',
    oauth: 'OAuth',
    mcp: 'MCP',
    guest: '匿名调用'
  }
  let loadGeneration = 0
  async function load() {
    const generation = ++loadGeneration
    loading.value = true
    try {
      const data = await td.runs(page.value, pageSize.value, statusFilter.value)
      if (generation !== loadGeneration) return
      runs.value = data.items
      total.value = data.total
      if (selected.value)
        selected.value =
          runs.value.find((r) => r.run_id === selected.value?.run_id) || selected.value
    } finally {
      if (generation === loadGeneration) loading.value = false
    }
  }
  function resetFilter() {
    page.value = 1
    void load()
  }
  function select(run: Run) {
    selected.value = run
    open.value = true
  }
  let timer: ReturnType<typeof setInterval>
  onMounted(() => {
    load()
    timer = setInterval(() => load().catch(() => {}), 5000)
  })
  onBeforeUnmount(() => clearInterval(timer))
</script>
<style scoped>
  .heading {
    display: flex;
    justify-content: space-between;
    margin-bottom: 24px;
  }
  .pagination {
    display: flex;
    justify-content: flex-end;
    margin-top: 20px;
  }
  h1 {
    font-size: 26px;
    font-weight: 600;
  }
  pre {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
</style>
