# 第一版验证记录

验证日期：2026-09-07，Windows + Docker Desktop Linux 容器。

- `go test -race ./...`、`go vet ./...` 通过。
- `npm run build` 通过（Vite 生产构建 + Vue TypeScript 检查）。
- `docker compose build` 完整多阶段构建通过，服务健康检查通过。
- 四种实际容器执行通过：PHP、Node.js、Python、Go；JS / py / golang 别名映射到对应运行时。
- Python 图片示例：异步执行、参考图上传、文件 ID 权限检查、只读输入文件、图片产物与下载通过。
- 浏览器实测：管理员登录、动态菜单、工具抽屉、提示词输入、参考图上传、提交、状态轮询、图片预览通过。
- API Key 工具权限、跨调用方文件隔离、凭证撤销通过。
- 第三方 HTTPS 请求通过允许域名代理完成，直接出站 TCP 和未授权内网请求被拒绝。
- 第三方密钥授权、注入、原文日志脱敏及运行中取消通过。
- 单元测试覆盖 ZIP 路径穿越、链接、重复条目、结果归档路径、输入校验、幂等冲突、取消、重启不重放、OAuth 受众/有效期/权限。
- BOS 官方 SDK 对模拟对象服务的签名 PUT / GET、内容往返及缺失配置拒绝测试通过。

尚未验证：真实 BOS Bucket 上传/下载（缺少 AK/SK），外部 OAuth 提供商联调（未配置提供商），NAS 部署（本次仅本地部署）。没有调用付费 AI API。

本地地址：`http://localhost:18088`。管理员账号为 `admin`，密码在未提交 Git 的 `.env`。本地当前显式使用 local 存储，以便没有云凭证也能体验；填好 BOS AK/SK 后切换 `TOOLDECK_STORAGE=bos` 并重启。

开发过程中产生的临时版本及记录已归档到数据卷 `dev-test-archive/`；当前保留五个 1.0.0 示例。此归档位于同一数据卷，不是异地备份。
