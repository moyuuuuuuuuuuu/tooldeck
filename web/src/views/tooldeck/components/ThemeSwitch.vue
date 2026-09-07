<template><div class="theme-toggle"><span aria-hidden="true" :class="{active:siteTheme==='light'}">☀</span><ElSwitch :model-value="siteTheme==='dark'" aria-label="深色模式" @change="value=>setSiteTheme(value?'dark':'light')"/><span aria-hidden="true" :class="{active:siteTheme==='dark'}">☾</span></div></template>
<script setup lang="ts">
import {siteTheme,setSiteTheme} from '@/hooks/core/useSiteTheme'
</script>
<style scoped>.theme-toggle{display:inline-flex;align-items:center;gap:8px;white-space:nowrap}.theme-toggle>span{font-size:18px;color:var(--el-text-color-placeholder);line-height:1}.theme-toggle>span.active{color:var(--el-color-primary)}.theme-toggle :deep(.el-switch){--el-switch-on-color:#5269ef;--el-switch-off-color:#b7c3dd}</style>
