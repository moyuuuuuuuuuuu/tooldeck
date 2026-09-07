# ToolDeck

一个面向个人使用与工具分享的代码工具站。上传源码 ZIP，即可通过动态表单或 API 执行工具；也可以直接在线编写代码并查看结果。

后端使用 Go，前端基于 SaiAdmin UI 改造为用户站点，执行任务使用独立 Docker 容器。

## 功能概览

- **工具库**：源码上传、版本管理、自动安装依赖与构建、公开审核和私有工具。
- **网页运行**：根据工具声明生成文本、数字、选项、JSON、图片及文件表单，展示结果、日志和产物。
- **在线运行**：单文件代码编辑、语法高亮、版本选择、标准输入、输出、停止运行与代码下载。
- **API 接入**：个人 API Key、长期有效 Key、所有工具通用授权，以及外部 OAuth 令牌验证。
- **账号与个人中心**：邮箱验证码注册、登录、默认字母头像、资料维护、密码修改和个人使用统计。
- **环境变量**：工具个人配置、账号统一配置和作者共享配置；值加密保存，不回显。
- **异步任务**：任务查询、取消、站内通知和邮件提醒。
- **文件存储**：百度智能云 BOS，或本地开发存储。

## 快速启动

需要 Docker Compose 和 Linux 容器环境。Windows 可使用 Docker Desktop 的 Linux 容器模式。

```sh
git clone https://github.com/moyuuuuuuuuuuu/tooldeck.git
cd tooldeck
cp .env.example .env
```

Windows PowerShell 复制配置可使用 `Copy-Item .env.example .env`。

编辑 `.env`，至少填写：

| 配置 | 说明 |
| --- | --- |
| `TOOLDECK_ADMIN_PASSWORD` | 首次初始化的管理员密码，至少 12 位 |
| `TOOLDECK_MASTER_KEY` | 64 位十六进制主密钥，用于加密配置，必须备份 |
| `TOOLDECK_STORAGE` | `bos` 或 `local`；未配置 BOS 时，本地体验设为 `local` |
| `TOOLDECK_BOS_AK` / `TOOLDECK_BOS_SK` | 使用 BOS 时必填 |

生成主密钥：

```sh
python -c "import secrets; print(secrets.token_hex(32))"
docker compose up -d --build
```

打开 **http://localhost:18088**，使用 `admin` 和配置的密码登录。管理员密码仅在首次初始化时使用，修改环境变量不会覆盖已有账号密码。

运行环境按需下载和构建，首次使用可能需要等待。也可以提前准备所有支持版本：

```powershell
# Windows PowerShell
./scripts/build-runtimes.ps1
```

```sh
# Linux
sh scripts/build-runtimes.sh
```

## 运行环境与工具包

| 语言 | 支持版本 | 依赖文件 |
| --- | --- | --- |
| PHP | 8.0–8.3 | `composer.json`、`composer.lock` |
| JavaScript / Node.js | 20–23 | `package.json`、`package-lock.json` |
| Python | 3.10–3.12 | 锁定版本并附完整哈希的 `requirements.txt` |
| Go | 1.22–1.24 | 模块项目使用 `go.mod`、`go.sum` |

ZIP 根目录放 `tooldeck.json` 和入口代码，不必上传 `node_modules` 或 `vendor`。代码从标准输入读取 JSON，向标准输出写一个 JSON 结果，日志写入标准错误。不要启动常驻 HTTP 服务。

```text
my-tool.zip
├── tooldeck.json
└── main.py
```

- 通过 `runtime_version` 或上传页面选择版本，使用 `build_command` 声明可选构建命令。
- 构建状态为 `queued → building → ready / failed`，失败可重试，成功版本固定产物与镜像 ID；修改依赖需上传新版本。
- 构建不注入业务密钥，仅允许受限依赖仓库访问；私有依赖仓库凭证暂不支持。
- 公开工具默认需审核，审核关闭时后续公开上传自动通过；私有工具无需审核，仅本人及管理员可见。
- 上传可选择是否使用第三方服务、异步通知以及允许 API 调用。

