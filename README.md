# ToolDeck Python 空白工具模板

模板仅使用标准库，通过抽象基类定义 `run()` 与 `cancel()`。分支根目录可以直接压缩为 ToolDeck 工具包。

```bash
printf '%s' '{"text":"hello"}' | python3 main.py
```

主任务取得第三方任务 ID 后，将 JSON 状态原子写入 `TOOLDECK_STATE_FILE`。如需第三方依赖，请增加带精确版本和哈希的 `requirements.txt`。
