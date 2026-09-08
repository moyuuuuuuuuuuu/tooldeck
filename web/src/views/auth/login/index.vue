<template>
  <div class="login"
    ><div class="auth-theme"><ThemeSwitch /></div
    ><ElCard class="login-card"
      ><span class="brand">ToolDeck</span><h1>欢迎回来</h1><p>你的工具、工作流与每一次灵感。</p
      ><ElForm @submit.prevent="login" label-position="top"
        ><ElFormItem label="邮箱账号"
          ><ElInput
            v-model="username"
            autocomplete="username"
            placeholder="请输入邮箱或账号" /></ElFormItem
        ><ElFormItem label="密码"
          ><ElInput
            v-model="password"
            type="password"
            show-password
            autocomplete="current-password"
            placeholder="请输入登录密码"
            @keyup.enter="login" /></ElFormItem
        ><ElButton type="primary" size="large" :loading="busy" @click="login" style="width: 100%"
          >登录 ToolDeck</ElButton
        ></ElForm
      ><ElButton
        link
        type="primary"
        @click="router.push('/auth/forget-password')"
        style="margin-top: 20px"
        >忘记密码？</ElButton
      ><ElButton link type="primary" @click="router.push('/auth/register')" style="margin-top: 20px"
        >还没有账号？注册</ElButton
      ></ElCard
    ></div
  >
</template>
<script setup lang="ts">
  import ThemeSwitch from '../../tooldeck/components/ThemeSwitch.vue'
  import { ref } from 'vue'
  import { useRouter } from 'vue-router'
  import { useUserStore } from '@/store/modules/user'
  import { fetchLogin, fetchGetUserInfo } from '@/api/auth'
  const username = ref(''),
    password = ref(''),
    busy = ref(false),
    router = useRouter(),
    user = useUserStore()
  async function login() {
    if (busy.value) return
    busy.value = true
    try {
      const r = await fetchLogin({
        username: username.value,
        password: password.value,
        code: '',
        uuid: ''
      })
      user.setToken(r.access_token)
      user.setUserInfo(await fetchGetUserInfo())
      user.setLoginStatus(true)
      await router.push('/tooldeck/tools')
    } finally {
      busy.value = false
    }
  }
</script>
<style scoped>
  .auth-theme {
    position: absolute;
    right: 24px;
    top: 24px;
  }
  .login {
    min-height: 100vh;
    display: grid;
    place-items: center;
    background: radial-gradient(ellipse at 20% 20%, #e5efff, transparent 60%), #f6f8fc;
    padding: 24px;
  }
  .login-card {
    width: min(420px, 100%);
    padding: 24px;
    border-radius: 18px;
  }
  .brand {
    font-size: 18px;
    font-weight: 800;
    color: #4b6ff0;
    letter-spacing: 1px;
  }
  h1 {
    font-size: 28px;
    margin-top: 30px;
    font-weight: 700;
  }
  p {
    color: #8991a1;
    margin: 10px 0 30px;
  }
</style>
