<template>
  <section>
    <header class="heading"><div><h1>我上传的工具</h1><p>管理每个版本的发布状态，查看审核反馈。</p></div><ElButton type="primary" @click="router.push('/tooldeck/tools?upload=1')">上传工具包</ElButton></header>
    <div class="filters"><ElInput v-model="search" placeholder="搜索工具名称或版本" clearable/><ElSelect v-model="filter"><ElOption label="全部状态" value=""/><ElOption v-for="(label,key) in labels" :key="key" :label="label" :value="key"/></ElSelect><ElButton :loading="loading" @click="load">刷新</ElButton></div>
    <ElTable v-loading="loading" :data="filtered" empty-text="暂无符合条件的工具">
      <ElTableColumn label="工具" min-width="240"><template #default="{row}"><strong>{{row.manifest.title}}</strong><div class="secondary">{{row.manifest.name}} · v{{row.manifest.version}}</div><div class="description">{{row.manifest.description}}</div></template></ElTableColumn>
      <ElTableColumn label="可见范围" width="100"><template #default="{row}"><ElTag type="info" effect="plain">{{row.public===false?'仅自己':'公开'}}</ElTag></template></ElTableColumn>
      <ElTableColumn label="当前状态" min-width="220"><template #default="{row}"><ElTag :type="status(row as Tool)==='approved'?'success':status(row as Tool)==='rejected'?'danger':status(row as Tool)==='pending'?'warning':'info'">{{labels[status(row as Tool)]}}</ElTag><p v-if="status(row as Tool)==='rejected'" class="reason">驳回原因：{{row.review_note || '审核人未填写原因，请联系站点管理员。'}}</p></template></ElTableColumn>
      <ElTableColumn label="构建" width="120"><template #default="{row}">{{buildLabels[row.build_status||''] || row.build_status}}</template></ElTableColumn>
      <ElTableColumn label="操作" min-width="260"><template #default="{row}"><ElButton link type="primary" @click="router.push('/tooldeck/tools?tool='+encodeURIComponent(row.id))">详情</ElButton><ElButton v-if="['pending','failed'].includes(row.build_status||'')" link type="primary" :loading="busy===row.id" @click="build(row as Tool)">{{row.build_status==='failed'?'重新构建':'开始构建'}}</ElButton><ElButton v-if="['draft','rejected'].includes(row.review_status||'')||row.withdrawn" link type="success" :disabled="busy===row.id || row.build_status!=='ready'" @click="change(row as Tool,'submit')">{{row.public===false?'发布':row.review_status==='rejected'?'重新提交':'提交审核'}}</ElButton><ElButton v-if="!row.withdrawn&&['pending','approved'].includes(row.review_status||'')" link type="danger" :disabled="busy===row.id" @click="change(row as Tool,'withdraw')">下架</ElButton></template></ElTableColumn>
    </ElTable>
    <p class="footnote">上传后先主动构建，构建成功才能提交。下架仅影响所选版本，保留代码、配置和历史记录；公开版本提交时按站点规则审核，私有工具提交后直接可用。</p>
  </section>
</template>
<script setup lang="ts">
import {computed,onMounted,ref} from 'vue'
import {useRouter} from 'vue-router'
import {ElMessage,ElMessageBox} from 'element-plus'
import {td,type Tool} from '@/api/tooldeck'
defineOptions({name:'MyTools'})
const router=useRouter(),tools=ref<Tool[]>([]),search=ref(''),filter=ref(''),loading=ref(false),busy=ref('')
const labels:Record<string,string>={draft:'待提交',pending:'审核中',rejected:'已驳回',approved:'已通过',withdrawn:'已下架'}
const buildLabels:Record<string,string>={'':'可运行',pending:'待构建',ready:'构建成功',failed:'构建失败',queued:'等待构建',building:'正在构建'}
const status=(t:Tool)=>t.withdrawn?'withdrawn':t.review_status||'approved'
const filtered=computed(()=>tools.value.filter(t=>(!filter.value||status(t)===filter.value)&&`${t.manifest.title} ${t.manifest.name} ${t.manifest.version}`.toLowerCase().includes(search.value.trim().toLowerCase())))
async function load(){loading.value=true;try{tools.value=await td.list('my-tools')}finally{loading.value=false}}
async function change(t:Tool,action:string){
 try{await ElMessageBox.confirm(action==='withdraw'?`下架「${t.manifest.title}」v${t.manifest.version}？其他用户将无法发起新的调用，已开始的任务会继续执行。`:'提交此版本？公开工具将按站点规则进入审核。','确认发布操作',{type:'warning',confirmButtonText:'确认',cancelButtonText:'取消'})}catch{return}
 busy.value=t.id;try{await td.save(`tools/${t.id}/publication`,{action});ElMessage.success('状态已更新');await load()}finally{busy.value=''}
}
async function build(t:Tool){busy.value=t.id;try{await td.save(`tools/${t.id}/build`,{});ElMessage.success('构建任务已提交');await load()}finally{busy.value=''}}
onMounted(load)
</script>
<style scoped>
.heading{display:flex;align-items:center;justify-content:space-between;gap:20px;margin-bottom:28px}h1{font-size:28px;margin:0 0 10px}.heading p,.secondary,.footnote{color:var(--el-text-color-secondary)}.filters{display:flex;gap:12px;margin-bottom:24px}.filters .el-input{max-width:360px}.filters .el-select{width:150px}.secondary{font-size:12px;margin-top:6px}.description{font-size:13px;margin-top:6px}.reason{color:var(--el-color-danger);white-space:pre-wrap;overflow-wrap:anywhere;font-size:13px;margin-bottom:0}.footnote{font-size:13px;line-height:1.8;margin-top:20px}@media(max-width:600px){.heading{align-items:flex-start}.filters{flex-wrap:wrap}}
</style>
