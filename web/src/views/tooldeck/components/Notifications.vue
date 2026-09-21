<template>
  <ElPopover trigger="click" width="380"
    ><template #reference
      ><ElBadge :value="items.length" :hidden="!items.length"
        ><ElButton text>通知</ElButton></ElBadge
      ></template
    >
    <h3>通知</h3
    ><div class="preferences"
      ><ElCheckbox v-model="prefs.in_app" @change="savePrefs">站内通知</ElCheckbox
      ><ElCheckbox v-model="prefs.email" @change="savePrefs">邮件通知</ElCheckbox></div
    >
    <ElEmpty v-if="!items.length" description="暂无新通知" :image-size="50" />
    <div class="notices"
      ><button v-for="item in items" :key="item.id" @click="open(item)"
        ><strong>{{ item.title }}</strong
        ><small>{{ new Date(item.created_at).toLocaleString() }} · 查看详情</small
        ><small v-if="item.email_status === 'failed'">邮件发送失败，请检查邮箱设置</small></button
      ></div
    >
  </ElPopover>
  <ElDialog v-model="visible" title="运行详情" width="min(760px,95vw)"
    ><RunResult :run="selected" @update="selected = $event"
  /></ElDialog>
</template>
<script setup lang="ts">
  import { ref, onMounted, onBeforeUnmount } from 'vue'
  import { useRouter } from 'vue-router'
  import { td, type Run } from '@/api/tooldeck'
  import RunResult from './RunResult.vue'
  const router = useRouter(),
    items = ref<any[]>([]),
    selected = ref<Run | null>(null),
    visible = ref(false),
    prefs = ref({ in_app: true, email: false })
  let timer: ReturnType<typeof setInterval> | undefined
  async function load() {
    items.value = await td.list('inbox')
  }
  async function savePrefs() {
    prefs.value = await td.save('inbox/preferences', prefs.value)
    await load()
  }
  async function open(item: any) {
    if (['run', 'callback'].includes(item.kind)) {
      selected.value = await td.run(item.object)
      visible.value = true
    } else {
      await router.push('/tooldeck/my-tools')
    }
    await td.save('inbox/' + item.id, {})
    await load()
  }
  onMounted(() => {
    load().catch(() => {})
    td.get<any>('inbox/preferences')
      .then((value) => (prefs.value = value))
      .catch(() => {})
    timer = setInterval(() => load().catch(() => {}), 10000)
  })
  onBeforeUnmount(() => clearInterval(timer))
</script>
<style scoped>
  h3 {
    font-weight: 600;
    margin-bottom: 12px;
  }
  .preferences {
    margin-bottom: 12px;
  }
  .notices {
    max-height: 380px;
    overflow: auto;
  }
  .notices button {
    display: flex;
    flex-direction: column;
    text-align: left;
    width: 100%;
    gap: 5px;
    border: 0;
    background: var(--el-fill-color-light);
    padding: 12px;
    margin: 8px 0;
    border-radius: 8px;
    cursor: pointer;
  }
  .notices small {
    color: var(--el-text-color-secondary);
  }
</style>
