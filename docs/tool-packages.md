# 工具包规范 v1

ZIP 根目录包含 `tooldeck.json` 与入口文件。示例见 `examples/`。

五种空白工具模板均提供独立源码分支，分支根目录只包含可上传的工具包代码，不包含 ToolDeck 平台本体：[PHP + Composer](https://github.com/moyuuuuuuuuuuu/tooldeck/tree/php-tool-example)、[JavaScript](https://github.com/moyuuuuuuuuuuu/tooldeck/tree/js-tool-example)、[Node.js + npm](https://github.com/moyuuuuuuuuuuu/tooldeck/tree/node-tool-example)、[Python](https://github.com/moyuuuuuuuuuuu/tooldeck/tree/python-tool-example)、[Go Modules](https://github.com/moyuuuuuuuuuuu/tooldeck/tree/go-tool-example)。

## 输入与输出

入口程序从 stdin 读取一个 JSON 对象，stdout 输出且仅输出一个 JSON 值，日志写 stderr。非零退出码表示失败。

文件字段使用 `format: "tooldeck-file"`。API 提交文件 ID；Worker 在运行前将其替换为容器内只读文件路径。输出文件写入 `TOOLDECK_OUTPUT_DIR`，平台在程序结束后收集并返回 `artifacts`，不接受程序提供的任意宿主机路径。运行产物默认保留 7 天且每个文件最多下载 3 次，到期或额度用完后从对象存储和运行记录中删除；Web 使用记录显示剩余次数与到期时间，开启异步邮件通知时可发送同样受限的签名下载链接。

`output_schema.type` 可取 `json`、`text`、`image-gallery`。JSON / 文本通过 result 展示，图片及文件通过 artifacts 展示。不执行输出的 HTML 或 JavaScript。

## Schema 支持范围

这是 JSON Schema 的受限子集：type、title、description、format、properties、required、items、enum、default、minLength、maxLength、minItems、maxItems、minimum、maximum。未知描述字段在上传时拒绝，不加载 `$ref` 或远程 schema。对象拒绝额外输入属性，最多嵌套 8 层。第一版仅顶层属性自动应用 default。

ui_schema 按字段名配置 widget（textarea、radio、select、image-upload、file-upload、json）、order（数字越小越靠前）、placeholder、rows、accept（精确 MIME 列表）、max_file_size_mb。图片格式服务端按内容探测，不能只改文件后缀绕过限制。

上传界面可覆盖 execution.mode。execution.timeout_seconds 范围 1..900，memory_mb 范围 64..2048。执行以平台保存的描述为准。

## 取消钩子

需要取消第三方异步任务的工具可设置 `execution.cancel_hook: true`。用户取消一个正在运行的任务时，平台先在同一个容器中再次调用工具入口，并设置 `TOOLDECK_ACTION=cancel`；取消入口仍从 stdin 收到原始输入。主任务使用 `TOOLDECK_ACTION=run`。

主任务取得第三方任务 ID 后，应以 JSON 写入 `TOOLDECK_STATE_FILE` 指定的文件（先写临时文件再原子重命名）。取消入口可读取该文件并调用第三方取消接口。钩子最长执行 15 秒；成功后平台停止主任务并标记 `canceled`，失败或超时则停止主任务并标记 `cancel_failed`，错误摘要位于 `cancel_error`。排队中尚未启动的任务直接取消，不调用钩子。取消请求具有幂等性，`canceling`、`canceled` 或其他终态不会再次调用钩子。

取消钩子的 stdout 和 stderr 不进入运行结果或公开日志，避免第三方 SDK 错误意外泄漏密钥。工具应自行把第三方取消请求设计为幂等操作，并正确处理“第三方任务 ID 尚未写入”的竞态。PHP 面向对象示例见 `examples/blank-php`。

## 网络

network.enabled 为 false 时完全无出站网络。启用后必须列出 allowed_hosts 精确域名，例如 `api.example.com`。不支持通配符或内网目标，重定向到其他域名也需要授权。

运行环境注入 HTTP_PROXY / HTTPS_PROXY（及小写变量），Python urllib、Go 默认 HTTP transport、PHP curl 可通过它们访问。Node 24 配置 NODE_USE_ENV_PROXY；自定义 SDK 如自行创建网络 transport，需要明确启用代理。直接 TCP 连接不会绕过隔离。

第三方密钥在 secrets 中声明环境变量名，并在管理页面授予对应工具使用权。已授权代码能够读取并使用这些密钥，因此仅向可信工具授权。平台不会把密钥混入 API 示例或返回密钥管理列表；日志中出现的原始密钥值会被替换，但工具自身不应输出凭证。

## 源码构建与依赖

无需上传 node_modules 或 vendor。Node 提交 package.json 与 package-lock.json，平台执行 npm ci；PHP 提交 composer.json 与 composer.lock，执行 composer install --no-dev；Python requirements.txt 必须包含精确版本及哈希，执行 pip install --require-hashes；Go 模块提交 go.mod/go.sum，平台下载依赖并生成可执行文件，单文件 Go 不要求模块文件。

runtime_version 可选 PHP 8.0/8.1/8.2/8.3、Node 20/21/22/23、Python 3.10/3.11/3.12、Go 1.22/1.23/1.24。默认分别为8.3、22、3.12、1.24。build_command 为可选命令（如 npm run build），运行入口在构建完成后必须存在。构建与执行绑定同一镜像 ID。

首次使用版本会自动准备环境，最长20分钟；依赖构建最长10分钟，限制1CPU、1GB内存、512MB工作目录、256MB临时目录。构建不注入业务密钥，仅允许内置依赖仓库域名出站；不支持私有仓库授权。基础 Node/Python 镜像包含基础编译工具，额外系统库仍需专用镜像。

构建产物限512MB、40000条目，支持目录内相对软链接，禁止路径逃逸和特殊文件。成功产物固定保存，执行时只读复用，Go不再逐次编译。构建失败可重试；构建成功后如需变更源码或依赖请上传新版本。公开工具必须构建成功才可审核通过和被他人使用。旧工具继续使用旧执行方式。

ZIP 仍限制64MB、展开128MB、2000条目。日志在构建结束后提供，不是实时流。

## 上传选项

网页上传提供第三方访问、执行模式和允许 API 调用等选项。异步 API 调用必须由请求方传入公网 HTTPS callback_url，可选 callback_secret；任务结束后平台回调结果，失败时按 3、30、300、3000、300000 秒重试。正常完成不发站内信或邮件，所有回调尝试均失败后才向 API Key 所属账号发送一条站内异常通知。私有工具仅本人和管理员可见，公开工具按站点设置审核后对登录用户可见，不可引用平台密钥；关闭 API 的工具会被服务端拒绝外部调用。

## 工具环境变量

描述文件可声明 `env_mode: "developer"`（开发者统一提供）或 `"user"`（使用者各自配置），并声明 `env: [{"name":"IMAGE_API_KEY","description":"生图服务密钥","required":true,"sensitive":true}]`。作者也可在工具详情的环境变量页维护当前版本定义。值通过网页设置，不进入代码ZIP。

所有配置值加密保存且不回显；按上传者、工具name、提供方式、配置者隔离。不同版本共用同一范围的值；每个版本独立读取 env，未声明或空数组均表示不需要环境变量，不继承旧定义；详情隐藏环境变量栏目，统一配置列表排除此工具。两个提供方式的配置独立，切换不会把原凭证公开给使用者。必填未配置返回422。运行时注入声明的变量，日志遮盖原值；工具代码不得主动在stdout、输出文件中泄漏凭证。保留的运行环境变量不能覆盖。

`GET/POST /api/v1/tools/{id}/environment` 需要用户会话。作者可提交mode和fields；配置者提交values对象和delete名称数组。其他使用者无法查看开发者配置值，也不能更改定义。

API 执行 `POST /api/v1/tools/{id}/runs` 可在请求体顶层传递 `env` 对象。不传时使用调用身份已保存的个人配置；传入的已声明变量仅覆盖本次运行，优先级最高。临时值加密持久化以支持异步队列，但不会包含在任务响应、使用记录或日志中。同一个 `Idempotency-Key` 重试时，`input` 与 `env` 都必须保持一致。

个人配置始终开放，不受env_mode限制。执行逐变量优先使用当前用户个人配置；env_mode为developer（或空）时缺失项可使用作者共享配置，user模式禁止回退。GET/POST环境变量接口默认编辑个人配置；仅作者可传 `?scope=shared` 编辑共享配置。两类存储隔离，其他使用者不能修改共享值，不能修改变量定义。清除个人值后恢复共享回退策略。

## 管理自己上传的工具

上传完成后，版本处于“待构建 / 待提交”。作者在「我上传的工具」中主动开始构建，构建成功后才能提交；公开版本按站点规则进入审核，私有版本提交后直接可用。该页面按版本展示构建和审核状态、驳回原因，并支持搜索筛选。下架只作用于选中版本，阻止其他用户的新调用，同时保留作者访问、代码、配置及历史记录，已开始的任务继续执行。
