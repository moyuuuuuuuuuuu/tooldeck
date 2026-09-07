<template>
 <section v-if="run" class="result">
  <div class="result-heading"><h3>执行结果</h3><ElTag :type="run.status==='succeeded'?'success':run.status==='failed'?'danger':'info'">{{ labels[run.status] || run.status }}</ElTag><span>{{ run.duration_ms }} ms</span><ElButton v-if="pending" text type="danger" @click="cancel">取消执行</ElButton></div>
  <p class="hint">{{ run.run_id }}</p><ElProgress v-if="pending" :percentage="50" :indeterminate="true" :show-text="false"/>
  <ElAlert v-if="run.error" :title="run.error" type="error" :closable="false"/>
  <div v-if="images.length" class="gallery"><div v-for="image in images" :key="image.id"><ElImage :src="image.url" :preview-src-list="images.map(x=>x.url)" fit="contain"/><p>{{ image.name }}</p></div></div>
  <div v-for="file in run.artifacts || []" :key="file.file_id"><ElButton text @click="download(file)">下载 {{ file.name }}（{{ Math.ceil(file.size/1024) }} KB）</ElButton></div>
  <pre v-if="run.result !== null">{{ typeof run.result==='string'?run.result:JSON.stringify(run.result,null,2) }}</pre>
  <ElCollapse v-if="run.logs"><ElCollapseItem title="执行日志"><pre>{{ run.logs }}</pre></ElCollapseItem></ElCollapse>
 </section>
</template>
<script setup lang="ts">
import {ref,computed,watch,onBeforeUnmount} from 'vue'
import {td,type Run,type Artifact} from '@/api/tooldeck'
const props=defineProps<{run:Run|null}>();const emit=defineEmits(['update'])
const pending=computed(()=>['queued','running'].includes(props.run?.status||''))
const labels:Record<string,string>={queued:'排队中',running:'执行中',succeeded:'已完成',failed:'失败',timed_out:'已超时',canceled:'已取消'}
const images=ref<{id:string;url:string;name:string}[]>([]);let generation=0
function clear(){images.value.forEach(x=>URL.revokeObjectURL(x.url));images.value=[]}
watch(()=>props.run?.artifacts,async(files)=>{const gen=++generation;clear();for(const f of files||[]){if(['image/png','image/jpeg','image/webp','image/gif'].includes(f.mime)){try{const blob=await td.blob(f.file_id);if(gen!==generation)return;images.value.push({id:f.file_id,url:URL.createObjectURL(blob),name:f.name})}catch{}}}},{immediate:true})
async function download(f:Artifact){const blob=await td.blob(f.file_id);const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download=f.name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}
async function cancel(){if(props.run)emit('update',await td.cancel(props.run.run_id))}
onBeforeUnmount(()=>{generation++;clear()})
</script>
<style scoped>
.result{margin-top:24px;border-top:1px solid var(--el-border-color);padding-top:20px}.result-heading{display:flex;align-items:center;gap:12px}.hint{font-size:12px;color:var(--el-text-color-secondary);margin:8px 0}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--el-fill-color-light);padding:16px;border-radius:8px;max-height:400px;overflow:auto;margin-top:12px}.gallery{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px;margin-top:18px}.gallery .el-image{width:100%;height:240px}
</style>
