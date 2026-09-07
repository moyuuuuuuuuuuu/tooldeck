<template>
 <div class="tool-page">
  <header><div><span class="eyebrow">YOUR PERSONAL TOOLKIT</span><h1>找到工具，让想法即刻发生</h1><p>写下提示词、上传素材，或输入你的问题。剩下的交给工具。</p></div><div v-if="useUserStore().isLogin" class="actions"><ElButton size="large" @click="$router.push('/tooldeck/guide')">工具包开发指引</ElButton><ElButton type="primary" size="large" @click="uploadOpen=true">＋ 上传工具包</ElButton></div></header>
  <div class="filters"><ElInput v-model="search" placeholder="搜索工具名称或用途" clearable style="max-width:340px"/><ElSelect v-if="isAdmin()" v-model="runtime" clearable placeholder="所有运行环境" style="width:170px"><ElOption v-for="r in ['php','js','node','python','go']" :key="r" :label="r" :value="r"/></ElSelect><ElButton @click="load">刷新</ElButton></div>
  <ElEmpty v-if="!loading&&!filtered.length" description="暂时没有可用工具，请稍后再来看看。"/>
  <div v-loading="loading" class="cards">
   <ElCard v-for="tool in filtered" :key="tool.id" shadow="never" class="tool-card">
    <div class="card-top">
     <span class="runtime">{{ isAdmin() ? tool.manifest.runtime.toUpperCase() : '在线工具' }}</span>
     <span class="visibility" :class="tool.withdrawn?'muted':tool.review_status==='rejected'?'rejected':tool.review_status==='pending'?'pending':tool.public===false?'muted':'public'"><i/>{{tool.withdrawn?'已下架':tool.public===false?'私有':tool.review_status==='pending'?'审核中':tool.review_status==='rejected'?'已驳回':'公开'}}</span>
     <span class="execution-mode">{{tool.manifest.execution.stream?'流式输出':tool.manifest.execution.mode==='async'?'后台处理':'即时返回'}}</span>
    </div>
    <h2 :title="tool.manifest.title || tool.manifest.name">{{ tool.manifest.title || tool.manifest.name }}</h2>
    <p class="card-description" :title="tool.manifest.description">{{ tool.manifest.description || '暂无说明' }}</p>
    <p v-if="tool.review_status==='rejected' && !tool.withdrawn" class="review-note">审核意见：{{ tool.review_note || '请调整后上传新版本' }}</p>
    <ElTag v-if="tool.build_status && tool.build_status!=='ready'" class="build-status" size="small" :type="tool.build_status==='failed'?'danger':'warning'">{{ buildLabels[tool.build_status] || tool.build_status }}</ElTag>
    <div class="card-bottom"><span class="version">v{{ tool.manifest.version }}</span><ElButton type="primary" plain @click="select(tool)">运行工具 <span class="run-arrow" aria-hidden="true">→</span></ElButton></div>
   </ElCard>
  </div>
  <ElDrawer v-model="drawer" :title="selected?.manifest.title || selected?.manifest.name" size="min(760px, 95vw)" destroy-on-close @closed="stopPoll">
   <template v-if="selected"><p class="intro">{{ selected.manifest.description }}</p><ElTabs v-model="tab"><ElTabPane label="在线运行" name="run">
    <DynamicField v-for="([name,field]) in orderedFields" :key="selected.id+name" :label="String(name)" :schema="field" :ui="selected.manifest.ui_schema?.[name] || {}" :required="selected.manifest.input_schema.required?.includes(String(name))" v-model="input[name]" @uploading="uploading += $event ? 1 : -1"/>
    <ElButton type="primary" size="large" :loading="running" :disabled="uploading>0 || pending || (!!selected.build_status && selected.build_status!=='ready')" @click="execute">{{ pending?'正在执行':'运行工具' }}</ElButton>
    <RunResult :run="result" @update="result=$event"/>
   </ElTabPane><ElTabPane v-if="isAdmin()||selected.owner===String(useUserStore().info.id)" label="构建与日志" name="build" lazy><ToolBuild :key="selected.id" :tool-id="selected.id" @updated="selected=$event"/></ElTabPane><ElTabPane v-if="selected.manifest.env?.length" label="环境变量" name="env" lazy><ToolEnvironment :key="selected.id" :tool-id="selected.id"/></ElTabPane><ElTabPane v-if="useUserStore().isLogin && selected.api_enabled!==false" label="API 调用" name="api"><p>先在“API 接入”创建允许调用此工具的 API Key。示例参数使用当前表单内容。</p><ElTabs><ElTabPane label="cURL"><pre>{{ apiExample }}</pre></ElTabPane><ElTabPane v-for="(code,language) in examples" :key="language" :label="String(language)"><pre>{{ code }}</pre></ElTabPane></ElTabs><p>HTTP 200/202 表示请求已接受。queued/running 时使用同一凭证查询 GET /api/v1/runs/{run_id}，直到 succeeded、failed、timed_out 或 canceled；结果见 data.result 与 data.artifacts。</p><p>文件字段先通过 POST /api/v1/files 上传，再传入返回的 file_id。</p></ElTabPane><ElTabPane v-if="isAdmin()" label="工具配置" name="config"><pre>{{ JSON.stringify(selected.manifest,null,2) }}</pre></ElTabPane></ElTabs></template>
  </ElDrawer>
  <ElDialog v-model="uploadOpen" title="上传工具包" width="520px"><p class="intro">ZIP 根目录包含 tooldeck.json 和入口代码。公开工具审核通过后对所有登录用户可见，私有工具无需审核。</p><ElFormItem label="构建环境版本"><ElSelect v-model="buildVersion" placeholder="使用代码包声明或平台默认版本" clearable style="width:100%"><ElOptionGroup v-for="(versions,language) in versionOptions" :key="language" :label="String(language)"><ElOption v-for="v in versions" :key="language+v" :value="String(language)+':'+v" :label="language+' '+v"/></ElOptionGroup></ElSelect></ElFormItem><ElFormItem label="构建命令（可选）"><ElInput v-model="buildCommand" placeholder="例如 npm run build，留空使用包内声明" maxlength="512"/></ElFormItem><p class="intro">不必上传 node_modules 或 vendor。依赖清单需包含锁文件；构建成功后才能使用或审核。</p><div style="margin-bottom:12px"><ElCheckbox v-model="isPublic">公开工具（按站点设置审核）</ElCheckbox></div><ElCheckbox v-model="thirdParty">使用第三方服务</ElCheckbox><ElInput v-if="thirdParty" v-model="allowedHosts" placeholder="允许访问的域名，逗号分隔；留空使用代码包声明" style="margin:12px 0"/><div><ElCheckbox v-model="notifyResult">异步通知结果（站内 + 邮件）</ElCheckbox></div><p v-if="notifyResult" class="intro">开启后后台执行，完成时站内通知，并向已验证邮箱发送完成提醒。需站点已配置邮件服务。</p><ElFormItem label="SSE 流式输出"><ElSelect v-model="streamMode" style="width:100%"><ElOption value="" label="跟随工具包声明（未声明则关闭）"/><ElOption value="true" label="开启 SSE 流式输出"/><ElOption value="false" label="关闭 SSE 流式输出"/></ElSelect></ElFormItem><p v-if="streamMode==='true'" class="intro">工具需按协议发送增量事件；网页实时显示，API 可订阅 SSE。与完成通知独立。</p><div style="margin-bottom:18px"><ElCheckbox v-model="apiEnabled">允许 API 调用</ElCheckbox></div><ElSelect v-model="uploadMode" :disabled="notifyResult" style="width:100%;margin-bottom:18px"><ElOption value="" label="使用代码包声明的运行模式"/><ElOption value="sync" label="同步返回结果"/><ElOption value="async" label="异步执行，查询结果"/></ElSelect><ElUpload drag accept=".zip" :auto-upload="false" :limit="1" :on-change="f=>zip=f.raw" :on-remove="()=>zip=undefined"><p>将 ZIP 拖到这里，或点击选择</p><small>最大 64 MB</small></ElUpload><template #footer><ElButton @click="uploadOpen=false">取消</ElButton><ElButton type="primary" :loading="uploadBusy" :disabled="!zip" @click="publish">上传并注册</ElButton></template></ElDialog>
 </div>
