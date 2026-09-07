<template><div><div class="heading"><h1>我的使用记录</h1><ElButton @click="load">刷新</ElButton></div><ElTable :data="runs" v-loading="loading" @row-click="select"><ElTableColumn prop="run_id" label="任务" min-width="240"/><ElTableColumn label="调用方式" width="130"><template #default="{row}"><ElTag effect="plain">{{sourceLabels[row.source]||'历史记录'}}</ElTag></template></ElTableColumn><ElTableColumn prop="status" label="状态" width="130"/><ElTableColumn prop="duration_ms" label="耗时 ms" width="130"/><ElTableColumn prop="created_at" label="创建时间" min-width="210"/><ElTableColumn label="操作" width="100"><template #default="{row}"><ElButton link type="primary" @click.stop="select(row as Run)">查看</ElButton></template></ElTableColumn></ElTable><div class="pagination"><ElPagination v-model:current-page="page" v-model:page-size="pageSize" :total="total" :page-sizes="[10,20,50,100]" layout="total, sizes, prev, pager, next" @change="load"/></div><ElDrawer v-model="open" title="任务详情" size="min(760px,95vw)"><ElAlert v-if="selected" :title="`${sourceLabels[selected.source||'']||'历史调用'}${selected.env_overridden?' · 本次使用了临时环境变量覆盖':''}`" type="info" :closable="false" style="margin-bottom:16px"/><RunResult :run="selected" @update="selected=$event"/><ElCollapse v-if="selected"><ElCollapseItem title="执行输入"><pre>{{ JSON.stringify(selected.input,null,2) }}</pre></ElCollapseItem></ElCollapse></ElDrawer></div></template>
<script setup lang="ts">
import {ref,onMounted,onBeforeUnmount} from 'vue'
import {td,type Run} from '@/api/tooldeck'
import RunResult from '../components/RunResult.vue'
defineOptions({name:'Runs'})
const runs=ref<Run[]>([]),loading=ref(false),selected=ref<Run|null>(null),open=ref(false),page=ref(1),pageSize=ref(20),total=ref(0)
const sourceLabels:Record<string,string>={web:'网页调用',api_key:'API Key',oauth:'OAuth',guest:'匿名调用'}
async function load(){loading.value=true;try{const data=await td.runs(page.value,pageSize.value);runs.value=data.items;total.value=data.total;if(selected.value)selected.value=runs.value.find(r=>r.run_id===selected.value?.run_id)||selected.value}finally{loading.value=false}}
function select(run:Run){selected.value=run;open.value=true}
let timer:ReturnType<typeof setInterval>;onMounted(()=>{load();timer=setInterval(()=>load().catch(()=>{}),5000)});onBeforeUnmount(()=>clearInterval(timer))
</script>
<style scoped>.heading{display:flex;justify-content:space-between;margin-bottom:24px}.pagination{display:flex;justify-content:flex-end;margin-top:20px}h1{font-size:26px;font-weight:600}pre{white-space:pre-wrap;overflow-wrap:anywhere}</style>
