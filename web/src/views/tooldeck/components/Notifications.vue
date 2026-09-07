<template><ElPopover trigger="click" width="340"><template #reference><ElBadge :value="items.length" :hidden="!items.length"><ElButton text>通知</ElButton></ElBadge></template><h3>工具执行通知</h3><ElEmpty v-if="!items.length" description="暂无新通知" :image-size="50"/><div class="notices"><button v-for="item in items" :key="item.run_id" @click="open(item)"><strong>{{ item.status==='succeeded'?'工具已完成':'工具执行已结束' }}</strong><span>{{ item.run_id }}</span><small>{{ new Date(item.created_at).toLocaleString() }} · 点击查看结果</small></button></div></ElPopover><ElDialog v-model="visible" title="执行结果" width="min(760px,95vw)"><RunResult :run="selected" @update="selected=$event"/></ElDialog></template>
<script setup lang="ts">
import {ref,onMounted,onBeforeUnmount} from 'vue'
import {td,type Run} from '@/api/tooldeck'
import RunResult from './RunResult.vue'
const items=ref<Run[]>([]),selected=ref<Run|null>(null),visible=ref(false)
let timer:ReturnType<typeof setInterval>|undefined
async function load(){items.value=await td.list('notifications')}
async function open(run:Run){selected.value=run;visible.value=true;await td.save('notifications/'+run.run_id,{});await load()}
onMounted(()=>{load().catch(()=>{});timer=setInterval(()=>load().catch(()=>{}),10000)})
onBeforeUnmount(()=>clearInterval(timer))
</script><style scoped>h3{font-weight:600;margin-bottom:12px}.notices{max-height:380px;overflow:auto}.notices button{display:flex;flex-direction:column;text-align:left;width:100%;gap:5px;border:0;background:var(--el-fill-color-light);padding:12px;margin:8px 0;border-radius:8px;cursor:pointer}.notices span{font-size:11px;overflow-wrap:anywhere}.notices small{color:var(--el-text-color-secondary)}</style>
