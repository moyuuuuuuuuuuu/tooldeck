# ToolDeck JavaScript 空白工具模板

这是无需第三方依赖的 CommonJS 模板。分支根目录可以直接压缩为 ToolDeck 工具包，`Application` 类提供 `run()` 与 `cancel()`。

```bash
printf '%s' '{"text":"hello"}' | node main.js
```

主任务取得第三方任务 ID 后，将 JSON 状态原子写入 `TOOLDECK_STATE_FILE`。ToolDeck 取消任务时会以 `TOOLDECK_ACTION=cancel` 再次调用入口。
