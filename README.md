# ToolDeck PHP 空白工具模板

分支根目录就是可上传的工具包源码。业务类实现 `ToolInterface` 与 `CancelableToolInterface`，入口文件只负责 JSON 协议和 `run/cancel` 分派。

## 开发

```bash
composer install
printf '%s' '{"text":"hello"}' | php main.php
```

在 `tooldeck.json` 中声明网络白名单、环境变量和 Secret。取得第三方异步任务 ID 后，将 JSON 状态原子写入 `TOOLDECK_STATE_FILE`；取消时 `Application::cancel()` 会收到该状态。

打包时将 `tooldeck.json`、`main.php`、`composer.json`、`composer.lock`、`src/` 一起放在 ZIP 根目录，不要包含 `vendor/`。
