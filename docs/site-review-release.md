# 站点评审修复上线步骤

适用于群晖 `/volume1/docker/tooldeck`，使用 `deploy/compose.synology.yaml` 和原 `.env`。本次只准备发布，未操作 NAS。

## 发布内容

- 平台：TD01～TD05 修复，运行镜像修订号 build6。
- 工具包：`url-codec`、`json-formatter`、`stream-demo` 均为 **1.0.1**。原 1.0.0 保留，可在版本发布页回滚。
- 本地打包：`python3 scripts/package-site-review.py`，输出到 `work/releases/site-review-20261013/`，附 SHA-256 和源码提交号。
- 平台源码包仅由 `git archive` 生成，不包含 `.env`、生产数据、密码或缓存。解压到新的版本目录；不要覆盖现有 `.env` 和 `data`。

## 群晖更新

先记录当前源码提交和镜像 ID，确认没有执行中/排队任务或构建。以下备份必须成功后才能更新。

```sh
cd /volume1/docker/tooldeck
release_stamp=$(date +%Y%m%d-%H%M%S)
sudo mkdir -p /volume1/docker/tooldeck-backups
sudo chmod 700 /volume1/docker/tooldeck-backups
git rev-parse HEAD > /tmp/tooldeck-before-revision
sudo docker image inspect tooldeck:dev --format '{{.Id}}' > /tmp/tooldeck-before-image
sudo docker tag "$(cat /tmp/tooldeck-before-image)" "tooldeck:rollback-$release_stamp"
sudo docker compose -f deploy/compose.synology.yaml stop platform
sudo tar -czpf "/volume1/docker/tooldeck-backups/pre-review-$release_stamp.tar.gz" .env data
sudo chmod 600 "/volume1/docker/tooldeck-backups/pre-review-$release_stamp.tar.gz"
sudo docker compose -f deploy/compose.synology.yaml start platform
```

不要直接在生产目录解压源码归档。将其解压到同级新目录，复制原 `.env`（文件权限设为 600），在新目录构建。群晖 compose 的数据挂载仍指向原 `/volume1/docker/tooldeck/data`。

```sh
cd /volume1/docker/tooldeck-release-20261013
sudo docker compose -f deploy/compose.synology.yaml build platform
sudo docker compose -f deploy/compose.synology.yaml up -d platform
sudo docker compose -f deploy/compose.synology.yaml ps
curl -fsS http://127.0.0.1:18088/healthz
```

备份完成后先恢复旧服务，构建期间继续服务；新目录的 `up -d` 会在同一 compose 项目中替换旧容器。不要以其他 compose 项目名同时挂载同一数据目录启动。

若构建需要已有代理，沿用原部署的 `sudo env TOOLDECK_BUILD_HTTP_PROXY=... TOOLDECK_BUILD_HTTPS_PROXY=...`，不要将代理凭证写入仓库。构建失败时应恢复旧服务，不要继续上线。

## 更新已安装示例

由各工具原作者登录上传对应 1.0.1 ZIP，保持原工具标识、作者、公开/API 设置。不能换成其他作者上传同名工具。

按 URL、JSON、SSE 顺序操作：上传源码 → 构建成功 → 提交审核 → 审核通过 → 版本发布设为默认。审核通过不会自动替换原默认版本。每个工具完成下面的验收后，再处理下一个。

## 线上验收

1. 默认 PHP 8.1、内置代码输出 `Hello, ToolDeck!`；第一次准备镜像允许等待，日志应实时更新。
2. URL 文本留空时提示并聚焦，历史不新增记录；输入 `a b` 编码为 `a%20b`。
3. SSE 示例有实时片段，最终纯文本为 `你好，这是 ToolDeck 实时输出。`，无格式异常提示。
4. JSON 输入 `{"a":}` 时直接提示格式错误；修正成 `{"a":1}` 后成功。
5. 历史显示工具名称、中文状态和本地时间；状态筛选正确；搜索 JSON、打开后返回，搜索及浏览位置保留。
6. 公网首页、登录后页面、`/healthz`、已有 MCP 调用各检查一次；以实际结果为准，容器启动不等于上线验收通过。

## 回滚

平台和工具版本分别回滚。工具在“版本发布”页回滚至旧默认版本；新运行镜像不强制清理。

平台回滚先停止新服务，在旧源码目录将 `tooldeck:dev` 恢复为已记录的 `tooldeck:rollback-<备份时间>`，然后用旧 compose 启动，**不要执行 build**。本次没有数据库迁移，先保留升级后数据；若确认需要恢复备份，先另存当前整个 data 目录，再离线恢复，避免丢失升级期间的新记录。