生成示例 ZIP：

```sh
python scripts/package-examples.py
```

在工具库上传 `work/examples/` 下的 ZIP。图片表单示例不调用真实 AI 服务。
`examples/blank-php`、`blank-js`、`blank-node`、`blank-python`、`blank-go` 提供五种运行时的空白工程模板；PHP 模板使用 Composer PSR-4 自动加载，其余模板也将入口与业务逻辑按语言习惯分离。

详细格式见 [工具包规范](docs/tool-packages.md)，也可查看站内 **开发指引**。

## 在线运行

支持上述语言和版本，采用上下布局的代码编辑区与结果区。JavaScript 使用 Node 引擎，不提供浏览器 DOM；在线编辑器只支持单文件与标准库，第三方依赖项目请上传工具包。

在线代码最多 64 KB，每用户滚动 24 小时最多新增 200 份不同代码；相同代码与版本复用构建。执行最多 10 秒、256 MB 内存，执行阶段禁网，不默认注入业务凭证。代码保持私有，不显示在工具库，结果可在“我的记录”查看。

## 环境变量与第三方服务

个人中心提供统一环境变量管理，最多 50 个。执行时按以下优先级匹配工具声明的变量：

**当前工具个人配置 → 当前工具作者共享配置**

作者可禁用共享回退。网页和本人 API Key 调用共用配置；未声明的变量不会自动注入。所有值加密保存且不回显，缺失必填变量时拒绝运行。不要把真实凭证放入源码、表单、结果或日志。

第三方 HTTP(S) 请求需声明精确公网域名，通过受限代理访问，仅开放 80/443 端口，不支持内网目标、通配域名或任意 TCP/UDP。客户端需遵循 `HTTP_PROXY` / `HTTPS_PROXY`。Fake-IP 网络可配置 `TOOLDECK_EGRESS_DOH`。

## API 接入

在“API 接入”创建个人 Key，可指定工具及 1–365 天有效期，也可选择长期有效、所有工具通用（含以后新增工具）。长期 Key 撤销前有效；通用授权仍遵守工具可见性与 API 开关。

```sh
curl -X POST http://localhost:18088/api/v1/tools/TOOL_ID/runs \
  -H 'X-API-Key: YOUR_API_KEY' \
  -H 'Idempotency-Key: YOUR_UNIQUE_REQUEST_ID' \
  -H 'Content-Type: application/json' \
  -d '{"input":{"text":"hello"}}'
```

响应包装为 `{"code":200,"data":...,"message":""}`，具体 HTTP 状态表示成功、受理或失败。同步等待最多 20 秒，超时返回 202，任务继续；异步返回 `data.run_id`。重试同一操作使用相同 `Idempotency-Key` 与输入，新操作使用新值。

| 方法 | 路径 | 用途 |
| --- | --- | --- |
| GET | `/api/v1/tools` | 查找有权使用的工具版本 ID |
| POST | `/api/v1/tools/{id}/runs` | 提交执行 |
| GET | `/api/v1/runs/{id}` | 查询状态、结果及产物 |
| POST | `/api/v1/runs/{id}/cancel` | 取消任务 |
| GET | `/api/v1/runs/{id}/logs` | 查看执行日志 |
| POST | `/api/v1/files` | 上传文件，multipart `file` 字段 |
| GET | `/api/v1/files/{id}` | 授权下载文件 |

工具详情提供 PHP、Java、Python、Go 调用示例。上传、账号配置与授权管理使用登录会话，执行 Key 不具备这些管理权限。

OAuth 使用外部 RFC 7662 introspection 服务，平台不提供授权服务器。配置 `.env.example` 中的 URL、客户端凭证和 audience，调用方携带 `Authorization: Bearer TOKEN`；scope 使用 `tool:工具名` 或 `tool:*`。

## 邮箱注册与通知

QQ SMTP 默认使用 `smtp.qq.com:465` 和 TLS，填写：

