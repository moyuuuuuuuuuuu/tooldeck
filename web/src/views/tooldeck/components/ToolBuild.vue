<template><section><p>上传后由作者主动构建。构建固定使用所选语言版本，依赖安装和构建不会注入业务环境变量。</p><ElTag>{{ labels[tool?.build_status||'legacy'] }}</ElTag><span v-if="tool" style="margin-left:12px">{{ tool.manifest.runtime }} {{ tool.manifest.runtime_version || '旧运行环境' }}</span><ElButton style="margin-left:12px" @click="load">刷新</ElButton><ElButton v-if="['pending','failed'].includes(tool?.build_status||'')" type="primary" :loading="retrying" @click="retry">{{tool?.build_status==='pending'?'开始构建':'重新构建'}}</ElButton><ElAlert v-if="tool?.build_error" :title="tool.build_error" type="error" :closable="false" style="margin-top:16px"/><pre>{{ tool?.build_log || (tool?.build_status==='pending'?'尚未开始构建。':'暂无日志；依赖安装结束后将显示构建日志。') }}</pre><p v-if="tool?.build_status==='ready'">产物已保存，可以提交审核。发布后的版本不重新构建；代码或依赖变更请上传新版本。</p></section></template>
<script setup lang="ts">
import {ref,onMounted,onBeforeUnmount} from 'vue'
import request from '@/utils/http'
import {td,type Tool} from '@/api/tooldeck'
const props=defineProps<{toolId:string}>();const emit=defineEmits(['updated']);const tool=ref<Tool>(),retrying=ref(false)
const labels:Record<string,string>={legacy:'旧版直接执行',pending:'待构建',queued:'等待构建',building:'正在构建',ready:'构建成功',failed:'构建失败'}
let timer:ReturnType<typeof setInterval>|undefined
async function load(){tool.value=await request.get<Tool>({url:`/v1/tools/${props.toolId}/build`});emit('updated',tool.value)}
async function retry(){retrying.value=true;try{await td.save(`tools/${props.toolId}/build`,{});await load()}finally{retrying.value=false}}
onMounted(()=>{load();timer=setInterval(()=>{if(['queued','building'].includes(tool.value?.build_status||''))load().catch(()=>{})},3000)})
onBeforeUnmount(()=>clearInterval(timer))
</script><style scoped>p{margin:16px 0;color:var(--el-text-color-secondary)}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--el-fill-color-light);padding:16px;max-height:440px;overflow:auto}</style>
