# ToolDeck Go 空白工具模板

模板使用 Go Modules，并通过 `Tool` 接口约束 `Run` 与 `Cancel`。Go 没有传统类继承，示例使用接口和结构体组合保持清晰边界。

```bash
go mod download
printf '%s' '{"text":"hello"}' | go run .
```

主任务取得第三方任务 ID 后，将 JSON 状态原子写入 `TOOLDECK_STATE_FILE`。打包时包含 `go.mod`、`go.sum`、源码和 `tooldeck.json`，不要包含本地编译产物。
