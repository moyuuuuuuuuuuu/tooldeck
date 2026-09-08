<template>
  <div class="public-site"
    ><header
      ><RouterLink class="brand" to="/explore">ToolDeck</RouterLink
      ><nav
        ><ElButton text @click="show()">发现工具</ElButton
        ><ElButton text @click="show('guide')">开发指引</ElButton
        ><ElButton text @click="show('api')">API 接入</ElButton
        ><ElButton text @click="login('/tooldeck/playground')">在线运行</ElButton></nav
      ><div class="header-actions"
        ><ThemeSwitch /><ElButton @click="login(route.fullPath)">登录</ElButton
        ><ElButton type="primary" @click="$router.push('/auth/register')">注册</ElButton></div
      ></header
    ><main
      ><template v-if="route.query.tool"><Run /></template
      ><template v-else-if="route.query.view === 'guide'"><Guide /></template
      ><template v-else-if="route.query.view === 'api'"><ApiGuide /></template
      ><template v-else
        ><p class="notice"
          >公开工具均可浏览；无需环境变量的工具可以直接运行，需要个人配置的工具会在使用时引导登录。</p
        ><Tools /></template></main
  ></div>
</template>
<script setup lang="ts">
  import { useRoute, useRouter } from 'vue-router'
  import Tools from '../tools/index.vue'
  import Run from '../run/index.vue'
  import Guide from '../guide/index.vue'
  import ApiGuide from '../api-guide/index.vue'
  import ThemeSwitch from '../components/ThemeSwitch.vue'
  const route = useRoute(),
    router = useRouter()
  function show(view?: string) {
    router.push({ path: '/explore', query: view ? { view } : {} })
  }
  function login(redirect: string) {
    router.push({ path: '/auth/login', query: { redirect } })
  }
</script>
<style scoped>
  .header-actions,
  nav {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .public-site {
    min-height: 100vh;
    background: var(--el-bg-color);
  }
  header {
    max-width: 1240px;
    margin: auto;
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 22px 32px;
    border-bottom: 1px solid var(--el-border-color-lighter);
  }
  .brand {
    font-size: 23px;
    font-weight: 800;
    color: #5269ef;
    text-decoration: none;
  }
  main {
    max-width: 1240px;
    margin: auto;
    padding: 24px;
  }
  .notice {
    font-size: 13px;
    color: var(--el-text-color-secondary);
    line-height: 1.8;
    margin: 0 8px 8px;
  }
  @media (max-width: 800px) {
    nav {
      display: none;
    }
  }
  @media (max-width: 600px) {
    main {
      padding: 16px 8px;
    }
  }
</style>
