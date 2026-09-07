<template>
 <section v-if="run" class="result">
  <div class="result-heading"><h3>执行结果</h3><ElTag :type="run.status==='succeeded'?'success':run.status==='failed'?'danger':'info'">{{ labels[run.status] || run.status }}</ElTag><span>{{ run.duration_ms }} ms</span><ElButton v-if="pending" text type="danger" @click="cancel">取消执行</ElButton></div>
  <p class="hint">{{ run.run_id }}</p><ElProgress v-if="pending" :percentage="50" :indeterminate="true" :show-text="false"/>
  <pre v-if="pending && streamText" class="stream-text">{{ streamText }}</pre><p v-if="streamIssue && pending" class="hint">{{streamIssue}}</p>
  <ElAlert v-if="run.error" :title="run.error" type="error" :closable="false"/>
  <div v-if="images.length" class="gallery"><div v-for="image in images" :key="image.id"><ElImage :src="image.url" :preview-src-list="images.map(x=>x.url)" fit="contain"/><p>{{ image.name }}</p></div></div>
  <div v-for="file in run.artifacts || []" :key="file.file_id"><ElButton text @click="download(file)">下载 {{ file.name }}（{{ Math.ceil(file.size/1024) }} KB）</ElButton></div>
  <pre v-if="outputText !== null" class="output-text">{{ outputText }}</pre>
  <ElCollapse v-if="extraResult"><ElCollapseItem title="补充信息"><pre>{{ JSON.stringify(extraResult,null,2) }}</pre></ElCollapseItem></ElCollapse>
  <ElCollapse v-if="run.logs"><ElCollapseItem title="执行日志"><pre>{{ run.logs }}</pre></ElCollapseItem></ElCollapse>
 </section>
</template>
<script setup lang="ts">
import {toolApiPrefix} from '@/api/tooldeck'
import {ref,computed,watch,onBeforeUnmount} from 'vue'
import {useUserStore} from '@/store/modules/user'
import {td,type Run,type Artifact} from '@/api/tooldeck'
const streamText=ref(''),streamIssue=ref('')
let streamController:AbortController|undefined
async function subscribe(id:string,controller:AbortController){
 let cursor='',finished=false
 for(let attempt=0;attempt<4&&!controller.signal.aborted&&!finished;attempt++){
 try{
 const response=await fetch(`${import.meta.env.VITE_API_URL}${toolApiPrefix()}/runs/${id}/events`,{headers:{Authorization:`Bearer ${useUserStore().accessToken}`,Accept:'text/event-stream',...(cursor?{'Last-Event-ID':cursor}:{})},signal:controller.signal})
 if(!response.ok||!response.body)throw new Error('stream unavailable')
 const reader=response.body.getReader(),decoder=new TextDecoder();let buffer=''
 try{while(!controller.signal.aborted){const {value,done}=await reader.read();if(done)break;buffer+=decoder.decode(value,{stream:true});let split
 while((split=buffer.indexOf('\n\n'))>=0){const block=buffer.slice(0,split);buffer=buffer.slice(split+2);let kind='',data='',eventId='';for(const line of block.split('\n')){if(line.startsWith('event:'))kind=line.slice(6).trim();if(line.startsWith('data:'))data+=line.slice(5).trim();if(line.startsWith('id:'))eventId=line.slice(3).trim()}
 if(!data)continue;const payload=JSON.parse(data)
 if(kind==='delta'){if(!eventId||Number(eventId)>Number(cursor||0))streamText.value+=payload.text; if(eventId)cursor=eventId}
 if(kind==='result')emit('update',payload)
 if(kind==='done')finished=true
 }
 }}finally{await reader.cancel().catch(()=>{});reader.releaseLock()}
 if(!finished)throw new Error('stream disconnected')
 }catch{if(controller.signal.aborted)return;if(attempt===3){streamIssue.value='实时连接暂不可用，运行记录仍会刷新。';return}await new Promise(resolve=>setTimeout(resolve,1000))}
 }
}
const props=defineProps<{run:Run|null}>();const emit=defineEmits(['update'])
// Render the common {result: ...} package envelope without showing its field name.
const wrappedResult=computed(()=>{const value=props.run?.result;return value!==null && typeof value==='object' && !Array.isArray(value) && Object.prototype.hasOwnProperty.call(value,'result')})
const outputText=computed(()=>{const value=wrappedResult.value?props.run?.result.result:props.run?.result;if(value===undefined || value===null)return null;return typeof value==='string'?value:JSON.stringify(value,null,2)})
const extraResult=computed(()=>{if(!wrappedResult.value)return null;const extra=Object.fromEntries(Object.entries(props.run!.result).filter(([key])=>key!=='result'));return Object.keys(extra).length?extra:null})
const pending=computed(()=>['queued','running'].includes(props.run?.status||''))
const labels:Record<string,string>={queued:'排队中',running:'执行中',succeeded:'已完成',failed:'失败',timed_out:'已超时',canceled:'已取消'}
const images=ref<{id:string;url:string;name:string}[]>([]);let generation=0
function clear(){images.value.forEach(x=>URL.revokeObjectURL(x.url));images.value=[]}
watch(()=>props.run?.artifacts,async(files)=>{const gen=++generation;clear();for(const f of files||[]){if(['image/png','image/jpeg','image/webp','image/gif'].includes(f.mime)){try{const blob=await td.blob(f.file_id);if(gen!==generation)return;images.value.push({id:f.file_id,url:URL.createObjectURL(blob),name:f.name})}catch{}}}},{immediate:true})
async function download(f:Artifact){const blob=await td.blob(f.file_id);const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download=f.name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000)}
async function cancel(){if(props.run)emit('update',await td.cancel(props.run.run_id))}
watch(()=>props.run?.run_id,id=>{streamController?.abort();streamText.value='';streamIssue.value='';if(id&&pending.value){streamController=new AbortController();void subscribe(id,streamController)}},{immediate:true})
onBeforeUnmount(()=>{streamController?.abort();generation++;clear()})
</script>
<style scoped>
.result{margin-top:24px;border-top:1px solid var(--el-border-color);padding-top:20px}.result-heading{display:flex;align-items:center;gap:12px}.hint{font-size:12px;color:var(--el-text-color-secondary);margin:8px 0}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--el-fill-color-light);padding:16px;border-radius:8px;max-height:400px;overflow:auto;margin-top:12px}.gallery{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:16px;margin-top:18px}.gallery .el-image{width:100%;height:240px}
</style>
