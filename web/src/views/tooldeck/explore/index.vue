<template><div class="public-site"><header><RouterLink class="brand" to="/explore">ToolDeck</RouterLink><div class="header-actions"><ThemeSwitch/><ElButton @click="$router.push('/auth/login')">登录</ElButton><ElButton type="primary" @click="$router.push('/auth/register')">注册</ElButton></div></header><main><p class="notice">以下工具无需登录即可使用。登录后可使用需要个人配置的工具，并管理自己的工具与记录。</p><Tools/></main></div></template>
<script setup lang="ts">
import Tools from '../tools/index.vue'
import ThemeSwitch from '../components/ThemeSwitch.vue'
</script>
<style scoped>.header-actions{display:flex;align-items:center;gap:10px}.public-site{min-height:100vh;background:var(--el-bg-color)}header{max-width:1240px;margin:auto;display:flex;align-items:center;justify-content:space-between;padding:22px 32px;border-bottom:1px solid var(--el-border-color-lighter)}.brand{font-size:23px;font-weight:800;color:#5269ef;text-decoration:none}main{max-width:1240px;margin:auto;padding:24px}.notice{font-size:13px;color:var(--el-text-color-secondary);line-height:1.8;margin:0 8px 8px}@media(max-width:600px){main{padding:16px 8px}}</style>
