<template><ElAvatar :size="size" :src="user.info.avatar || undefined" class="letter-avatar" :aria-label="'账号头像 '+letter">{{ letter }}</ElAvatar></template>
<script setup lang="ts">
import {computed} from 'vue'
import {useUserStore} from '@/store/modules/user'
withDefaults(defineProps<{size?:number}>(),{size:34})
const user=useUserStore()
const letter=computed(()=>Array.from((user.info.email||user.info.username||'?').trim())[0]?.toLocaleUpperCase()||'?')
</script><style scoped>.letter-avatar{background:#5269ef;color:white;font-weight:700;flex-shrink:0}</style>
