export class Application {
  run(input) {
    const text = String(input.text ?? "").trim();
    if (!text) throw new Error("文本内容不能为空");

    // 在这里替换为你的业务逻辑。
    return { message: "Node.js 空白工具运行成功", text };
  }
}
