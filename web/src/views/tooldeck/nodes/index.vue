<template>
  <section>
    <header class="heading"
      ><div><h1>运行与治理</h1><p>查看执行容量、运行质量与存储使用情况。</p></div
      ><ElButton :loading="loading" @click="load">刷新</ElButton></header
    >
    <ElAlert v-if="error" :title="error" type="error" :closable="false" />
    <div class="stats">
      <div v-for="stat in stats" :key="stat.label"
        ><small>{{ stat.label }}</small
        ><strong>{{ stat.value }}</strong></div
      >
    </div>
    <ElTabs>
      <ElTabPane label="执行节点">
        <ElTable :data="nodes"
          ><ElTableColumn prop="name" label="节点" /><ElTableColumn label="Docker"
            ><template #default="{ row }"
              ><ElTag :type="row.online ? 'success' : 'danger'">{{
                row.online ? '在线' : '不可用'
              }}</ElTag></template
            ></ElTableColumn
          ><ElTableColumn prop="concurrency" label="运行并发" /><ElTableColumn
            prop="user_concurrency"
            label="单用户并发" /><ElTableColumn prop="build_concurrency" label="构建并发"
        /></ElTable>
        <p class="muted"
          >平均排队 {{ metrics.mean_wait_ms || 0 }} ms · 最长当前等待
          {{ metrics.oldest_wait_ms || 0 }} ms · 构建排队 {{ metrics.build_queued || 0 }}</p
        >
        <p class="muted"
          >调度限额阻挡次数：全局 {{ metrics.limit_hits?.global || 0 }} / 用户
          {{ metrics.limit_hits?.user || 0 }} / 工具
          {{ metrics.limit_hits?.tool || 0 }}（自服务启动累计）</p
        >
      </ElTabPane>
      <ElTabPane label="审计日志">
        <ElTable :data="audit.items"
          ><ElTableColumn label="时间" min-width="180"
            ><template #default="{ row }">{{
              new Date(row.created_at).toLocaleString()
            }}</template></ElTableColumn
          ><ElTableColumn prop="actor" label="操作者" /><ElTableColumn
            prop="action"
            label="操作"
            min-width="190" /><ElTableColumn
            prop="object"
            label="对象"
            min-width="160" /><ElTableColumn prop="ip" label="来源 IP" /><ElTableColumn
            prop="status"
            label="结果"
            width="80"
        /></ElTable>
        <ElPagination
          v-model:current-page="page"
          :page-size="50"
          :total="audit.total"
          layout="prev,pager,next"
          @current-change="loadAudit"
        />
      </ElTabPane>
      <ElTabPane label="存储治理">
        <p
          >本地目录 {{ bytes(storage.local_bytes) }} · 配额计入
          {{ bytes(storage.retained_bytes) }}（工具包、构建产物和文件，含对象存储）</p
        >
        <ElForm label-position="top" class="quotas"
          ><ElFormItem
            v-for="field in quotaFields"
            :key="field.key"
            :label="field.label + '（MiB，0 为不限）'"
            ><ElInputNumber v-model="quotaMB[field.key]" :min="0" :precision="0" /></ElFormItem
          ><ElButton @click="saveQuotas">保存配额</ElButton></ElForm
        >
        <ElAlert
          title="只清理超过 24 小时且未被引用的目录或文件。所选内容移入 quarantine，可恢复；移入后不会释放磁盘空间。"
          type="info"
          :closable="false"
        />
        <ElTable :data="storage.items" @selection-change="selected = $event"
          ><ElTableColumn
            type="selection"
            :selectable="(row: any) => row.candidate"
          /><ElTableColumn prop="path" label="路径" min-width="260" /><ElTableColumn label="大小"
            ><template #default="{ row }">{{ bytes(row.bytes) }}</template></ElTableColumn
          ><ElTableColumn label="状态"
            ><template #default="{ row }">{{
              row.candidate ? '可清理' : '保留'
            }}</template></ElTableColumn
          ></ElTable
        >
        <ElButton type="warning" :disabled="!selected.length" @click="cleanup"
          >移入可恢复目录（{{ selected.length }}）</ElButton
        >
      </ElTabPane>
      <ElTabPane label="Docker 资源">
        <ElButton :loading="dockerLoading" @click="loadDocker">扫描本实例资源</ElButton>
        <ElAlert v-if="dockerError" :title="dockerError" type="warning" :closable="false" />
        <p class="muted"
          >只显示本实例创建并标记的容器和运行镜像。超过 24
          小时、无任务引用的停止容器及闲置镜像可以清理。</p
        >
        <ElTable :data="dockerItems"
          ><ElTableColumn prop="kind" label="类型" width="110" /><ElTableColumn
            prop="name"
            label="资源"
            min-width="240"
          /><ElTableColumn label="操作" width="120"
            ><template #default="{ row }"
              ><ElButton :disabled="!row.candidate" type="danger" link @click="cleanDocker(row)"
                >清理</ElButton
              ></template
            ></ElTableColumn
          ></ElTable
        >
      </ElTabPane>
    </ElTabs>
  </section>
