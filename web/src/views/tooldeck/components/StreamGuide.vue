<template><div><p>在 tooldeck.json 中设置 execution.stream: true。网页会实时显示增量，普通 API 提交立即返回 run_id；最终 JSON 结果、日志和文件产物协议保持不变。</p><pre>{{ config }}</pre><h3>工具侧协议</h3><p>将每个片段写入标准错误（stderr）：固定前缀 TOOLDECK_EVENT 后紧跟一行 JSON，再换行并 flush。type 当前仅支持 delta，text 是追加文本。普通 stderr 行仍作为日志；stdout 最后输出一个完整 JSON 结果。</p><pre>{{ protocol }}</pre><ElTabs><ElTabPane v-for="(code,language) in samples" :key="language" :label="language"><pre>{{code}}</pre></ElTabPane></ElTabs><p>对接大模型时，启用服务商的流式接口，在每次收到 token／文本片段时发送 delta；结束时将完整结果写入 stdout。不要把上游原始 SSE 帧直接打印到 stdout，也不要在事件中输出密钥。</p><h3>API 调用</h3><pre>{{ api }}</pre><p>也可以先用普通 POST 创建任务，再 GET /api/v1/runs/{run_id}/events 订阅。使用相同登录令牌、API Key 或 OAuth，网页通过 fetch 携带请求头，不把凭证放进 URL。</p><p>事件依次包含 run（任务编号）、status（状态）、delta（text 增量）、result（完整任务记录）和 done；失败时额外返回 error（message），长连接每 15 秒发送 heartbeat。只有 delta 带递增 id，重连 GET 时在 Last-Event-ID 中传最后收到的编号。网络断开不取消任务，取消使用原 cancel 接口。</p><p>单条事件最多 64 KB，最多 4096 条，stderr 含事件合计最多 3 MB。正常结束后事件随运行记录保存；进程意外中断时尚未落盘的片段可能丢失，应以任务记录为准。反向代理需关闭响应缓冲并允许长连接；平台发送 X-Accel-Buffering: no。</p></div></template>
<script setup lang="ts">
const config='"execution": {"mode":"async","stream":true,"timeout_seconds":120,"memory_mb":256}'
const protocol='TOOLDECK_EVENT {"type":"delta","text":"你好"}\nTOOLDECK_EVENT {"type":"delta","text":"，世界"}'
const samples={PHP:`fwrite(STDERR, "TOOLDECK_EVENT " . json_encode(["type"=>"delta", "text"=>$chunk], JSON_UNESCAPED_UNICODE) . "\\n");
fflush(STDERR);
// 模型完成后：
echo json_encode(["text"=>$fullText], JSON_UNESCAPED_UNICODE);`,Node:`process.stderr.write('TOOLDECK_EVENT ' + JSON.stringify({type:'delta', text:chunk}) + '\\n');
// 模型完成后：
process.stdout.write(JSON.stringify({text:fullText}));`,Python:`print('TOOLDECK_EVENT ' + json.dumps({'type':'delta','text':chunk}, ensure_ascii=False), file=sys.stderr, flush=True)
# 模型完成后：
print(json.dumps({'text':full_text}, ensure_ascii=False))`,Go:`event, _ := json.Marshal(map[string]string{"type":"delta", "text":chunk})
fmt.Fprintf(os.Stderr, "TOOLDECK_EVENT %s\\n", event)
// 模型完成后：
json.NewEncoder(os.Stdout).Encode(map[string]string{"text":fullText})`}
const api=`curl -N -X POST '${location.origin}/api/v1/tools/TOOL_ID/runs' \\
  -H 'X-API-Key: YOUR_KEY' \\
  -H 'Accept: text/event-stream' \\
  -H 'Content-Type: application/json' \\
  -H 'Idempotency-Key: UNIQUE_REQUEST_ID' \\
  -d '{"input":{"prompt":"你好"}}'`
</script>
<style scoped>p{font-size:13px;line-height:1.9;color:var(--el-text-color-secondary);margin:12px 0}h3{margin:20px 0 10px;font-size:16px;font-weight:600}pre{white-space:pre-wrap;overflow-wrap:anywhere;background:var(--el-fill-color-light);padding:18px;border-radius:10px;font:12px/1.8 monospace}</style>