- `TOOLDECK_SMTP_FROM`：发件邮箱。
- `TOOLDECK_SMTP_USERNAME`：SMTP 登录邮箱。
- `TOOLDECK_SMTP_PASSWORD`：SMTP 授权码，不是 QQ 登录密码。

未配置 SMTP 时无法完成新用户邮箱验证，已存在的账号可继续登录。新账号以已验证邮箱登录，历史用户名登录仍兼容。暂不支持更换注册邮箱或邮件找回密码。

启用异步通知后，任务完成会生成站内通知，并向用户已验证邮箱发送提醒；邮件不包含输入、日志或完整结果。邮件最多尝试三次，不保证严格一次投递。

## BOS 与数据备份

默认配置为北京 `https://bj.bcebos.com`、Bucket `tooldeck`，文件写入 `tooldeck/files/{file_id}`。BOS 模式缺少配置会拒绝启动。程序不创建 Bucket 或修改 ACL。

**公共读 Bucket 中的对象可被知道地址的人直接下载**，平台接口鉴权不能替代存储桶访问控制。涉及私密文件时应使用私有读策略。

平台账号、任务、工具和配置存放在 Docker 命名卷 `tooldeck_data`。备份需要同时保存数据卷、主密钥和 BOS 对象；不要执行 `docker compose down -v` 删除数据。切换存储不会自动迁移旧文件，旧文件仍按原元信息读取。

修改 `.env` 后运行 `docker compose up -d --force-recreate`；更新代码运行 `git pull --ff-only` 后再执行 `docker compose up -d --build`。

### 群晖 NAS 使用 Git 更新

生产源码建议克隆到 `/volume1/docker/tooldeck/repository`，将生产配置保留在外层 `/volume1/docker/tooldeck/.env`，并在源码目录创建 `.env` 软链接。持久化数据仍位于 `/volume1/docker/tooldeck/data`，不会被 Git 更新覆盖。

首次初始化（使用 `moyuu` 身份）：

```bash
cd /volume1/docker/tooldeck
/usr/local/bin/git clone --branch main --single-branch https://github.com/moyuuuuuuuuuuu/tooldeck.git repository
ln -s ../.env repository/.env
```

后续更新先使用 `moyuu` 身份拉取代码：

```bash
cd /volume1/docker/tooldeck/repository
/usr/local/bin/git pull --ff-only
```

再在 root 终端从源码构建并重建服务：

```bash
cd /volume1/docker/tooldeck/repository
/usr/local/bin/docker compose -f deploy/compose.synology.yaml up -d --build
```

不要执行 `docker compose down -v`，也不要把生产 `.env`、`data` 或构建依赖提交到 Git。

## 执行隔离与当前边界

构建及执行使用非 root 容器、只读根目录、移除全部 capabilities、禁止提权、Docker 默认 seccomp、独立 IPC/cgroup、进程和内存限制。执行容器不挂载 Docker Socket，运行限制 1 CPU；禁用 core dump、限制文件句柄和单文件大小、关闭 Docker 日志落盘。

运行临时目录使用 `nodev/nosuid/noexec`，构建目录因编译需要保留执行权限；历史未预编译 Go 工具也保留临时目录执行权限。代码没有专门的木马扫描或危险函数过滤。

**普通 Docker 不保证绝对防逃逸。控制服务持有 Docker Socket，目前未部署 gVisor、Kata 或独立执行虚拟机。** 公开承载不可信代码前，应进一步隔离执行节点并维护宿主机与容器运行时。

其他限制：

- 单平台进程、内置任务 Worker、文件持久化；不可多个进程同时写同一数据目录，尚无多机调度或外部数据库。
- 失败任务不自动重试，重启中断的执行不自动重跑；结果与日志主要在结束后查询。
- 暂无任意 Webhook 回调、自动历史清理或版本删除界面。
- 旧工具保留原运行镜像；新工具使用按版本构建流程。
- 已完成本地构建与功能测试；真实 BOS 与邮件投递仍需部署方填入凭证并验证。

## 本地开发

