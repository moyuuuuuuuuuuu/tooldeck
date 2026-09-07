<template>
 <div class="run-page" v-loading="loading">
  <template v-if="tool">
   <header><ElButton text @click="router.push('/tooldeck/tools')">← 返回工具列表</ElButton><div><span class="eyebrow">{{tool.manifest.runtime.toUpperCase()}} · v{{tool.manifest.version}}</span><h1>{{tool.manifest.title||tool.manifest.name}}</h1><p>{{tool.manifest.description}}</p></div></header>
   <main class="workspace">
    <section class="form-panel"><div class="panel-heading"><div><span>INPUT</span><h2>运行参数</h2></div><ElTag v-if="tool.manifest.execution.mode==='async'" type="info">异步执行</ElTag></div>
     <DynamicField v-for="([name,field]) in orderedFields" :key="tool.id+name" :label="String(name)" :schema="field" :ui="tool.manifest.ui_schema?.[name]||{}" :required="tool.manifest.input_schema.required?.includes(String(name))" v-model="input[name]" @uploading="uploading += $event ? 1 : -1"/>
     <ElButton class="run-button" type="primary" size="large" :loading="running" :disabled="uploading>0||pending||(!!tool.build_status&&tool.build_status!=='ready')" @click="execute">{{pending?'正在执行':'运行工具'}}</ElButton>
     <ElAlert v-if="tool.build_status&&tool.build_status!=='ready'" title="工具尚未构建成功，暂时不能运行。" type="warning" :closable="false"/>
     <ElCollapse v-if="owner" class="management"><ElCollapseItem title="构建与日志"><ToolBuild :tool-id="tool.id" @updated="tool=$event"/></ElCollapseItem><ElCollapseItem v-if="tool.manifest.env?.length" title="环境变量"><ToolEnvironment :tool-id="tool.id"/></ElCollapseItem><ElCollapseItem title="工具配置"><pre>{{JSON.stringify(tool.manifest,null,2)}}</pre></ElCollapseItem></ElCollapse>
    </section>
    <section class="result-panel"><div class="panel-heading"><div><span>OUTPUT</span><h2>运行结果</h2></div><ElButton v-if="result" text @click="refresh">刷新</ElButton></div><ElEmpty v-if="!result" description="填写左侧参数并运行，结果将在这里显示"/><RunResult v-else :run="result" @update="result=$event"/></section>
   </main>
  </template>
  <ElResult v-else-if="!loading" icon="warning" title="工具不可用" sub-title="工具不存在、尚未公开或当前账号无权访问"><template #extra><ElButton type="primary" @click="router.push('/tooldeck/tools')">返回工具列表</ElButton></template></ElResult>
 </div>
</template>
<script setup lang="ts">
import {computed,onBeforeUnmount,onMounted,ref} from 'vue'
import {useRoute,useRouter} from 'vue-router'
import {ElMessage} from 'element-plus'
import {useUserStore} from '@/store/modules/user'
import {td,type Field,type Run,type Tool} from '@/api/tooldeck'
import DynamicField from '../components/DynamicField.vue'
import RunResult from '../components/RunResult.vue'
import ToolBuild from '../components/ToolBuild.vue'
import ToolEnvironment from '../components/ToolEnvironment.vue'
defineOptions({name:'ToolRun'})
const route=useRoute(),router=useRouter(),user=useUserStore()
const tool=ref<Tool>(),input=ref<Record<string,any>>({}),result=ref<Run|null>(null),loading=ref(true),running=ref(false),uploading=ref(0)
const orderedFields=computed(()=>Object.entries(tool.value?.manifest.input_schema.properties||{}).sort(([a],[b])=>(tool.value?.manifest.ui_schema?.[a]?.order??0)-(tool.value?.manifest.ui_schema?.[b]?.order??0)))
const pending=computed(()=>['queued','running'].includes(result.value?.status||''))
const owner=computed(()=>user.info.roles?.includes('R_SUPER')||tool.value?.owner===String(user.info.id))
let timer:ReturnType<typeof setTimeout>|undefined
function defaults(s:Field):any{if(s.default!==undefined)return structuredClone(s.default);if(s.type==='object')return Object.fromEntries(Object.entries(s.properties||{}).map(([k,v])=>[k,defaults(v)]).filter(([,v])=>v!==undefined));if(s.type==='array')return [];if(s.type==='boolean')return false;return undefined}
function stopPoll(){if(timer)clearTimeout(timer);timer=undefined}
async function refresh(){if(result.value)result.value=await td.run(result.value.run_id)}
async function poll(){if(!result.value)return;try{await refresh();if(pending.value)timer=setTimeout(poll,1500)}catch{ElMessage.warning('状态刷新失败，请到使用记录查看；任务不会重复提交')}}
async function execute(){if(!tool.value)return;running.value=true;try{result.value=await td.execute(tool.value.id,input.value);if(pending.value)timer=setTimeout(poll,1000)}finally{running.value=false}}
onMounted(async()=>{try{const id=String(route.params.id||'');let list=await td.tools();tool.value=list.find(item=>item.id===id);if(!tool.value&&user.isLogin)tool.value=(await td.list('my-tools')).find(item=>item.id===id);if(tool.value)input.value=defaults(tool.value.manifest.input_schema)}finally{loading.value=false}})
onBeforeUnmount(stopPoll)
</script>
<style scoped>
.run-page{min-height:calc(100vh - 190px)}header{display:grid;gap:18px;margin-bottom:24px}header .el-button{justify-self:start;padding-left:0}.eyebrow,.panel-heading span{font-size:11px;letter-spacing:1.8px;color:var(--el-color-primary);font-weight:700}h1{font-size:30px;margin:8px 0}header p{margin:0;color:var(--el-text-color-secondary);line-height:1.7}.workspace{display:grid;grid-template-columns:minmax(360px,0.9fr) minmax(440px,1.1fr);gap:22px;align-items:start}.form-panel,.result-panel{background:var(--el-bg-color);border:1px solid var(--el-border-color-lighter);border-radius:18px;padding:24px;min-width:0}.result-panel{position:sticky;top:18px;min-height:420px}.panel-heading{display:flex;align-items:center;justify-content:space-between;gap:16px;margin-bottom:22px}.panel-heading h2{font-size:20px;margin:5px 0 0}.run-button{width:100%;margin-top:8px}.form-panel>.el-alert{margin-top:14px}.management{margin-top:24px}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--el-fill-color-light);padding:14px;border-radius:8px;max-height:360px;overflow:auto}@media(max-width:900px){.workspace{grid-template-columns:1fr}.result-panel{position:static;min-height:320px}}@media(max-width:600px){h1{font-size:25px}.form-panel,.result-panel{padding:18px;border-radius:14px}}
</style>