</template>
<script setup lang="ts">
import {useUserStore} from '@/store/modules/user'
const isAdmin = () => useUserStore().info.roles?.includes('R_SUPER')

import {ref,computed,onMounted,onBeforeUnmount} from 'vue'
import {useRoute} from 'vue-router'
const route=useRoute()
import {ElMessage} from 'element-plus'
import {td,type Tool,type Run,type Field} from '@/api/tooldeck'
import DynamicField from '../components/DynamicField.vue'
import ToolEnvironment from '../components/ToolEnvironment.vue'
import ToolBuild from '../components/ToolBuild.vue'
import RunResult from '../components/RunResult.vue'
import {apiExamples} from '../components/apiExamples'
defineOptions({name:'Tools'})
const tools=ref<Tool[]>([]),loading=ref(false),search=ref(''),runtime=ref(''),drawer=ref(false),selected=ref<Tool>(),tab=ref('run'),input=ref<Record<string,any>>({}),result=ref<Run|null>(null),running=ref(false),uploading=ref(0)
const buildLabels:Record<string,string>={queued:'等待构建',building:'正在构建',failed:'构建失败'}
const buildVersion=ref(''),buildCommand=ref('')
const versionOptions:Record<string,string[]>={php:['8.0','8.1','8.2','8.3'],node:['20','21','22','23'],python:['3.10','3.11','3.12'],go:['1.22','1.23','1.24']}
const isPublic=ref(true)
const streamMode=ref('')
const thirdParty=ref(false),allowedHosts=ref(''),notifyResult=ref(false),apiEnabled=ref(true)
const examples=computed(()=>apiExamples(location.origin,selected.value?.id||'TOOL_ID',input.value))
const uploadOpen=ref(false),uploadMode=ref(''),zip=ref<File>(),uploadBusy=ref(false)
const filtered=computed(()=>tools.value.filter(t=>(!runtime.value||t.manifest.runtime===runtime.value)&&`${t.manifest.name} ${t.manifest.title} ${t.manifest.description}`.toLowerCase().includes(search.value.toLowerCase())))
const orderedFields=computed(()=>Object.entries(selected.value?.manifest.input_schema.properties||{}).sort(([a],[b])=>(selected.value?.manifest.ui_schema?.[a]?.order??0)-(selected.value?.manifest.ui_schema?.[b]?.order??0)))
const pending=computed(()=>['queued','running'].includes(result.value?.status||''))
const apiExample=computed(()=>`curl -X POST '${location.origin}/api/v1/tools/${selected.value?.id}/runs' \\\n  -H 'X-API-Key: <your-key>' \\\n  -H 'Content-Type: application/json' \\\n  -d '${JSON.stringify({input:input.value},null,2)}'`)
let timer:ReturnType<typeof setTimeout>|undefined
function stopPoll(){if(timer)clearTimeout(timer);timer=undefined}
async function poll(){if(!result.value||!drawer.value)return;try{result.value=await td.run(result.value.run_id);if(pending.value)timer=setTimeout(poll,1500)}catch{ElMessage.warning('状态刷新失败，请到执行记录查看；任务不会被重复提交')}}
async function load(){loading.value=true;try{tools.value=await td.tools()}finally{loading.value=false}}
function defaults(s:Field):any{if(s.default!==undefined)return structuredClone(s.default);if(s.type==='object')return Object.fromEntries(Object.entries(s.properties||{}).map(([k,v])=>[k,defaults(v)]).filter(([,v])=>v!==undefined));if(s.type==='array')return [];if(s.type==='boolean')return false;return undefined}
function select(tool:Tool){stopPoll();selected.value=tool;input.value=defaults(tool.manifest.input_schema);result.value=null;tab.value='run';uploading.value=0;drawer.value=true}
async function execute(){if(!selected.value)return;running.value=true;try{result.value=await td.execute(selected.value.id,input.value);if(pending.value)timer=setTimeout(poll,1000)}finally{running.value=false}}
async function publish(){if(!zip.value)return;uploadBusy.value=true;try{await td.upload(zip.value,'tools',uploadMode.value,{build_runtime:buildVersion.value.split(':')[0]||'',runtime_version:buildVersion.value.split(':')[1]||'',build_command:buildCommand.value,stream:streamMode.value,public:String(isPublic.value),third_party:String(thirdParty.value),notify_result:String(notifyResult.value),api_enabled:String(apiEnabled.value),allowed_hosts:allowedHosts.value});ElMessage.success('源码已上传，平台正在准备构建');uploadOpen.value=false;await load()}finally{uploadBusy.value=false}}
onMounted(async()=>{await load();if(useUserStore().isLogin && route.query.upload==='1')uploadOpen.value=true;let target=tools.value.find(t=>t.id===route.query.tool);if(!target && route.query.tool && useUserStore().isLogin)target=(await td.list('my-tools')).find(t=>t.id===route.query.tool);if(target)select(target)});onBeforeUnmount(stopPoll)
</script>
<style scoped>
.tool-page{padding:12px 8px}header{display:flex;justify-content:space-between;align-items:center;margin-bottom:30px;gap:20px}.eyebrow{font-size:11px;letter-spacing:2px;color:var(--el-color-primary);font-weight:700}h1{font-size:30px;font-weight:700;margin:8px 0}header p,.intro{color:var(--el-text-color-secondary);margin:8px 0 20px}.filters{display:flex;gap:12px;margin-bottom:24px}.cards{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:20px;min-height:80px}.tool-card{border-radius:14px}.card-top,.card-bottom{display:flex;justify-content:space-between;align-items:center}.runtime{font-size:12px;font-weight:700;color:var(--el-color-primary);background:var(--el-color-primary-light-9);padding:8px 10px;border-radius:8px}h2{font-size:19px;margin:20px 0 10px;font-weight:600}.tool-card p{color:var(--el-text-color-secondary);min-height:48px;line-height:1.7}.card-bottom{margin-top:24px;font-size:12px;color:var(--el-text-color-secondary)}pre{white-space:pre-wrap;overflow-wrap:anywhere;padding:18px;background:var(--el-fill-color-light);border-radius:10px;margin:18px 0}@media(max-width:600px){header{align-items:flex-start;flex-direction:column}.filters{flex-wrap:wrap}}
</style>

