<template>
  <section class="donation-page" v-loading="loading">
    <header class="intro"
      ><span class="eyebrow">SUPPORT TOOLDECK</span><h1>让好用的工具，持续生长</h1
      ><p>{{
        settings.message ||
        '如果 ToolDeck 帮你节省了时间，欢迎自愿捐赠，支持服务器运行与后续维护。感谢每一份支持。'
      }}</p
      ><p class="hint">捐赠完全自愿，不影响工具使用，也不附带额外服务。</p></header
    >
    <ElAlert
      v-if="loadError"
      title="捐赠信息加载失败，请重试"
      type="error"
      :closable="false"
      show-icon
      ><ElButton text @click="load">重新加载</ElButton></ElAlert
    >
    <div class="payment-grid" v-if="settings.wechat_qr || settings.alipay_qr">
      <article
        v-for="channel in channels.filter((item) => settings[item.key])"
        :key="channel.key"
        class="payment-card"
      >
        <span class="payment-icon" :class="channel.key">{{ channel.short }}</span
        ><h2>{{ channel.label }}</h2
        ><p>使用{{ channel.label }}扫描下方收款码</p
        ><img
          :src="settings[channel.key]"
          :alt="`${channel.label}收款码`"
          class="qr-code"
          referrerpolicy="no-referrer"
        /><p class="hint">付款前请核对收款人信息与金额</p>
      </article>
    </div>
    <ElEmpty
      v-else-if="!loading && !loadError"
      description="收款码尚未配置，感谢你对 ToolDeck 的支持。"
    />
    <section v-if="admin && !loadError" class="settings">
      <h2>捐赠页设置</h2
      ><p class="hint"
        >微信、支付宝各保留一张收款码。选择新图后点击保存，替换当前图片；清除并保存后停止展示对应方式。</p
      >
      <ElForm label-position="top" :disabled="saving"
        ><ElFormItem label="页面说明"
          ><ElInput
            v-model="draft.message"
            type="textarea"
            :rows="3"
            maxlength="1000"
            show-word-limit
        /></ElFormItem>
        <div class="payment-grid"
          ><ElFormItem
            v-for="channel in channels"
            :key="channel.key"
            :label="`${channel.label}收款码`"
            ><div class="upload-field"
              ><img
                v-if="draft[channel.key]"
                :src="draft[channel.key]"
                :alt="`${channel.label}待保存收款码`"
                class="preview"
              /><input
                type="file"
                :disabled="saving || reading > 0"
                accept="image/png,image/jpeg,image/webp"
                :aria-label="`上传${channel.label}收款码`"
                @change="selectImage($event, channel.key)"
              /><span class="hint">PNG / JPEG / WebP，最大 512 KB</span
              ><ElButton
                v-if="draft[channel.key]"
                text
                type="danger"
                @click="draft[channel.key] = ''"
                >清除图片</ElButton
              ></div
            ></ElFormItem
          ></div
        >
        <ElButton type="primary" :loading="saving" :disabled="loading || reading > 0" @click="save"
          >保存设置</ElButton
        >
      </ElForm>
    </section>
  </section>
</template>
<script setup lang="ts">
  import { computed, onMounted, reactive, ref } from 'vue'
  import { ElMessage } from 'element-plus'
  import request from '@/utils/http'
  import { useUserStore } from '@/store/modules/user'
  defineOptions({ name: 'Donation' })
  type QRKey = 'wechat_qr' | 'alipay_qr'
  interface Settings {
    message: string
    wechat_qr: string
    alipay_qr: string
  }
  const channels: { key: QRKey; label: string; short: string }[] = [
    { key: 'wechat_qr', label: '微信', short: '微' },
    { key: 'alipay_qr', label: '支付宝', short: '支' }
  ]
  const admin = computed(() => useUserStore().info.roles?.includes('R_SUPER'))
  const settings = reactive<Settings>({ message: '', wechat_qr: '', alipay_qr: '' })
  const draft = reactive<Settings>({ ...settings })
  const loading = ref(true),
    saving = ref(false),
    loadError = ref(false),
    reading = ref(0)
  async function load() {
    loading.value = true
    loadError.value = false
    try {
      const data = await request.get<Settings>({ url: '/public/donation' })
      Object.assign(settings, data)
      Object.assign(draft, data)
    } catch {
      loadError.value = true
    } finally {
      loading.value = false
    }
  }
  async function selectImage(event: Event, key: QRKey) {
    const input = event.target as HTMLInputElement,
      file = input.files?.[0]
    input.value = ''
    if (!file) return
    if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type) || file.size > 512 * 1024) {
      ElMessage.error('请选择不超过 512 KB 的 PNG、JPEG 或 WebP 图片')
      return
    }
    reading.value++
    try {
      draft[key] = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader()
        reader.onload = () => resolve(String(reader.result))
        reader.onerror = reject
        reader.readAsDataURL(file)
      })
    } catch {
      ElMessage.error('图片读取失败，请重新选择')
    } finally {
      reading.value--
    }
  }
  async function save() {
    saving.value = true
    try {
      const data = await request.post<Settings>({
        url: '/v1/donation-settings',
        data: { ...draft }
      })
      Object.assign(settings, data)
      Object.assign(draft, data)
      ElMessage.success('捐赠设置已保存，旧收款码已替换')
    } finally {
      saving.value = false
    }
  }
  onMounted(load)
</script>
<style scoped>
  .donation-page {
    max-width: 960px;
    margin: 0 auto;
    padding: 36px 8px;
  }
  .intro {
    text-align: center;
    max-width: 680px;
    margin: 0 auto 36px;
  }
  .eyebrow {
    font-size: 12px;
    letter-spacing: 3px;
    color: var(--el-color-primary);
    font-weight: 700;
  }
  h1 {
    font-size: clamp(26px, 4vw, 38px);
    margin: 18px 0;
  }
  p {
    line-height: 1.9;
    white-space: pre-wrap;
    color: var(--el-text-color-regular);
  }
  .hint {
    font-size: 13px;
    color: var(--el-text-color-secondary);
  }
  .payment-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
    gap: 24px;
  }
  .payment-card {
    text-align: center;
    padding: 28px;
    border: 1px solid var(--el-border-color-light);
    border-radius: 18px;
    background: var(--el-bg-color);
  }
  .payment-icon {
    display: inline-grid;
    place-items: center;
    width: 44px;
    height: 44px;
    border-radius: 12px;
    font-size: 22px;
    color: white;
  }
  .wechat_qr {
    background: #16a45c;
  }
  .alipay_qr {
    background: #1677ff;
  }
  h2 {
    font-size: 20px;
    margin: 16px 0 8px;
  }
  .qr-code {
    display: block;
    margin-inline: auto;
    width: 100%;
    max-width: 260px;
    height: 280px;
    object-fit: contain;
    background: white;
    border-radius: 8px;
  }
  .settings {
    margin-top: 42px;
    padding-top: 24px;
    border-top: 1px solid var(--el-border-color);
  }
  .upload-field {
    display: flex;
    flex-direction: column;
    gap: 12px;
    max-width: 100%;
  }
  .preview {
    width: 150px;
    height: 170px;
    object-fit: contain;
    background: white;
  }
  input {
    max-width: 100%;
  }
</style>
