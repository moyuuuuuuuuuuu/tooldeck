# MCP 接入

地址：`https://你的域名/mcp`，本地后端为 `http://localhost:18088/mcp`。使用官方 MCP Go SDK 的无会话 Streamable HTTP，支持初始化、ping、工具发现和工具调用；响应为 JSON。GET/DELETE 返回 405，不提供旧版 `/sse` 入口。

## 共用 API Key

MCP 与 `/api/v1` 共用“访问凭证”中的 API Key，无需创建专用 Key。每个请求携带任一请求头：

```http
Authorization: Bearer YOUR_API_KEY
```

```http
X-API-Key: YOUR_API_KEY
```

工具授权范围、有效期、撤销、可见性和“允许 API / MCP 调用”开关均生效。每次请求重新鉴权，初始化不保留已撤销的权限。OAuth 也复用已配置的 introspection 验证；平台不提供 OAuth 授权服务器或自动发现登录流程，客户端需支持手动配置请求头。

API 和 MCP 使用相同 Key 时，共用任务身份、个人环境变量、输入文件和产物，可交叉查询、取消和下载。不同账号或身份不能访问对方任务。API Key 不授予管理权限。真实 Key 不应放入 URL、源码或浏览器前端。

## 客户端配置

支持远程 Streamable HTTP 和自定义请求头的客户端可参考以下配置；具体字段名以客户端文档为准：

```json
{
  "mcpServers": {
    "tooldeck": {
      "type": "http",
      "url": "https://你的域名/mcp",
      "headers": { "Authorization": "Bearer YOUR_API_KEY" }
    }
  }
}
```

线上使用 HTTPS，反向代理须将 `/mcp` 转发至 Go 后端。有 Origin 的请求只允许与访问域名一致的有效 HTTP(S) Origin；无 Origin 的服务端/桌面客户端可正常接入。

## 发现、执行与结果

`tools/list` 按身份授权、工具可见性、构建/审核状态、默认发布版本及灰度返回可执行工具。关闭 API / MCP 的工具不会暴露；下架或未就绪工具不进入发现列表。本服务不宣告 `tools.listChanged`，也不发送工具列表变更通知。发布新版本、调整授权或开关后，调用方须重新执行 `tools/list`；客户端缓存无法手动刷新时重新连接。

| MCP 工具 | 参数 | 用途 |
| --- | --- | --- |
| `tooldeck_run_{tool_id}` | `input`，可选 `env`、`idempotency_key`、`callback_url`、`callback_secret` | 提交发现列表中指定版本的任务 |
| `tooldeck_get_run` | `run_id` | 查询精简状态、小结果和产物摘要 |
| `tooldeck_cancel_run` | `run_id` | 使用现有取消流程及提供方取消钩子 |
| `tooldeck_read_run_output` | `run_id`、`section`，可选 `offset`、`limit` | 分页读取结果 JSON 或日志文本 |

执行工具的 `inputSchema.properties.input` 是原有工具参数定义。`env` 只允许已声明的变量、仅覆盖本次运行；省略时使用 Key 所属用户的个人配置，按工具策略回退作者共享配置。复用参数、文件权限、环境校验、持久化队列和现有并发/容量限制。

所有 MCP 提交立即返回任务，不在请求中等待工具执行完成。提交、查询和取消均返回精简任务摘要，同时提供 JSON 文本 `content` 与对象 `structuredContent`。其中 `run_id` 用于后续查询，`details_url` 是完整 REST 记录的相对地址，需要同一 API Key。`queued`、`running`、`canceling` 表示尚未结束；终态为 `succeeded`、`failed`、`timed_out`、`canceled`、`cancel_failed`。查询应间隔数秒。MCP HTTP 200 表示协议调用成功，不代表业务任务已成功。