<style scoped>.tool-page>header{padding:32px;border-radius:20px;background:linear-gradient(115deg,#edf0ff,#f6f8ff 65%,#eaf7f4);border:1px solid #e4e9f6}.tool-page>header h1{color:#27324d;font-size:32px;letter-spacing:-1px}.tool-page>header p{color:#6a7690;margin-bottom:0;line-height:1.8}.filters{padding:16px;background:var(--el-bg-color);border:1px solid var(--el-border-color-lighter);border-radius:14px}.tool-card{border-radius:18px;box-shadow:none;border:1px solid var(--el-border-color-lighter);transition:transform .18s,box-shadow .18s}.tool-card:hover{transform:translateY(-3px);box-shadow:0 12px 28px #2639600c}.tool-card :deep(.el-card__body){padding:24px}.card-bottom{padding-top:18px;border-top:1px solid var(--el-border-color-lighter)}@media(max-width:600px){.tool-page>header{padding:24px 20px}.tool-page>header h1{font-size:26px}}</style>

<style scoped>
.tool-card{border-radius:16px;overflow:hidden}.tool-card :deep(.el-card__body){height:100%;box-sizing:border-box;display:flex;flex-direction:column;padding:22px 24px}.card-top{justify-content:flex-start;gap:12px;min-height:26px;flex-wrap:wrap}.runtime{padding:4px 8px;border-radius:6px;font-size:11px;line-height:18px;letter-spacing:.3px}.visibility{display:inline-flex;align-items:center;gap:5px;font-size:12px;color:var(--el-text-color-secondary);white-space:nowrap}.visibility i{height:5px;width:5px;border-radius:50%;background:currentColor}.visibility.public{color:#368775}.visibility.pending{color:var(--el-color-warning)}.visibility.rejected{color:var(--el-color-danger)}.execution-mode{margin-left:auto;font-size:12px;color:var(--el-text-color-placeholder);white-space:nowrap}.tool-card h2{font-size:18px;line-height:1.5;margin:18px 0 9px;letter-spacing:.1px;display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}.tool-card .card-description{font-size:14px;line-height:1.8;margin:0 0 22px;min-height:76px;display:-webkit-box;-webkit-line-clamp:3;-webkit-box-orient:vertical;overflow:hidden}.tool-card .review-note{min-height:0;font-size:12px;color:var(--el-color-danger);margin:0 0 14px}.build-status{align-self:flex-start;margin-bottom:14px}.card-bottom{margin-top:auto;padding-top:16px;gap:16px}.version{font-size:12px;color:var(--el-text-color-placeholder);font-variant-numeric:tabular-nums}.card-bottom .el-button{height:34px;padding:0 13px;border-color:transparent;background:var(--el-color-primary-light-9)}.card-bottom .el-button:hover{border-color:var(--el-color-primary-light-5);background:var(--el-color-primary-light-8)}.run-arrow{margin-left:10px;font-size:16px}@media(max-width:600px){.tool-card :deep(.el-card__body){padding:20px}.tool-card .card-description{min-height:0}}
</style>
