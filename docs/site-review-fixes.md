# 2026-10-13 站点评审修复验收

对应用户提供的 `ToolDeck站点评审.md`，代码修改和验证均在本地完成，尚未推送或部署线上。

| 问题 | 修复 | 验收 |
| --- | --- | --- |
| TD01 默认 PHP 环境失败 | PHP 8.1 使用官方 Bookworm 镜像；Composer 使用官方二进制镜像减少下载；运行镜像修订号升级至 build6；区分 environment/build 失败，实时展示准备日志及重试入口 | 真实 Docker 构建通过；隔离实例默认 PHP 8.1 示例输出 Hello, ToolDeck!；失败分类回归通过 |
| TD02 必填留空可执行 | 网页递归检查必填项、聚焦并定位字段；URL 示例声明 minLength=1，API 拒绝空文本 | 浏览器空输入显示中文提示并聚焦；API 返回 422；有效 URL 编码成功 |
| TD03 流式结果格式异常 | 示例最终输出统一为 result 字符串；兼容旧 text 对象和 MIME 类型别名，保留额外元数据 | SSE 片段拼接与最终结果一致；浏览器显示纯文本，无格式警告 |
| TD04 JSON 错误只显示退出码 | 示例输入使用 format=json；服务端校验报告行列及修正方向；前端提交前校验；程序失败直接展示有限长度诊断日志 | 无效 JSON 返回中文位置提示；修正后成功；浏览器提示直接可见 |
| TD05 历史与返回缺少上下文 | 历史显示工具名称、中文状态、本地时间，支持状态筛选；新记录保存名称，旧记录回填；搜索条件保存在 URL，恢复滚动位置并处理缓存页面 | 筛选、分页、调用方隔离、已删除工具名称回归通过；浏览器返回后仍保留 JSON 搜索 |

## 可重复检查

```sh
go test -race ./...
go vet ./...
cd web
COREPACK_ENABLE_AUTO_PIN=0 pnpm --ignore-workspace run build
./node_modules/.bin/tsx scripts/test-review-flows.ts
```

`scripts/review-smoke.py` 仅接受 localhost 的隔离实例。准备专用数据目录、当前后端和前端产物，并通过专用环境文件配置本地管理员密码；不要指向生产数据目录。

```sh
TOOLDECK_TEST_URL=http://127.0.0.1:18089 \
TOOLDECK_REVIEW_ENV_FILE=/path/to/local-review.env \
python3 scripts/review-smoke.py
```

该脚本创建测试工具和运行记录，包含默认 PHP 示例、URL 空值与成功执行、SSE 结果一致性、JSON 修正重试和历史筛选。

## 上线边界

- 默认运行环境实际在本地成功，但原线上失败的完整日志尚未取得，因此不能宣称已确认其唯一根因。
- 已安装的 URL/JSON/流式示例需要更新工具包、重新构建并发布版本；修改仓库源码不会自动替换数据库中的已发布工具。
- build6 镜像在首次使用时重新准备，需允许下载官方基础镜像和 Debian 包。
- 本轮没有变更生产数据、账户配置或线上服务，没有调用外部 AI 工具。
