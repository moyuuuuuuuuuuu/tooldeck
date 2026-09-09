const fs = require("node:fs");

class Application {
  run(input) {
    const text = String(input.text ?? "").trim();
    if (!text) throw new Error("文本内容不能为空");

    // 在这里替换为你的业务逻辑。
    return { message: "JavaScript 空白工具运行成功", text };
  }

  cancel(input, state) {
    // 使用 state.provider_task_id 调用第三方取消接口。
    // 此空白示例没有远程任务，因此无需执行操作。
  }
}

function readState() {
  const path = process.env.TOOLDECK_STATE_FILE;
  return path && fs.existsSync(path) ? JSON.parse(fs.readFileSync(path, "utf8")) : {};
}

try {
  const input = JSON.parse(fs.readFileSync(0, "utf8"));
  const application = new Application();
  if (process.env.TOOLDECK_ACTION === "cancel") {
    application.cancel(input, readState());
  } else {
    process.stdout.write(JSON.stringify(application.run(input)));
  }
} catch (error) {
  process.stderr.write(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
}
