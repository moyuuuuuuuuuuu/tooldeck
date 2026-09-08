<template>
 <section v-if="run" class="result">
  <div class="result-heading"><h3>执行结果</h3><ElTag :type="run.status==='succeeded'?'success':run.status==='failed'?'danger':'info'">{{ labels[run.status] || run.status }}</ElTag><span>{{ run.duration_ms }} ms</span><ElButton v-if="pending" text type="danger" @click="cancel">取消执行</ElButton></div>
  <p class="hint">{{ run.run_id }}</p><ElProgress v-if="pending" :percentage="50" :indeterminate="true" :show-text="false"/>
  <pre v-if="pending && streamText" class="stream-text">{{ streamText }}</pre><p v-if="streamIssue && pending" class="hint">{{streamIssue}}</p>
  <ElAlert v-if="run.error" :title="run.error" type="error" :closable="false"/>
  <ElAlert v-if="run.callback_failed" class="callback-state" :title="`结果回调最终失败（共尝试 ${run.callback_attempts || 6} 次）`" :description="run.callback_error || '回调接收方未返回成功状态'" type="error" :closable="false" show-icon/>
  <ElAlert v-else-if="run.callback_sent" class="callback-state" :title="`结果已成功回调（第 ${run.callback_attempts || 1} 次送达）`" type="success" :closable="false" show-icon/>
  <ElAlert v-else-if="run.callback_url && !pending" class="callback-state" :title="run.callback_attempts ? `回调失败，等待第 ${run.callback_attempts + 1} 次尝试` : '等待发送结果回调'" :description="run.callback_error" type="warning" :closable="false" show-icon/>
  <div v-if="run.artifacts?.length" class="artifacts"><ElAlert title="运行产物默认保留 7 天；每个文件最多下载 3 次，到期或次数用完后自动删除。" type="warning" :closable="false"/><div v-for="file in run.artifacts" :key="file.file_id" class="artifact"><div><strong>{{file.name}}</strong><p>{{Math.ceil(file.size/1024)}} KB · 剩余 {{file.downloads_remaining ?? 3}} 次<span v-if="file.expires_at"> · {{new Date(file.expires_at).toLocaleString()}} 到期</span></p></div><ElButton type="primary" plain :disabled="!available(file)" @click="download(file)">下载</ElButton></div></div>
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
async function download(f:Artifact){const blob=await td.blob(f.file_id);const url=URL.createObjectURL(blob);const a=document.createElement('a');a.href=url;a.download=f.name;a.click();setTimeout(()=>URL.revokeObjectURL(url),1000);const remaining=Math.max(0,(f.downloads_remaining??3)-1);emit('update',{...props.run,artifacts:remaining===0?props.run!.artifacts.filter(x=>x.file_id!==f.file_id):props.run!.artifacts.map(x=>x.file_id===f.file_id?{...x,downloads_remaining:remaining}:x)})}
const available=(f:Artifact)=>(f.downloads_remaining??3)>0&&(!f.expires_at||new Date(f.expires_at).getTime()>Date.now())
async function cancel(){if(props.run)emit('update',await td.cancel(props.run.run_id))}
watch(()=>props.run?.run_id,id=>{streamController?.abort();streamText.value='';streamIssue.value='';if(id&&pending.value){streamController=new AbortController();void subscribe(id,streamController)}},{immediate:true})
onBeforeUnmount(()=>streamController?.abort())
</script>
<style scoped>
.result{margin-top:24px;border-top:1px solid var(--el-border-color);padding-top:20px}.result-heading{display:flex;align-items:center;gap:12px}.hint{font-size:12px;color:var(--el-text-color-secondary);margin:8px 0}.callback-state{margin-top:12px}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--el-fill-color-light);padding:16px;border-radius:8px;max-height:400px;overflow:auto;margin-top:12px}.artifacts{display:grid;gap:12px;margin-top:18px}.artifact{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:14px;border:1px solid var(--el-border-color);border-radius:8px}.artifact p{margin:6px 0 0;color:var(--el-text-color-secondary);font-size:12px}
</style>
