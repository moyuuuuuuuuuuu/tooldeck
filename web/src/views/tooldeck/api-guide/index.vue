<template>
  <div class="api-guide">
    <header v-if="!embedded" class="tooldeck-page-hero"
      ><div
        ><span class="eyebrow">TOOLDECK API / MCP</span><h1>API / MCP 调用指南</h1
        ><p
          >通过 REST API 或 MCP 发现工具、提交任务、查询结果和下载产物，两种方式共用 API Key。</p
        ></div
      ><ElButton type="primary" @click="router.push('/tooldeck/credentials')"
        >管理 API Key</ElButton
      ></header
    >
    <main>
      <aside
        ><span class="nav-label">API / MCP 接入</span
        ><a v-for="(item, i) in sections" :key="item.id" :href="'#' + item.id"
          ><small>{{ String(i + 1).padStart(2, '0') }}</small
          >{{ item.title }}</a
        ></aside
      >
      <article>
        <section id="auth"
          ><h2>1. 身份认证</h2
          ><p
            >在“访问凭证”创建 API Key，REST API 和 MCP 可以共用同一个 Key，通过请求头传递。不要把
            Key 放入 URL、源码或浏览器前端。</p
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
          ><h2>4. 查询、回调与取消任务</h2
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
          ><pre>GET /api/v1/runs/{run_id}</pre
          ><p
            >处理中状态包括 queued、running、canceling；终态包括
            succeeded、failed、timed_out、canceled、cancel_failed。使用创建任务时返回的 run_id
            取消尚未结束的任务：</p
          ><pre>{{ cancelCurl }}</pre
          ><p
            >排队任务直接取消；运行中任务先进入 canceling，待工具取消钩子完成后进入 canceled 或
            cancel_failed。接口返回最新任务记录，重复请求不会重复执行取消钩子。</p
          >
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
        <section id="mcp">
          <h2>9. MCP 接入</h2>
          <p>
            MCP 地址为 <code>https://你的域名/mcp</code>，本地为
            <code>http://localhost:18088/mcp</code>。采用无会话 Streamable HTTP，支持远程 HTTP
            和自定义请求头的客户端可参考以下配置，字段名以客户端文档为准：
          </p>
          <pre>{{ mcpConfig }}</pre>
          <p>
            直接使用现有 API Key，可通过 <code>Authorization: Bearer YOUR_API_KEY</code> 或
            <code>X-API-Key</code> 传递。工具授权、有效期、撤销、可见性及“允许 API / MCP
            调用”开关同时生效。OAuth 复用平台已配置的令牌验证，不提供自动发现登录流程。
          </p>
          <p>
            客户端通过 <code>tools/list</code> 发现当前可调用版本和参数，通过
            <code>tools/call</code> 调用返回的 <code>tooldeck_run_{tool_id}</code>。 所有 MCP
            提交立即返回任务，使用 <code>tooldeck_get_run</code> 查询结果，
            <code>tooldeck_cancel_run</code> 取消任务。
          </p>
          <pre>{{ mcpCalls }}</pre>
          <p>
            结果中的 <code>structuredContent</code> 是精简任务摘要，包含 run_id、status、小结果和
            artifacts，不包含 input、logs 或 stream_events。queued、running、canceling
            表示尚未结束，查询应间隔数秒；HTTP 200 不代表任务执行成功。参数或权限失败会返回
            <code>isError: true</code>。
          </p>
          <p>
            完成任务的结果 JSON 不超过 16 KiB 时直接返回；更大时会标记
            <code>result_truncated: true</code> 并提供 result_bytes。 用
            <code>tooldeck_read_run_output</code> 分页读取，section 为 result 或 logs， offset 从 0
            起按 Unicode 字符计数，limit 默认 2048、最多 4096。 按 next_offset 继续，直到 has_more
            为 false，拼接 text 可还原完整输出。 产物摘要最多 16 条，完整记录通过 details_url
            和同一个 Key 获取。
          </p>
          <p>
            初始化、ping、发现、查询和分页不会写入操作审计；创建与取消保留审计。
            服务不发送工具列表变更通知，发布新版本或调整授权后需重新执行 tools/list，
            客户端无法刷新缓存时重新连接。
          </p>
          <p>
            异步 MCP 默认查询结果，无需提供 callback_url；也可传入公网 HTTPS 回调地址。 原 REST 异步
            API 仍要求 callback_url。env 使用 Key 所属用户的个人配置， idempotency_key 与 REST 的
            Idempotency-Key 共用幂等记录。
          </p>
          <p>
            文件仍通过上述 REST 接口上传和下载，使用同一个 Key；运行记录来源标记为 MCP。 断开 MCP
            连接不会取消任务。实时 delta 使用 REST SSE 接口，当前 MCP 不提供实时订阅。 手工 POST
            时设置 Content-Type: application/json、Accept: application/json,
            text/event-stream，初始化后携带协商的 MCP-Protocol-Version。 无需
            Mcp-Session-Id，GET/DELETE 返回 405。
          </p>
        </section>
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
    { id: 'status', title: '查询与取消' },
    { id: 'files', title: '文件接口' },
    { id: 'retry', title: '安全重试' },
    { id: 'example', title: '完整示例' },
    { id: 'errors', title: '状态码' },
    { id: 'mcp', title: 'MCP 接入' }
  ]
  const authHeaders = `X-API-Key: YOUR_API_KEY\nContent-Type: application/json`
  const mcpConfig = JSON.stringify(
    {
      mcpServers: {
        tooldeck: {
          type: 'http',
          url: 'https://你的域名/mcp',
          headers: { Authorization: 'Bearer YOUR_API_KEY' }
        }
      }
    },
    null,
    2
  )
  const mcpCalls = `// 提交：使用 tools/list 返回的实际工具名\n{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tooldeck_run_TOOL_ID","arguments":{"input":{"text":"hello"},"idempotency_key":"YOUR_UNIQUE_REQUEST_ID"}}}\n\n// 查询：使用任务返回的实际 run_id\n{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"tooldeck_get_run","arguments":{"run_id":"RUN_ID"}}}\n\n// 取消\n{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"tooldeck_cancel_run","arguments":{"run_id":"RUN_ID"}}}`
  const requestBody = `{\n  "input": { "text": "hello" },\n  "env": { "API_KEY": "仅本次使用的值" },\n  "callback_url": "https://example.com/tooldeck/callback",\n  "callback_secret": "用于HMAC-SHA256验签的密钥"\n}`
  const callbackBody = `{\n  "event": "tool.run.completed",\n  "run_id": "run_xxx",\n  "tool_id": "tool_xxx",\n  "status": "succeeded",\n  "result": { "result": "..." },\n  "artifacts": [],\n  "created_at": "...",\n  "started_at": "...",\n  "duration_ms": 1200\n}`
  const fileUpload = `POST /api/v1/files\nContent-Type: multipart/form-data\nfile=@/path/to/input.png`
  const fileDownload = `GET /api/v1/files/{file_id}\nX-API-Key: YOUR_API_KEY`
  const cancelCurl = `curl -X POST 'https://你的域名/api/v1/runs/RUN_ID/cancel' \\
  -H 'X-API-Key: YOUR_API_KEY'`
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
