<template>
  <div class="api-guide">
    <header v-if="!embedded" class="tooldeck-page-hero"
      ><div
        ><span class="eyebrow">TOOLDECK API</span><h1>API 调用指南</h1
        ><p>完成工具发现、任务提交、异步结果回调和文件产物下载的通用接入说明。</p></div
      ><ElButton type="primary" @click="router.push('/tooldeck/credentials')"
        >管理 API Key</ElButton
      ></header
    >
    <main>
      <aside
        ><span class="nav-label">API 接入</span
        ><a v-for="(item, i) in sections" :key="item.id" :href="'#' + item.id"
          ><small>{{ String(i + 1).padStart(2, '0') }}</small
          >{{ item.title }}</a
        ></aside
      >
      <article>
        <section id="auth"
          ><h2>1. 身份认证</h2
          ><p>在“访问凭证”创建 API Key，并通过请求头传递。不要把 Key 放入 URL、源码或浏览器前端。</p
          ><pre>{{ authHeaders }}</pre
          ><p
            >OAuth 使用 <code>Authorization: Bearer TOKEN</code>，授权范围为
            <code>tool:工具名</code> 或 <code>tool:*</code>。</p
          ></section
        >
        <section id="tools"
          ><h2>2. 获取可访问工具</h2><pre>GET /api/v1/tools</pre
          ><p
            >返回当前凭证有权使用的工具版本。调用执行接口时使用返回记录的
            <code>id</code>，不要使用工具显示名称代替。</p
          ></section
        >
        <section id="submit"
          ><h2>3. 提交工具执行</h2><pre>POST /api/v1/tools/{tool_id}/runs</pre
          ><p
            >工具详情提供准确请求地址和参数定义。请求体顶层为 <code>input</code>；可选的
            <code>env</code> 仅用于本次运行。异步工具必须传入 HTTPS <code>callback_url</code>，可选
            <code>callback_secret</code> 用于验签。</p
          ><pre>{{ requestBody }}</pre
          ><p
            >env 省略时使用 API Key
            所属用户保存的个人配置，并按工具策略回退作者共享配置。callback_url 仅允许公网 HTTPS
            地址，禁止内网、回环和保留地址。</p
          ></section
        >
        <section id="status"
          ><h2>4. 接收异步结果</h2
          ><p
            >任务结束后，平台向 callback_url POST 完整结果；接收方返回任意 2xx 即视为成功。失败后按
            3、30、300、3000、300000 秒重试，共重试 5 次。</p
          ><pre>{{ callbackBody }}</pre
          ><p
            >设置 callback_secret 后，请按原始请求体计算 HMAC-SHA256，并与
            <code>X-ToolDeck-Signature: sha256=...</code> 做常量时间比较。请求还包含
            <code>X-ToolDeck-Event</code> 和
            <code>X-ToolDeck-Run-ID</code
            >。最终失败时仅产生站内信，不发送邮件。排障时仍可使用同一凭证查询：</p
          ><pre>GET /api/v1/runs/{run_id}</pre><p>可取消尚未结束的任务：</p
          ><pre>POST /api/v1/runs/{run_id}/cancel</pre>
        </section>
        <section id="files"
          ><h2>5. 文件上传与下载</h2
          ><p>输入文件先上传，再把响应中的 file_id 放入工具 input 对应字段：</p
          ><pre>{{ fileUpload }}</pre
          ><p>执行结果中的产物使用同一个 API Key 下载：</p><pre>{{ fileDownload }}</pre
          ><p>产物默认保存 7 天、最多下载 3 次；到期或次数耗尽后自动删除。</p></section
        >
        <section id="retry"
          ><h2>6. 幂等与安全重试</h2
          ><p
            >每次新操作生成新的
            <code>Idempotency-Key</code>。网络错误后重试同一次操作时复用原值，并保持 input 和 env
            完全一致。</p
          ><pre>Idempotency-Key: 550e8400-e29b-41d4-a716-446655440000</pre>
        </section>
        <section id="example"
          ><h2>7. 完整异步调用示例</h2><pre>{{ curl }}</pre>
        </section>
        <section id="errors"
          ><h2>8. 常见状态码</h2
          ><ul
            ><li>200：执行完成或查询成功</li
            ><li>202：任务已接受，需要继续查询</li
            ><li>401/403：凭证无效或没有工具权限</li
            ><li>404：资源不存在或当前身份无权访问</li
            ><li>409：工具未就绪或幂等参数不一致</li
            ><li>422：输入、文件或环境变量校验失败</li
            ><li>429：队列已满或触发频率限制</li></ul
          ><p>env 不会出现在响应、调用记录或执行日志中；异步任务所需临时值会加密保存。</p></section
        >
      </article>
    </main>
  </div>