默认响应不包含 input、logs 或 stream_events。完成任务的 result JSON 不超过 16 KiB 时内联返回；更大的结果返回 `result: null`、`result_truncated: true` 和 `result_bytes`。这表示省略了内联内容，并未删除存储的结果。产物最多返回 16 条摘要，额外产物通过 `artifacts_count` 和 `artifacts_truncated` 提示；完整记录使用 details_url 读取。错误说明最多显示 1024 个 Unicode 字符，较长时以省略号结尾。

读取完整输出时调用 `tooldeck_read_run_output`：`section` 为 `result`（完整结果的 JSON 文本）或 `logs`。offset 从 0 起按 Unicode 字符计数，limit 默认 2048、范围 1–4096。按返回的 `next_offset` 继续，`has_more: false` 表示结束；按顺序拼接 text 可还原全文，result 全文再做 JSON 解析。结果尚未完成时读取 result 返回工具错误 409；日志仅提供当前快照，后续新增内容可再次读取。分页沿用任务所有权和工具授权检查，不改变任务状态。

```json
{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"tooldeck_read_run_output","arguments":{"run_id":"RUN_ID","section":"result","offset":0,"limit":2048}}}
```

异步 MCP 无需回调服务器，默认查询结果；也可传公网 HTTPS `callback_url` 和可选 `callback_secret`，复用现有回调投递/验签。同步工具不接受回调参数。原 REST 异步 API 仍要求 `callback_url`。

重试同一提交时复用 `idempotency_key`，保持 input、env、回调地址与密钥一致；新操作生成新键。它与 REST `Idempotency-Key` 共用索引：同一身份、工具版本和键不会重复创建任务，参数冲突会失败。

断开连接不会取消任务，需要显式调用 `tooldeck_cancel_run`。任务来源标记 `mcp`，创建/取消复用操作审计，每次操作只写一条审计记录。初始化、ping、工具发现、状态查询和输出分页不写审计或持久化状态。审计不保存请求体、Key 或环境值。

文件继续使用 REST：`POST /api/v1/files` 的 multipart `file` 字段上传后，把 `file_id` 填入 input；用同一 Key 请求 `GET /api/v1/files/{file_id}` 下载产物。原保存期限/下载次数限制继续生效。MCP 当前不提供二进制上传下载、实时 delta 订阅；需要实时事件时使用原 REST SSE 接口。

## HTTP 示例

客户端通常自动初始化。手工调用时 POST 同时接受 JSON 和 SSE，初始化后携带协商的协议版本。以下使用 `2025-11-25`，无需 `Mcp-Session-Id`。

```bash
# 初始化
curl 'http://localhost:18088/mcp' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"manual-client","version":"1.0.0"}}}'

# 初始化完成通知（HTTP 202，无响应体）
curl 'http://localhost:18088/mcp' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'MCP-Protocol-Version: 2025-11-25' \
  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}'

# 发现实际工具名称和参数
curl 'http://localhost:18088/mcp' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'MCP-Protocol-Version: 2025-11-25' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'

# 使用发现的工具名，替换 TOOL_ID
curl 'http://localhost:18088/mcp' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'MCP-Protocol-Version: 2025-11-25' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"tooldeck_run_TOOL_ID","arguments":{"input":{"text":"hello"},"idempotency_key":"YOUR_UNIQUE_REQUEST_ID"}}}'

# 间隔数秒查询，替换 RUN_ID
curl 'http://localhost:18088/mcp' \
  -H 'Authorization: Bearer YOUR_API_KEY' \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'MCP-Protocol-Version: 2025-11-25' \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"tooldeck_get_run","arguments":{"run_id":"RUN_ID"}}}'
```

无效/过期/撤销凭证返回 HTTP 401，非法 Origin 为 403，不支持的协议版本为 400。参数、资源权限、环境和队列错误通过工具结果 `isError: true`、`structuredContent.status/message` 返回。不存在或已不可用的 MCP 工具返回 JSON-RPC 错误。业务执行失败见任务的 `status` 和 `error`。

参考：[MCP Streamable HTTP](https://modelcontextprotocol.io/specification/2025-11-25/basic/transports)、[官方 Go SDK](https://github.com/modelcontextprotocol/go-sdk)。
