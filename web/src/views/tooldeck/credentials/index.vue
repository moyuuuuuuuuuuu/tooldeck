<template>
  <div
    ><header class="tooldeck-page-hero"><div><span class="page-eyebrow">ACCESS CREDENTIALS</span><h1>访问凭证</h1><p class="intro">创建和管理 API Key，在其他应用中调用工具。调用结果与使用次数会同步到你的账号。</p></div></header
    ><ElTabs
      ><ElTabPane label="API Key"
        ><ElButton type="primary" @click="keyOpen = true">创建 API Key</ElButton
        ><ElTable :data="keys"
          ><ElTableColumn prop="name" label="名称" /><ElTableColumn label="允许调用的工具"
            ><template #default="{ row }">{{ row.tools.includes('*') ? '所有工具（含后续新增）' : row.tools.join('、') }}</template></ElTableColumn
          ><ElTableColumn label="有效期"
            ><template #default="{ row }">{{ row.never_expires ? '长期有效' : new Date(row.expires_at).toLocaleString() }}</template></ElTableColumn
          ><ElTableColumn label="操作"
            ><template #default="{ row }"><ElButton text type="danger" @click="remove('keys', row.id)">撤销</ElButton></template></ElTableColumn
          ></ElTable
        ></ElTabPane
      ><ElTabPane v-if="isAdmin()" label="第三方密钥"
        ><ElButton type="primary" @click="secretOpen = true">添加／更新密钥</ElButton
        ><ElTable :data="secrets"
          ><ElTableColumn prop="name" label="环境变量名称" /><ElTableColumn prop="tools" label="获准使用的工具" /><ElTableColumn label="操作"
            ><template #default="{ row }"><ElButton text type="danger" @click="remove('secrets', row.name)">移除</ElButton></template></ElTableColumn
          ></ElTable
        ></ElTabPane
      ><ElTabPane label="OAuth"><ElAlert title="支持接入外部 OAuth 授权服务" type="info" :closable="false" /><p class="intro">在部署环境配置 introspection 地址、客户端凭证和 audience。调用方携带 Bearer 访问令牌，权限使用 tool:工具名，例如 tool:ai-image。平台验证令牌有效期、受众和工具范围。</p></ElTabPane></ElTabs
    >
    <ElDialog v-model="keyOpen" title="创建 API Key" width="480px"
      ><ElForm label-position="top"
        ><ElFormItem label="名称"><ElInput v-model="keyForm.name" /></ElFormItem
        ><ElFormItem label="允许调用的工具"
          ><ElSwitch v-model="keyForm.allTools" active-text="所有工具通用（含后续新增）" /><ElSelect v-if="!keyForm.allTools" v-model="keyForm.tools" multiple style="width: 100%"><ElOption v-for="t in names" :key="t" :value="t" :label="t" /></ElSelect></ElFormItem
        ><ElFormItem label="有效期"><ElSwitch v-model="keyForm.never_expires" active-text="长期有效（撤销前持续有效）" /><ElInputNumber v-if="!keyForm.never_expires" v-model="keyForm.days" :min="1" :max="365" /></ElFormItem></ElForm
      ><template #footer><ElButton type="primary" :loading="saving" @click="createKey">创建</ElButton></template></ElDialog
    >
    <ElDialog v-model="reveal" title="请保存 API Key" width="560px" @closed="newKey = ''"
      ><ElAlert title="完整密钥仅在创建时显示，请保存到你的密码管理器。" type="warning" :closable="false" /><ElInput :model-value="newKey" readonly style="margin-top: 18px" /><template #footer><ElButton @click="copyKey">复制</ElButton><ElButton type="primary" @click="reveal = false">已保存</ElButton></template></ElDialog
    >
    <ElDialog v-model="secretOpen" title="第三方密钥" width="480px" @closed="secretForm.value = ''"
      ><ElForm label-position="top"
        ><ElFormItem label="环境变量名称，例如 IMAGE_API_KEY"><ElInput v-model="secretForm.name" /></ElFormItem><ElFormItem label="密钥值"><ElInput v-model="secretForm.value" type="password" show-password /></ElFormItem
        ><ElFormItem label="获准使用的工具"
          ><ElSelect v-model="secretForm.tools" multiple style="width: 100%"><ElOption v-for="t in names" :key="t" :value="t" :label="t" /></ElSelect></ElFormItem></ElForm
      ><template #footer><ElButton type="primary" :loading="saving" @click="saveSecret">加密保存</ElButton></template></ElDialog
    ></div
  >
</template>
<script setup lang="ts">
  import { useUserStore } from '@/store/modules/user'
  const isAdmin = () => useUserStore().info.roles?.includes('R_SUPER')

  import { ref, reactive, onMounted } from 'vue'
  import { ElMessage, ElMessageBox } from 'element-plus'
  import { td } from '@/api/tooldeck'
  defineOptions({ name: 'Credentials' })
  const keys = ref<any[]>([]),
    secrets = ref<any[]>([]),
    names = ref<string[]>([]),
    keyOpen = ref(false),
    secretOpen = ref(false),
    reveal = ref(false),
    newKey = ref(''),
    saving = ref(false)
  const keyForm = reactive({
      name: '',
      tools: [] as string[],
      days: 30,
      allTools: false,
      never_expires: false
    }),
    secretForm = reactive({ name: '', value: '', tools: [] as string[] })
  async function load() {
    ;[keys.value, secrets.value] = await Promise.all([td.list('keys'), isAdmin() ? td.list('secrets') : Promise.resolve([])])
    names.value = [...new Set((await td.tools()).map((t) => t.manifest.name))]
  }
  async function createKey() {
    saving.value = true
    try {
      newKey.value = (
        await td.save('keys', {
          name: keyForm.name,
          tools: keyForm.allTools ? ['*'] : keyForm.tools,
          days: keyForm.days,
          never_expires: keyForm.never_expires
        })
      ).key
      keyOpen.value = false
      reveal.value = true
      await load()
    } finally {
      saving.value = false
    }
  }
  async function saveSecret() {
    saving.value = true
    try {
      await td.save('secrets', secretForm)
      secretOpen.value = false
      secretForm.value = ''
      ElMessage.success('已保存')
      await load()
    } finally {
      saving.value = false
    }
  }
  async function remove(kind: string, id: string) {
    await ElMessageBox.confirm('移除后相关调用将失去此凭证，是否继续？', '确认移除')
    await td.remove(kind, id)
    await load()
  }
  async function copyKey() {
    try {
      await navigator.clipboard.writeText(newKey.value)
      ElMessage.success('已复制')
    } catch {
      ElMessage.info('请手动选择并复制密钥')
    }
  }
  onMounted(load)
</script>
<style scoped>
  h1 {
    font-size: 26px;
    font-weight: 600;
  }
  .intro {
    margin: 16px 0 24px;
    color: var(--el-text-color-secondary);
    line-height: 1.8;
  }
  .el-table {
    margin-top: 20px;
  }
</style>