</template>
<script setup lang="ts">
  import { ref, computed, onMounted } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { td } from '@/api/tooldeck'
  defineOptions({ name: 'Nodes' })
  const nodes = ref<any[]>([]),
    metrics = ref<any>({}),
    audit = ref<any>({ items: [], total: 0 }),
    storage = ref<any>({ items: [] }),
    selected = ref<any[]>([])
  const dockerItems = ref<any[]>([]),
    dockerLoading = ref(false),
    dockerError = ref('')
  const loading = ref(false),
    error = ref(''),
    page = ref(1)
  const quotaFields = [
    { key: 'site_bytes', label: '站点配额' },
    { key: 'user_bytes', label: '每用户配额' },
    { key: 'tool_bytes', label: '每工具配额' }
  ]
  const quotaMB = ref<Record<string, number>>({ site_bytes: 0, user_bytes: 0, tool_bytes: 0 })
  const bytes = (n = 0) => (n >= 1048576 ? (n / 1048576).toFixed(1) + ' MiB' : n + ' B')
  const stats = computed(() => [
    { label: '磁盘可用', value: bytes(metrics.value.disk_free_bytes) },
    { label: '排队任务', value: metrics.value.queued || 0 },
    { label: '运行任务', value: metrics.value.running || 0 },
    { label: '成功率', value: ((metrics.value.success_rate || 0) * 100).toFixed(1) + '%' },
    { label: 'P95 耗时', value: (metrics.value.p95_duration_ms || 0) + ' ms' },
    {
      label: '构建失败率',
      value: ((metrics.value.build_failure_rate || 0) * 100).toFixed(1) + '%'
    },
    {
      label: '回调失败率',
      value: ((metrics.value.callback_failure_rate || 0) * 100).toFixed(1) + '%'
    }
  ])
  async function loadAudit() {
    audit.value = await td.get<any>('audit?page=' + page.value)
  }
  async function load() {
    loading.value = true
    error.value = ''
    try {
      const [n, m, st] = await Promise.all([
        td.list('nodes'),
        td.get<any>('metrics'),
        td.get<any>('storage')
      ])
      nodes.value = n
      metrics.value = m
      storage.value = st
      selected.value = []
      for (const f of quotaFields)
        quotaMB.value[f.key] = Math.floor((st.quotas[f.key] || 0) / 1048576)
      await loadAudit()
    } catch (e) {
      error.value = e instanceof Error ? e.message : '加载失败，请重试'
    } finally {
      loading.value = false
    }
  }
  async function saveQuotas() {
    const q: Record<string, number> = {}
    for (const f of quotaFields) q[f.key] = (quotaMB.value[f.key] || 0) * 1048576
    await td.save('storage/quotas', q)
    ElMessage.success('配额已保存')
    await load()
  }
  async function cleanup() {
    try {
      await ElMessageBox.confirm('将所选未引用资源移入可恢复目录？', '清理预览', {
        type: 'warning'
      })
    } catch {
      return
    }
    await td.save('storage/cleanup', {
      preview_token: storage.value.preview_token,
      paths: selected.value.map((x) => x.path)
    })
    ElMessage.success('资源已移入可恢复目录')
    await load()
  }
  async function loadDocker() {
    dockerLoading.value = true
    dockerError.value = ''
    try {
      dockerItems.value = await td.list('docker-resources')
    } catch (e) {
      dockerError.value = e instanceof Error ? e.message : 'Docker 不可用，请检查节点连接'
    } finally {
      dockerLoading.value = false
    }
  }
  async function cleanDocker(item: any) {
    try {
      await ElMessageBox.confirm('永久清理此未引用的 Docker 资源？', '确认清理', {
        type: 'warning'
      })
    } catch {
      return
    }
    await td.save('docker-resources', { id: item.id, kind: item.kind })
    ElMessage.success('资源已清理')
    await loadDocker()
  }
  onMounted(load)
</script>
<style scoped>
  .heading {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-bottom: 24px;
  }
  .heading h1 {
    font-size: 26px;
    font-weight: 600;
  }
  .heading p,
  .muted {
    color: var(--el-text-color-secondary);
    margin: 12px 0;
  }
  .stats {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 16px;
    margin: 24px 0;
  }
  .stats > div {
    padding: 20px;
    background: var(--el-fill-color-light);
    border-radius: 12px;
  }
  .stats small,
  .stats strong {
    display: block;
  }
  .stats strong {
    font-size: 24px;
    margin-top: 8px;
  }
  .quotas {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 20px;
    margin: 20px 0;
  }
  .el-pagination {
    margin-top: 20px;
  }
  @media (max-width: 700px) {
    .stats {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