</template>
<script setup lang="ts">
  import { useRouter } from 'vue-router'
  defineOptions({ name: 'ApiGuide' })
  defineProps<{ embedded?: boolean }>()
  const router = useRouter()
  const sections = [
    { id: 'auth', title: '身份认证' },
    { id: 'tools', title: '访问工具' },
    { id: 'submit', title: '提交执行' },
    { id: 'status', title: '结果回调' },
    { id: 'files', title: '文件接口' },
    { id: 'retry', title: '安全重试' },
    { id: 'example', title: '完整示例' },
    { id: 'errors', title: '状态码' }
  ]
  const authHeaders = `X-API-Key: YOUR_API_KEY\nContent-Type: application/json`
  const requestBody = `{\n  "input": { "text": "hello" },\n  "env": { "API_KEY": "仅本次使用的值" },\n  "callback_url": "https://example.com/tooldeck/callback",\n  "callback_secret": "用于HMAC-SHA256验签的密钥"\n}`
  const callbackBody = `{\n  "event": "tool.run.completed",\n  "run_id": "run_xxx",\n  "tool_id": "tool_xxx",\n  "status": "succeeded",\n  "result": { "result": "..." },\n  "artifacts": [],\n  "created_at": "...",\n  "started_at": "...",\n  "duration_ms": 1200\n}`
  const fileUpload = `POST /api/v1/files\nContent-Type: multipart/form-data\nfile=@/path/to/input.png`
  const fileDownload = `GET /api/v1/files/{file_id}\nX-API-Key: YOUR_API_KEY`
  const curl = `# 1. 获取可访问工具\ncurl 'https://你的域名/api/v1/tools' -H 'X-API-Key: YOUR_API_KEY'\n\n# 2. 提交异步任务\ncurl -X POST 'https://你的域名/api/v1/tools/TOOL_ID/runs' \\\n+  -H 'X-API-Key: YOUR_API_KEY' \\\n+  -H 'Idempotency-Key: YOUR_UNIQUE_REQUEST_ID' \\\n+  -H 'Content-Type: application/json' \\\n+  -d '{"input":{"text":"hello"},"callback_url":"https://example.com/tooldeck/callback","callback_secret":"YOUR_CALLBACK_SECRET"}'\n\n# 3. 接收端校验 X-ToolDeck-Signature 并返回任意 2xx\n# 排障时仍可查询任务\ncurl 'https://你的域名/api/v1/runs/RUN_ID' -H 'X-API-Key: YOUR_API_KEY'\n\n# 4. 下载结果文件\ncurl -OJ 'https://你的域名/api/v1/files/FILE_ID' -H 'X-API-Key: YOUR_API_KEY'`
</script>
<style scoped>
  .api-guide {
    max-width: 1320px;
    margin: auto;
  }
  .api-guide > header {
    padding: 32px;
    margin-bottom: 30px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 20px;
    background: linear-gradient(
      115deg,
      var(--el-color-primary-light-9),
      var(--el-bg-color) 65%,
      var(--el-color-success-light-9)
    );
  }
  .eyebrow {
    font-size: 11px;
    letter-spacing: 2px;
    color: var(--el-color-primary);
    font-weight: 700;
  }
  h1 {
    font-size: 32px;
    margin: 8px 0;
  }
  header p {
    max-width: 760px;
    color: var(--el-text-color-secondary);
    line-height: 1.8;
  }
  header .el-button {
    position: static;
    transform: none;
  }
  main {
    display: grid;
    grid-template-columns: 210px minmax(0, 1fr);
    gap: 40px;
  }
  aside {
    position: sticky;
    top: 88px;
    align-self: start;
    max-height: calc(100vh - 112px);
    overflow-y: auto;
    padding: 18px 0;
  }
  .nav-label {
    display: block;
    margin-bottom: 16px;
    font-size: 10px;
    font-weight: 700;
    letter-spacing: 1.8px;
    color: #6575ad;
  }
  aside a {
    display: flex;
    gap: 12px;
    padding: 10px 12px;
    margin-left: 0;
    border-radius: 8px;
    color: var(--el-text-color-secondary);
    text-decoration: none;
    font-size: 13px;
    transition: background 0.15s;
  }
  aside a:hover {
    color: var(--el-color-primary);
    background: var(--el-color-primary-light-9);
  }
  aside small {
    color: #98a1b6;
    font-size: 11px;
  }
  section {
    padding: 28px 30px;
    margin-bottom: 20px;
    border: 1px solid var(--el-border-color-lighter);
    border-radius: 16px;
    background: var(--el-bg-color);
    scroll-margin-top: 24px;
  }
  h2 {
    margin: 0 0 16px;
    font-size: 21px;
  }
  p,
  li {
    color: var(--el-text-color-secondary);
    line-height: 1.85;
    font-size: 14px;
  }
  pre {
    padding: 18px 20px;
    border-radius: 10px;
    background: var(--el-fill-color-light);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    line-height: 1.7;
  }
  code {
    color: var(--el-color-primary);
  }
  ul {
    padding-left: 20px;
  }
  @media (max-width: 800px) {
    .api-guide > header {
      padding: 26px 22px;
    }
    header .el-button {
      position: static;
      transform: none;
      margin-top: 10px;
    }
    main {
      grid-template-columns: 1fr;
    }
    aside {
      position: static;
      display: flex;
      flex-wrap: wrap;
      gap: 6px;
      overflow: auto;
      max-height: none;
      padding: 0;
    }
    .nav-label {
      display: none;
    }
    aside a {
      white-space: nowrap;
      padding: 6px 10px;
      margin: 0;
      border: 1px solid var(--el-border-color-lighter);
    }
    section {
      padding: 22px 18px;
    }
    h1 {
      font-size: 27px;
    }
  }
</style>
