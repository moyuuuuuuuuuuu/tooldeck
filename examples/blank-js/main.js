const fs = require("node:fs");

class Application {
  run(input) {
    const text = String(input.text ?? "").trim();
    if (!text) throw new Error("文本内容不能为空");

    // 在这里替换为你的业务逻辑。
    return { message: "JavaScript 空白工具运行成功", text };
  }
}

try {
  const input = JSON.parse(fs.readFileSync(0, "utf8"));
  process.stdout.write(JSON.stringify(new Application().run(input)));
} catch (error) {
  process.stderr.write(error instanceof Error ? error.message : String(error));
  process.exitCode = 1;
}
