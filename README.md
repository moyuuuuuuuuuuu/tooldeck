# ToolDeck Node.js 空白工具模板

模板使用 npm、ES Module 和面向对象结构。`src/tool.js` 定义工具基类，`Application` 实现 `run()` 与 `cancel()`，`main.js` 只处理 ToolDeck 协议。

```bash
npm ci
printf '%s' '{"text":"hello"}' | node main.js
```

主任务取得第三方任务 ID 后，将 JSON 状态原子写入 `TOOLDECK_STATE_FILE`。打包时包含 `package.json`、`package-lock.json`、源码与 `tooldeck.json`，不要包含 `node_modules/`。
