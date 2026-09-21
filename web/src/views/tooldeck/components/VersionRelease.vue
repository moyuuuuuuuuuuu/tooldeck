<template>
  <ElDialog v-model="visible" title="版本发布与回滚" width="min(760px,95vw)" @open="load">
    <p>{{ tool.manifest.title }} · v{{ tool.manifest.version }}</p>
    <ElDescriptions :column="1" border
      ><ElDescriptionsItem label="当前默认">{{
        version(release.default_version)
      }}</ElDescriptionsItem
      ><ElDescriptionsItem label="可回滚版本">{{
        version(release.previous_version)
      }}</ElDescriptionsItem
      ><ElDescriptionsItem label="灰度版本"
        >{{ version(release.canary_version) }}
        {{ release.canary_percent ? release.canary_percent + '%' : '' }}</ElDescriptionsItem
      ></ElDescriptions
    >
    <div class="actions"
      ><ElButton :loading="busy" type="primary" @click="act('default')">将此版本设为默认</ElButton
      ><ElInputNumber v-model="percent" :min="1" :max="99" /><ElButton
        :loading="busy"
        @click="act('canary')"
        >灰度发布 {{ percent }}%</ElButton
      ><ElButton :disabled="!release.canary_version" @click="act('stop-canary')">停止灰度</ElButton
      ><ElButton type="warning" :disabled="!release.previous_version" @click="act('rollback')"
        >回滚默认版本</ElButton
      ></div
    >
    <p class="hint"
      >灰度按调用者固定分组。默认版本切换影响工具发现列表，已创建的任务和指定版本的 API
      调用保持原版本。</p
    >
    <ElSelect v-model="compare" placeholder="选择版本对比" clearable @change="load"
      ><ElOption
        v-for="item in siblings"
        :key="item.id"
        :value="item.id"
        :label="'v' + item.manifest.version"
    /></ElSelect>
    <div v-if="comparison" class="comparison"
      ><div
        ><h4>此版本</h4><pre>{{ JSON.stringify(tool.manifest, null, 2) }}</pre></div
      ><div
        ><h4>对比版本</h4><pre>{{ JSON.stringify(comparison, null, 2) }}</pre>
      </div></div
    >
  </ElDialog>
</template>
<script setup lang="ts">
  import { computed, ref, watch } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { td, type Tool } from '@/api/tooldeck'
  const visible = defineModel<boolean>({ default: false })
  const props = defineProps<{ tool: Tool; tools: Tool[] }>(),
    emit = defineEmits<{ updated: [] }>()
  const release = ref<any>({}),
    percent = ref(10),
    busy = ref(false),
    compare = ref(''),
    comparison = ref<any>(null)
  const siblings = computed(() =>
    props.tools.filter(
      (t) =>
        t.manifest.name === props.tool.manifest.name &&
        t.owner === props.tool.owner &&
        t.id !== props.tool.id
    )
  )
  const version = (id: string) =>
    props.tools.find((t) => t.id === id)?.manifest.version || id || '未设置'
  async function load() {
    const data = await td.get<any>(
      'tools/' +
        props.tool.id +
        '/release' +
        (compare.value ? '?compare=' + encodeURIComponent(compare.value) : '')
    )
    release.value = data.release
    comparison.value = data.compare || null
  }
  async function act(action: string) {
    try {
      await ElMessageBox.confirm('确认执行此版本发布操作？现有任务会继续使用原版本。', '版本发布')
    } catch {
      return
    }
    busy.value = true
    try {
      release.value = await td.save('tools/' + props.tool.id + '/release', {
        action,
        percent: percent.value
      })
      ElMessage.success('发布配置已更新')
      emit('updated')
    } finally {
      busy.value = false
    }
  }
  watch(
    () => props.tool.id,
    () => {
      compare.value = ''
      comparison.value = null
      if (visible.value) load()
    }
  )
</script>
<style scoped>
  .actions {
    display: flex;
    flex-wrap: wrap;
    gap: 12px;
    margin: 20px 0;
  }
  .hint {
    color: var(--el-text-color-secondary);
    margin-bottom: 20px;
  }
  .comparison {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 16px;
    margin-top: 20px;
  }
  .comparison > div {
    min-width: 0;
  }
  pre {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    background: var(--el-fill-color-light);
    padding: 12px;
    max-height: 400px;
    overflow: auto;
  }
  @media (max-width: 600px) {
    .comparison {
      grid-template-columns: 1fr;
    }
  }
</style>