```sh
go test -race ./...
go vet ./...
cd web
npm ci
npm run build
```

前端开发使用 `npm run dev`，代理指向 `http://127.0.0.1:18088`。

| 目录 | 内容 |
| --- | --- |
| `cmd/`、`internal/` | Go 服务、认证、构建与执行 |
| `web/` | Vue 用户界面 |
| `runtimes/` | 各语言执行镜像与构建脚本 |
| `examples/`、`scripts/` | 示例代码包及辅助脚本 |
| `docs/`、`deploy/` | 规范、架构与部署资料 |

不要提交 `.env`、主密钥、上传代码、运行记录或第三方凭证。补充设计见 [架构说明](docs/architecture.md)。

## 上游与许可

前端基于 [SaiAdmin 6.x](https://github.com/saithink/saiadmin6.x) 的 `saiadmin-artd`，上游提交为 `8f5f6fa57b75c63c5cf370b4f4541e9e529eb306`，未引入其 PHP 后端。保留 [前端许可证](web/LICENSE) 和 [SaiAdmin 许可证](web/LICENSE-SaiAdmin)，使用和分发时请遵守对应许可。

### 群晖兼容部署

部分 DSM 内核不支持 CFS CPU 配额、PID 限制或私有 cgroup。显式设置 TOOLDECK_SANDBOX_PROFILE=synology 后，构建与执行固定至 CPU 0，省略 PID 限制及私有 cgroup 参数；不自动降级其他环境。该模式隔离能力较弱，需自行确认接受。群晖使用 `deploy/compose.synology.yaml` 从 Git 工作区构建，生产 `.env` 与 `data` 保留在仓库外层。NAS 主机的 seccomp 是否生效取决于内核支持，不能由配置补足。

## SSE 流式输出

工具声明 `execution.stream: true` 后，向 stderr 输出 `TOOLDECK_EVENT {"type":"delta","text":"片段"}`（每条换行并 flush），stdout 仍在结束时输出完整 JSON，文件产物协议不变。参考 `examples/stream-demo` 和站内开发指引。

网页实时追加显示内容。API 在执行 POST 请求中携带 `Accept: text/event-stream`，或创建任务后 GET `/api/v1/runs/{run_id}/events`；均要求登录令牌、API Key 或 OAuth。事件包含 run/status/delta/result/error/done/heartbeat，使用 delta 的 id 通过 Last-Event-ID 断线续传。断开订阅不取消任务。每实例最多 32 个 SSE 连接，单任务最多 4096 条事件，单行最多 64 KB，stderr 含事件合计最多 3 MB。

最终事件与结果保存在任务记录；意外退出前尚未落盘的增量可能丢失。反向代理需关闭缓冲并允许长连接，凭证不能放入 URL。工具仍需自行启用模型服务商的流式接口并逐片段转发；本示例不调用真实模型。

### 忘记密码

登录页点击「忘记密码」，使用已验证的注册邮箱接收6位验证码后设置新密码（12–128字节）。复用 `TOOLDECK_SMTP_*` 配置；重置验证码与注册验证码隔离，10分钟有效、60秒发送间隔、每邮箱每日最多10次，连续5次校验失败后需重新获取。成功后验证码立即失效，撤销该用户所有网页登录会话，保留 API Key 和业务数据。未绑定已验证邮箱的旧账号不能通过此方式找回。

### 免登录使用

未登录访问首页或 `/tooldeck/tools` 会进入 `/explore` 访客页面。仅公开、审核通过、构建成功、未下架且 `env` 和旧版 `secrets` 都为空的工具可匿名运行。同步、异步、SSE 与文件输入输出均按独立访客 Cookie 隔离，不提供匿名历史列表。上传工具、个人配置及账号功能仍需登录。

访客仅支持站内网页操作；网页内部请求要求同源浏览器上下文，不作为开放 API 提供。正式 `/api/v1/` 调用仍必须使用 API Key / OAuth，私有工具不向访客开放。访客写操作受全站账户操作频率限制，队列容量限制继续生效。
